package maprobe

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/url"
	"sync"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"
	"github.com/shogo82148/go-retry"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	otelmetricdata "go.opentelemetry.io/otel/sdk/metric/metricdata"
	otelresource "go.opentelemetry.io/otel/sdk/resource"
)

var (
	Version                = "HEAD"
	MaxConcurrency         = 100
	MaxClientConcurrency   = 5
	PostMetricBufferLength = 100
	sem                    = make(chan struct{}, MaxConcurrency)
	clientSem              = make(chan struct{}, MaxClientConcurrency)
	ProbeInterval          = 60 * time.Second
	metricTimeMargin       = -3 * time.Minute
	MackerelAPIKey         string
)

// CLI defines the command line interface structure for Kong
type CLI struct {
	LogLevel    string `name:"log-level" help:"log level" default:"info" env:"LOG_LEVEL"`
	GopsEnabled bool   `name:"gops" help:"enable gops agent" default:"false" env:"GOPS"`

	Version          VersionCmd          `cmd:"" help:"Show version"`
	Agent            AgentCmd            `cmd:"" help:"Run agent"`
	Once             OnceCmd             `cmd:"" help:"Run once"`
	Lambda           LambdaCmd           `cmd:"" help:"Run on AWS Lambda like once mode"`
	Ping             PingCmd             `cmd:"" help:"Run ping probe"`
	TCP              TCPCmd              `cmd:"" help:"Run TCP probe"`
	HTTP             HTTPCmd             `cmd:"" help:"Run HTTP probe"`
	FirehoseEndpoint FirehoseEndpointCmd `cmd:"" help:"Run Firehose HTTP endpoint"`
}

type VersionCmd struct{}

type AgentCmd struct {
	Config               string `short:"c" help:"configuration file path or URL(http|s3)" env:"CONFIG"`
	WithFirehoseEndpoint bool   `help:"run with firehose HTTP endpoint server"`
	Port                 int    `help:"firehose HTTP endpoint listen port" default:"8080"`
}

type OnceCmd struct {
	Config string `short:"c" help:"configuration file path or URL(http|s3)" env:"CONFIG"`
}

type LambdaCmd struct {
	Config string `short:"c" help:"configuration file path or URL(http|s3)" env:"CONFIG"`
}

type PingCmd struct {
	Address string        `arg:"" help:"Hostname or IP address" required:""`
	Count   int           `short:"c" help:"Iteration count"`
	Timeout time.Duration `short:"t" help:"Timeout to ping response"`
	HostID  string        `short:"i" help:"Mackerel host ID"`
}

type TCPCmd struct {
	Host               string        `arg:"" help:"Hostname or IP address" required:""`
	Port               string        `arg:"" help:"Port number" required:""`
	Send               string        `short:"s" help:"String to send to the server"`
	Quit               string        `short:"q" help:"String to send server to initiate a clean close of the connection"`
	Timeout            time.Duration `short:"t" help:"Timeout"`
	ExpectPattern      string        `short:"e" help:"Regexp pattern to expect in server response"`
	NoCheckCertificate bool          `short:"k" help:"Do not check certificate"`
	HostID             string        `short:"i" help:"Mackerel host ID"`
	TLS                bool          `help:"Use TLS"`
}

type HTTPCmd struct {
	URL                string            `arg:"" help:"URL" required:""`
	Method             string            `short:"m" help:"Request method" default:"GET"`
	Body               string            `short:"b" help:"Request body"`
	ExpectPattern      string            `short:"e" help:"Regexp pattern to expect in server response"`
	Timeout            time.Duration     `short:"t" help:"Timeout"`
	NoCheckCertificate bool              `short:"k" help:"Do not check certificate"`
	Headers            map[string]string `short:"H" help:"Request headers" placeholder:"Header: Value"`
	HostID             string            `short:"i" help:"Mackerel host ID"`
}

type FirehoseEndpointCmd struct {
	Port int `short:"p" help:"Listen port" default:"8080"`
}

var retryPolicy = retry.Policy{
	MinDelay: 1 * time.Second,
	MaxDelay: 10 * time.Second,
	MaxCount: 5,
}

func lock() {
	sem <- struct{}{}
	log.Printf("[trace] locked. concurrency: %d", len(sem))
}

func unlock() {
	<-sem
	log.Printf("[trace] unlocked. concurrency: %d", len(sem))
}

