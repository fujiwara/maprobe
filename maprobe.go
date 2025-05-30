package maprobe

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kong"
	"github.com/fujiwara/sloghandler"
	"github.com/fujiwara/sloghandler/otelmetrics"
	mackerel "github.com/mackerelio/mackerel-client-go"
	"github.com/mattn/go-isatty"
	"github.com/shogo82148/go-retry"
	"go.opentelemetry.io/otel"
	otelattribute "go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	otelmetric "go.opentelemetry.io/otel/metric"
	otelsdkmetric "go.opentelemetry.io/otel/sdk/metric"
	otelsdkmetricdata "go.opentelemetry.io/otel/sdk/metric/metricdata"
	otelsdkresource "go.opentelemetry.io/otel/sdk/resource"
)

var (
	Version                = "HEAD"
	MaxConcurrency         = 100
	MaxClientConcurrency   = 5
	PostMetricBufferLength = 100
	ProbeInterval          = 60 * time.Second
	MackerelAPIKey         string
	MackerelOtelEndpoint   = "otlp.mackerelio.com:4317"

	sem              = make(chan struct{}, MaxConcurrency)
	clientSem        = make(chan struct{}, MaxClientConcurrency)
	metricTimeMargin = -3 * time.Minute
)

var retryPolicy = retry.Policy{
	MinDelay: 1 * time.Second,
	MaxDelay: 10 * time.Second,
	MaxCount: 5,
}

func lock() {
	sem <- struct{}{}
	slog.Debug("locked", "concurrency", len(sem))
}

func unlock() {
	<-sem
	slog.Debug("unlocked", "concurrency", len(sem))
}

func Run(ctx context.Context, wg *sync.WaitGroup, configPath string, once bool) error {
	defer wg.Done()
	defer slog.Info("stopping maprobe")

	slog.Info("starting maprobe")
	conf, confDigest, err := LoadConfig(ctx, configPath)
	if err != nil {
		return err
	}
	slog.Debug("config", "config", conf.String())
	client := newClient(ctx, MackerelAPIKey, conf.Backup.FirehoseStreamName)

	chs := NewChannels(conf.Destination)
	defer chs.Close()

	var exporter otelsdkmetric.Exporter
	var resource *otelsdkresource.Resource
	var statsCollector *StatsCollector
	if oc := conf.Destination.Otel; oc != nil && oc.Enabled {
		var err error
		exporter, resource, err = newOtelExporter(ctx, conf.Destination.Otel)
		if err != nil {
			return fmt.Errorf("failed to create OpenTelemetry meter exporter: %w", err)
		}
		defer exporter.Shutdown(ctx)
		
		// Setup logger with metrics
		provider := modifyLoggerWithMetricExporter(exporter, resource, oc.StatsAttributes)
		
		// Create stats collector
		statsCollector, err = NewStatsCollector(provider)
		if err != nil {
			slog.Error("failed to create stats collector", "error", err)
			statsCollector = nil // Continue without stats metrics
		}
	}

	// Set probe configs count for stats
	if statsCollector != nil {
		statsCollector.SetProbeConfigs(int64(len(conf.Probes)))
	}

	if len(conf.Probes) > 0 {
		if conf.PostProbedMetrics {
			if conf.Destination.Mackerel.Enabled {
				wg.Add(2)
				go postHostMetricWorker(ctx, wg, client, chs)
				go postServiceMetricWorker(ctx, wg, client, chs)
			}
			if conf.Destination.Otel.Enabled {
				wg.Add(1)
				go postOtelMetricWorker(ctx, wg, exporter, resource, chs)
			}
		} else {
			if conf.Destination.Mackerel.Enabled {
				wg.Add(2)
				go dumpHostMetricWorker(ctx, wg, chs)
				go dumpServiceMetricWorker(ctx, wg, chs)
			}
			if conf.Destination.Otel.Enabled {
				wg.Add(1)
				go dumpOtelMetricWorker(ctx, wg, chs)
			}
		}
	}

	if len(conf.Aggregates) > 0 {
		if conf.PostAggregatedMetrics {
			if conf.Destination.Mackerel.Enabled {
				// aggregates are posted to Mackerel only
				wg.Add(1)
				go postServiceMetricWorker(ctx, wg, client, chs)
			}
			// TODO: aggregates are not posted to OTel yet
		} else {
			wg.Add(1)
			go dumpServiceMetricWorker(ctx, wg, chs)
		}
	}

	ticker := time.NewTicker(ProbeInterval)
	for {
		var wg2 sync.WaitGroup
		for _, pd := range conf.Probes {
			wg2.Add(1)
			go pd.RunProbes(ctx, client, chs, statsCollector, &wg2)
		}
		for _, ag := range conf.Aggregates {
			wg2.Add(1)
			go runAggregates(ctx, ag, client, chs, &wg2)
		}
		wg2.Wait()
		if once {
			return nil
		}

		slog.Debug("waiting for a next tick")
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		slog.Debug("checking a new config")
		newConf, digest, err := LoadConfig(ctx, configPath)
		if err != nil {
			slog.Warn("config load failed", "error", err)
			slog.Warn("still using current config")
		} else if confDigest != digest {
			conf = newConf
			confDigest = digest
			slog.Info("config reloaded")
			slog.Debug("reloaded config", "config", conf)
		}
	}
}