func Run(ctx context.Context, wg *sync.WaitGroup, configPath string, once bool) error {
	defer wg.Done()
	defer log.Println("[info] stopping maprobe")

	log.Println("[info] starting maprobe")
	conf, confDigest, err := LoadConfig(configPath)
	if err != nil {
		return err
	}
	log.Println("[debug]", conf.String())
	client := newClient(MackerelAPIKey, conf.Backup.FirehoseStreamName)

	chs := NewChannels(conf.Destination)
	defer chs.Close()

	if len(conf.Probes) > 0 {
		if conf.PostProbedMetrics {
			if conf.Destination.Mackerel.Enabled {
				wg.Add(2)
				go postHostMetricWorker(ctx, wg, client, chs)
				go postServiceMetricWorker(ctx, wg, client, chs)
			}
			if conf.Destination.Otel.Enabled {
				wg.Add(1)
				go postOtelMetricWorker(ctx, wg, chs, conf.Destination.Otel)
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
			go pd.RunProbes(ctx, client, chs, &wg2)
		}
		for _, ag := range conf.Aggregates {
			wg2.Add(1)
			go runAggregates(ctx, ag, client, chs, &wg2)
		}
		wg2.Wait()
		if once {
			return nil
		}

		log.Println("[debug] waiting for a next tick")
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		log.Println("[debug] checking a new config")
		newConf, digest, err := LoadConfig(configPath)
		if err != nil {
			log.Println("[warn]", err)
			log.Println("[warn] still using current config")
		} else if confDigest != digest {
			conf = newConf
			confDigest = digest
			log.Println("[info] config reloaded")
			log.Println("[debug]", conf)
		}
	}
}

func runAggregates(_ context.Context, ag *AggregateDefinition, client *Client, chs *Channels, wg *sync.WaitGroup) {
	defer wg.Done()

	service := ag.Service.String()
	roles := exStrings(ag.Roles)
	statuses := exStrings(ag.Statuses)
	log.Printf(
		"[debug] aggregates finding hosts service:%s roles:%s statuses:%v",
		service,
		roles,
		statuses,
	)

	hosts, err := client.FindHosts(&mackerel.FindHostsParam{
		Service:  service,
		Roles:    roles,
		Statuses: statuses,
	})
	if err != nil {
		log.Println("[error] aggregates find hosts failed", err)
		return
	}
	log.Printf("[debug] aggregates %d hosts found", len(hosts))

	hostIDs := make([]string, 0, len(hosts))
	for _, h := range hosts {
		hostIDs = append(hostIDs, h.ID)
	}
	metricNames := make([]string, 0, len(ag.Metrics))
	for _, m := range ag.Metrics {
		metricNames = append(metricNames, m.Name.String())
	}

	log.Printf("[debug] fetching latest metrics hosts:%v metrics:%v", hostIDs, metricNames)

	// TODO: If latest API will returns metrics refreshed at on minute,
	// We will replace to client.FetchLatestMetricValues().
	latest, err := client.fetchLatestMetricValues(hostIDs, metricNames)
	if err != nil {
		log.Printf("[error] fetch latest metrics failed. %s hosts:%v metrics:%v", err, hostIDs, metricNames)
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
					log.Printf("[trace] latest %s:%s is not found", hostID, name)
					continue
				}
				v, ok := _v.Value.(float64)
				if !ok {
					log.Printf("[warn] latest %s:%s = %v is not a float64 value", hostID, name, _v)
					continue
				}
				ts := time.Unix(_v.Time, 0)
				log.Printf("[trace] latest %s:%s:%d = %f", hostID, name, _v.Time, v)
				if ts.After(now.Add(metricTimeMargin)) {
					values = append(values, v)
					if _v.Time > timestamp {
						timestamp = _v.Time
					}
				} else {
					log.Printf("[warn] latest %s:%s at %s is outdated", hostID, name, ts)
				}
			}
		}
		if len(hosts) > 0 && len(values) == 0 {
			log.Printf("[warn] %s:%s latest values are not found", ag.Service, mc.Name)
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
			log.Printf("[debug] aggregates %s(%s)=%f -> %s:%s timestamp %d",
				output.Func, name, value,
				ag.Service, output.Name,
				timestamp,
			)
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
	log.Println("[info] starting postHostMetricWorker")
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
				log.Println("[info] shutting down postHostMetricWorker")
				run = false
			}
		case <-ticker.C:
		}
		if len(mvs) == 0 {
			continue
		}
		log.Printf("[debug] posting %d host metrics to Mackerel", len(mvs))
		b, _ := json.Marshal(mvs)
		log.Println("[debug]", string(b))
		if err := doRetry(ctx, func() error {
			return client.PostHostMetricValues(mvs)
		}); err != nil {
			log.Printf("[error] failed to post host metrics to Mackerel %s", err)
			continue
		}
		log.Printf("[debug] post host metrics succeeded.")
		// success. reset buffer
		mvs = mvs[:0]
	}
}