func runAggregates(_ context.Context, ag *AggregateDefinition, client *Client, chs *Channels, wg *sync.WaitGroup) {
	defer wg.Done()

	service := ag.Service.String()
	roles := exStrings(ag.Roles)
	statuses := exStrings(ag.Statuses)
	slog.Debug("aggregates finding hosts", "service", service, "roles", roles, "statuses", statuses)

	hosts, err := client.FindHosts(&mackerel.FindHostsParam{
		Service:  service,
		Roles:    roles,
		Statuses: statuses,
	})
	if err != nil {
		slog.Error("aggregates find hosts failed", "error", err)
		return
	}
	slog.Debug("aggregates hosts found", "count", len(hosts))

	hostIDs := make([]string, 0, len(hosts))
	for _, h := range hosts {
		hostIDs = append(hostIDs, h.ID)
	}
	metricNames := make([]string, 0, len(ag.Metrics))
	for _, m := range ag.Metrics {
		metricNames = append(metricNames, m.Name.String())
	}

	slog.Debug("fetching latest metrics", "hosts", hostIDs, "metrics", metricNames)

	// TODO: If latest API will returns metrics refreshed at on minute,
	// We will replace to client.FetchLatestMetricValues().
	latest, err := client.fetchLatestMetricValues(hostIDs, metricNames)
	if err != nil {
		slog.Error("fetch latest metrics failed", "error", err, "hosts", hostIDs, "metrics", metricNames)
		return
	}

	now := time.Now()
	for _, mc := range ag.Metrics {
		name := mc.Name.String()
		var timestamp int64
		values := []float64{}
		for hostID, metrics := range latest {
			if _v, ok := metrics[name]; ok {
				if _v == nil {
					slog.Debug("latest metric not found", "host", hostID, "metric", name)
					continue
				}
				v, ok := _v.Value.(float64)
				if !ok {
					slog.Warn("latest metric not float64", "host", hostID, "metric", name, "value", _v)
					continue
				}
				ts := time.Unix(_v.Time, 0)
				slog.Debug("latest metric", "host", hostID, "metric", name, "time", _v.Time, "value", v)
				if ts.After(now.Add(metricTimeMargin)) {
					values = append(values, v)
					if _v.Time > timestamp {
						timestamp = _v.Time
					}
				} else {
					slog.Warn("latest metric outdated", "host", hostID, "metric", name, "time", ts)
				}
			}
		}
		if len(hosts) > 0 && len(values) == 0 {
			slog.Warn("latest values not found", "service", ag.Service, "metric", mc.Name)
		}

		for _, output := range mc.Outputs {
			var value float64
			if len(values) == 0 {
				if !output.EmitZero {
					continue
				}
				timestamp = now.Add(-1 * time.Minute).Unix()
			} else {
				value = output.calc(values)
			}
			slog.Debug("aggregates result", "func", output.Func, "input", name, "value", value, "service", ag.Service, "output", output.Name, "timestamp", timestamp)
			m := Metric{
				Name:      output.Name.String(),
				Value:     value,
				Timestamp: time.Unix(timestamp, 0),
			}
			chs.SendAggregatedMetric(m.ServiceMetric(ag.Service.String()))
		}
	}
}

func postHostMetricWorker(ctx context.Context, wg *sync.WaitGroup, client *Client, chs *Channels) {
	slog.Info("starting postHostMetricWorker")
	defer wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	mvs := make([]*mackerel.HostMetricValue, 0, PostMetricBufferLength)
	run := true
	for run {
		select {
		case m, cont := <-chs.HostMetrics:
			if cont {
				mvs = append(mvs, m.HostMetricValue())
				if len(mvs) < PostMetricBufferLength {
					continue
				}
			} else {
				slog.Info("shutting down postHostMetricWorker")
				run = false
			}
		case <-ticker.C:
		}
		if len(mvs) == 0 {
			continue
		}
		slog.Debug("posting host metrics to Mackerel", "count", len(mvs))
		b, _ := json.Marshal(mvs)
		slog.Debug("host metrics payload", "payload", string(b))
		if err := doRetry(ctx, func() error {
			return client.PostHostMetricValues(ctx, mvs)
		}); err != nil {
			slog.Error("failed to post host metrics to Mackerel", "error", err)
			continue
		}
		slog.Debug("post host metrics succeeded")
		// success. reset buffer
		mvs = mvs[:0]
	}
}

func postServiceMetricWorker(ctx context.Context, wg *sync.WaitGroup, client *Client, chs *Channels) {
	slog.Info("starting postServiceMetricWorker")
	defer wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	mvsMap := make(map[string][]*mackerel.MetricValue)
	run := true
	for run {
		select {
		case m, cont := <-chs.ServiceMetrics:
			if cont {
				if math.IsNaN(m.Value) {
					slog.Warn("NaN value not supported by Mackerel", "service", m.Service, "metric", m.Name)
					continue
				} else {
					mvsMap[m.Service] = append(mvsMap[m.Service], m.MetricValue())
				}
				if len(mvsMap[m.Service]) < PostMetricBufferLength {
					continue
				}
			} else {
				slog.Info("shutting down postServiceMetricWorker")
				run = false
			}
		case <-ticker.C:
		}

		for serviceName, mvs := range mvsMap {
			if len(mvs) == 0 {
				continue
			}
			slog.Debug("posting service metrics to Mackerel", "count", len(mvs), "service", serviceName)
			b, _ := json.Marshal(mvs)
			slog.Debug("service metrics payload", "payload", string(b))
			if err := doRetry(ctx, func() error {
				return client.PostServiceMetricValues(ctx, serviceName, mvs)
			}); err != nil {
				slog.Error("failed to post service metrics to Mackerel", "service", serviceName, "error", err)
				continue
			}
			slog.Debug("post service succeeded")
			// success. reset buffer
			mvs = mvs[:0]
			mvsMap[serviceName] = mvs
		}
	}
}

func postOtelMetricWorker(ctx context.Context, wg *sync.WaitGroup, exporter otelsdkmetric.Exporter, resource *otelsdkresource.Resource, chs *Channels) {
	defer wg.Done()
	slog.Info("starting postOtelMetricWorker")

	ticker := time.NewTicker(10 * time.Second)
	mvs := make([]otelsdkmetricdata.Metrics, 0, PostMetricBufferLength)
	run := true
	for run {
		select {
		case m, cont := <-chs.OtelMetrics:
			if cont {
				mvs = append(mvs, m.Otel())
				slog.Debug("otel metric", "metric", m.OtelString())
				if len(mvs) < PostMetricBufferLength {
					continue
				}
			} else {
				slog.Info("shutting down postOtelMetricWorker")
				run = false
			}
		case <-ticker.C:
		}
		if len(mvs) == 0 {
			continue
		}
		slog.Debug("posting otel metrics", "count", len(mvs))
		rms := &otelsdkmetricdata.ResourceMetrics{
			Resource: resource,
			ScopeMetrics: []otelsdkmetricdata.ScopeMetrics{
				{Metrics: mvs},
			},
		}
		if err := doRetry(ctx, func() error {
			return exporter.Export(ctx, rms)
		}); err != nil {
			slog.Error("failed to export otel metrics", "error", err)
			continue
		}
		slog.Debug("post otel metrics succeeded")
		// success. reset buffer
		mvs = mvs[:0]
	}
}