func postServiceMetricWorker(ctx context.Context, wg *sync.WaitGroup, client *Client, chs *Channels) {
	log.Println("[info] starting postServiceMetricWorker")
	defer wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	mvsMap := make(map[string][]*mackerel.MetricValue)
	run := true
	for run {
		select {
		case m, cont := <-chs.ServiceMetrics:
			if cont {
				if math.IsNaN(m.Value) {
					log.Printf(
						"[warn] %s:%s value NaN is not supported by Mackerel",
						m.Service, m.Name,
					)
					continue
				} else {
					mvsMap[m.Service] = append(mvsMap[m.Service], m.MetricValue())
				}
				if len(mvsMap[m.Service]) < PostMetricBufferLength {
					continue
				}
			} else {
				log.Println("[info] shutting down postServiceMetricWorker")
				run = false
			}
		case <-ticker.C:
		}

		for serviceName, mvs := range mvsMap {
			if len(mvs) == 0 {
				continue
			}
			log.Printf("[debug] posting %d service metrics to Mackerel:%s", len(mvs), serviceName)
			b, _ := json.Marshal(mvs)
			log.Println("[debug]", string(b))
			if err := doRetry(ctx, func() error {
				return client.PostServiceMetricValues(serviceName, mvs)
			}); err != nil {
				log.Printf("[error] failed to post service metrics to Mackerel:%s %s", serviceName, err)
				continue
			}
			log.Printf("[debug] post service succeeded.")
			// success. reset buffer
			mvs = mvs[:0]
			mvsMap[serviceName] = mvs
		}
	}
}

func postOtelMetricWorker(ctx context.Context, wg *sync.WaitGroup, chs *Channels, oc *OtelConfig) {
	defer wg.Done()
	exporter, endpointURL, err := newOtelExporter(ctx, oc)
	if err != nil {
		log.Printf("[error] failed to create OpenTelemetry meter exporter: %v", err)
		return
	}
	defer exporter.Shutdown(ctx)
	log.Printf("[info] starting postOtelMetricWorker endpoint %s", endpointURL)
	attrs := otelresource.NewSchemaless()

	ticker := time.NewTicker(10 * time.Second)
	mvs := make([]otelmetricdata.Metrics, 0, PostMetricBufferLength)
	run := true
	for run {
		select {
		case m, cont := <-chs.OtelMetrics:
			if cont {
				mvs = append(mvs, m.Otel())
				log.Println("[debug][otel]", m.OtelString())
				if len(mvs) < PostMetricBufferLength {
					continue
				}
			} else {
				log.Println("[info] shutting down postOtelMetricWorker")
				run = false
			}
		case <-ticker.C:
		}
		if len(mvs) == 0 {
			continue
		}
		log.Printf("[debug] posting %d otel metrics to %s", len(mvs), endpointURL)
		rms := &otelmetricdata.ResourceMetrics{
			Resource: attrs,
			ScopeMetrics: []otelmetricdata.ScopeMetrics{
				{Metrics: mvs},
			},
		}
		if err := doRetry(ctx, func() error {
			return exporter.Export(ctx, rms)
		}); err != nil {
			log.Printf("[error] failed to export otel metrics: %v", err)
			continue
		}
		log.Printf("[debug] post otel metrics succeeded.")
		// success. reset buffer
		mvs = mvs[:0]
	}
}

func newOtelExporter(ctx context.Context, oc *OtelConfig) (*otlpmetricgrpc.Exporter, string, error) {
	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithHeaders(map[string]string{"Mackerel-Api-Key": MackerelAPIKey}),
		otlpmetricgrpc.WithCompressor("gzip"),
	}
	var endpointURL = url.URL{
		Scheme: "https",
		Host:   oc.Endpoint,
	}
	if oc.Endpoint != "" {
		opts = append(opts, otlpmetricgrpc.WithEndpoint(oc.Endpoint))
	} else {
		// TODO fix to use Mackrel when it is GA
		opts = append(opts, otlpmetricgrpc.WithEndpoint("localhost:4317"))
		endpointURL.Host = "localhost:4317"
	}
	if oc.Insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
		endpointURL.Scheme = "http"
	}
	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return nil, "", err
	}
	return exporter, endpointURL.String(), nil
}

func dumpHostMetricWorker(_ context.Context, wg *sync.WaitGroup, chs *Channels) {
	defer wg.Done()
	log.Println("[info] starting dumpHostMetricWorker")
	for m := range chs.HostMetrics {
		b, _ := json.Marshal(m.HostMetricValue())
		log.Printf("[info][host] %s %s", m.HostID, b)
	}
}

func dumpServiceMetricWorker(_ context.Context, wg *sync.WaitGroup, chs *Channels) {
	defer wg.Done()
	log.Println("[info] starting dumpServiceMetricWorker")
	for m := range chs.ServiceMetrics {
		b, _ := json.Marshal(m.MetricValue())
		log.Printf("[info][service] %s %s", m.Service, b)
	}
}

func dumpOtelMetricWorker(_ context.Context, wg *sync.WaitGroup, chs *Channels) {
	defer wg.Done()
	log.Println("[info] starting dumpOtelMetricWorker")
	for m := range chs.OtelMetrics {
		log.Printf("[info][otel] %s", m.OtelString())
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
		log.Printf("[warn] retrying: %s", err)
	}
	if r.Err() != nil {
		return r.Err()
	}
	return fmt.Errorf("retry failed: %w", err)
}