func newOtelExporter(ctx context.Context, oc *OtelConfig) (*otlpmetricgrpc.Exporter, *otelsdkresource.Resource, error) {
	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithHeaders(map[string]string{"Mackerel-Api-Key": MackerelAPIKey}),
		otlpmetricgrpc.WithCompressor("gzip"),
	}

	var endpointURL = url.URL{
		Scheme: "https",
		Host:   oc.Endpoint,
	}
	if endpointURL.Host == "" {
		endpointURL.Host = MackerelOtelEndpoint
	}
	opts = append(opts, otlpmetricgrpc.WithEndpoint(endpointURL.Host))
	if oc.Insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
		endpointURL.Scheme = "http"
	}
	slog.Info("creating otel exporter", "endpoint", endpointURL.String())

	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return nil, nil, err
	}

	// Create Resource with resource_attributes
	resourceAttrs := make([]otelattribute.KeyValue, 0, len(oc.ResourceAttributes))
	for k, v := range oc.ResourceAttributes {
		resourceAttrs = append(resourceAttrs, otelattribute.String(k, v))
	}
	resource, err := otelsdkresource.New(ctx, otelsdkresource.WithAttributes(resourceAttrs...))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create resource: %w", err)
	}
	slog.Info("otel exporter created", "resource", resource.String())

	return exporter, resource, nil
}

func dumpHostMetricWorker(_ context.Context, wg *sync.WaitGroup, chs *Channels) {
	defer wg.Done()
	slog.Info("starting dumpHostMetricWorker")
	for m := range chs.HostMetrics {
		b, _ := json.Marshal(m.HostMetricValue())
		slog.Info("host metric", "host", m.HostID, "metric", string(b))
	}
}

func dumpServiceMetricWorker(_ context.Context, wg *sync.WaitGroup, chs *Channels) {
	defer wg.Done()
	slog.Info("starting dumpServiceMetricWorker")
	for m := range chs.ServiceMetrics {
		b, _ := json.Marshal(m.MetricValue())
		slog.Info("service metric", "service", m.Service, "metric", string(b))
	}
}

func dumpOtelMetricWorker(_ context.Context, wg *sync.WaitGroup, chs *Channels) {
	defer wg.Done()
	slog.Info("starting dumpOtelMetricWorker")
	for m := range chs.OtelMetrics {
		slog.Info("otel metric", "metric", m.OtelString())
	}
}

type templateParam struct {
	Host *mackerel.Host
}

func doRetry(ctx context.Context, f func() error) error {
	r := retryPolicy.Start(ctx)
	var err error
	for r.Continue() {
		err = f()
		if err == nil {
			return nil
		}
		slog.Warn("retrying", "error", err)
	}
	if r.Err() != nil {
		return r.Err()
	}
	return fmt.Errorf("retry failed: %w", err)
}

// Main is the main entry point that handles CLI parsing and command execution
func Main(ctx context.Context, args []string) error {
	var cli CLI

	// Parse command line arguments
	parser, err := kong.New(&cli)
	if err != nil {
		return fmt.Errorf("failed to create parser: %w", err)
	}

	kongCtx, err := parser.Parse(args[1:]) // Skip program name
	if err != nil {
		return fmt.Errorf("failed to parse arguments: %w", err)
	}

	fullCommandName := kongCtx.Command()
	// Extract the base command name (Kong may return "command <arg>" format)
	cmdName, _, _ := strings.Cut(fullCommandName, " ")

	var wg sync.WaitGroup

	switch cmdName {
	case "version":
		fmt.Printf("maprobe version %s\n", Version)
		return nil
	case "agent":
		if cli.Agent.WithFirehoseEndpoint {
			wg.Add(1)
			go RunFirehoseEndpoint(ctx, &wg, cli.Agent.Port)
		}
		wg.Add(1)
		err = Run(ctx, &wg, cli.Agent.Config, false)
	case "once":
		wg.Add(1)
		err = Run(ctx, &wg, cli.Once.Config, true)
	case "lambda":
		slog.Info("running on AWS Lambda", "config", cli.Lambda.Config)
		wg.Add(1)
		err = Run(ctx, &wg, cli.Lambda.Config, true)
	case "ping":
		err = runProbe(ctx, cli.Ping.HostID, &PingProbeConfig{
			Address: cli.Ping.Address,
			Count:   cli.Ping.Count,
			Timeout: cli.Ping.Timeout,
		})
	case "tcp":
		err = runProbe(ctx, cli.TCP.HostID, &TCPProbeConfig{
			Host:               cli.TCP.Host,
			Port:               cli.TCP.Port,
			Timeout:            cli.TCP.Timeout,
			Send:               cli.TCP.Send,
			Quit:               cli.TCP.Quit,
			ExpectPattern:      cli.TCP.ExpectPattern,
			NoCheckCertificate: cli.TCP.NoCheckCertificate,
			TLS:                cli.TCP.TLS,
		})
	case "http":
		err = runProbe(ctx, cli.HTTP.HostID, &HTTPProbeConfig{
			URL:                cli.HTTP.URL,
			Method:             cli.HTTP.Method,
			Body:               cli.HTTP.Body,
			Headers:            cli.HTTP.Headers,
			Timeout:            cli.HTTP.Timeout,
			ExpectPattern:      cli.HTTP.ExpectPattern,
			NoCheckCertificate: cli.HTTP.NoCheckCertificate,
		})
	case "firehose-endpoint":
		wg.Add(1)
		RunFirehoseEndpoint(ctx, &wg, cli.FirehoseEndpoint.Port)
	default:
		return fmt.Errorf("command %s does not exist", cmdName)
	}

	wg.Wait()
	return err
}

func mackerelHost(id string) (*mackerel.Host, error) {
	if apikey := os.Getenv("MACKEREL_APIKEY"); id != "" && apikey != "" {
		slog.Debug("finding host", "id", id)
		client := mackerel.NewClient(apikey)
		return client.FindHost(id)
	}
	slog.Debug("using dummy host")
	return &mackerel.Host{ID: "dummy"}, nil
}

func runProbe(ctx context.Context, id string, pc ProbeConfig) error {
	slog.Debug("probe config", "config", fmt.Sprintf("%#v", pc))
	host, err := mackerelHost(id)
	if err != nil {
		return err
	}
	slog.Debug("host", "host", marshalJSON(host))
	p, err := pc.GenerateProbe(host)
	if err != nil {
		return err
	}
	ms, err := p.Run(ctx)
	if len(ms) > 0 {
		fmt.Print(ms.String())
	}
	if err != nil {
		return err
	}
	return nil
}

func marshalJSON(i interface{}) string {
	b, _ := json.Marshal(i)
	return string(b)
}

// SetupLogger configures structured logging
func SetupLogger(logLevel, logFormat string) {
	var w = os.Stderr

	var level slog.Level
	switch strings.ToLower(logLevel) {
	case "trace", "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var handler slog.Handler

	switch strings.ToLower(logFormat) {
	case "json":
		opts := &slog.HandlerOptions{
			Level:     level,
			AddSource: false,
		}
		handler = slog.NewJSONHandler(w, opts)
	default:
		// Use sloghandler for colorized text output
		opts := &sloghandler.HandlerOptions{
			HandlerOptions: slog.HandlerOptions{
				Level:     level,
				AddSource: false,
			},
			Color: isatty.IsTerminal(w.Fd()), // Enable color output if the output is a terminal
		}
		handler = sloghandler.NewLogHandler(w, opts)
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func modifyLoggerWithMetricExporter(exporter otelsdkmetric.Exporter, resource *otelsdkresource.Resource, attrs map[string]string) *otelsdkmetric.MeterProvider {
	slog.Info("modifying logger with metric exporter", "exporter", fmt.Sprintf("%T", exporter))
	reader := otelsdkmetric.NewPeriodicReader(exporter)
	provider := otelsdkmetric.NewMeterProvider(
		otelsdkmetric.WithReader(reader),
		otelsdkmetric.WithResource(resource),
	)

	meterOpts := make([]otelmetric.MeterOption, 0, len(attrs))
	for k, v := range attrs {
		meterOpts = append(meterOpts, otelmetric.WithInstrumentationAttributes(otelattribute.String(k, v)))
	}
	meter := provider.Meter(
		"maprobe/logs",
		meterOpts...,
	)
	counter, _ := meter.Int64Counter(
		"messages",
		otelmetric.WithDescription("Number of log messages by level"),
	)

	otel.SetMeterProvider(provider)

	handler := otelmetrics.NewHandler(slog.Default().Handler(), counter)
	slog.SetDefault(slog.New(handler))
	
	return provider
}
