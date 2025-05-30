package maprobe

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"text/template"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"
)

type Probe interface {
	Run(ctx context.Context) (Metrics, error)
	MetricName(string) string
}

func newMetric(p Probe, name string, value float64) Metric {
	return Metric{
		Name:      p.MetricName(name),
		Value:     value,
		Timestamp: time.Now(),
	}
}

type ProbeConfig interface {
	GenerateProbe(host *mackerel.Host) (Probe, error)
}

var (
	expandCache = sync.Map{}
)

func newFuncMap(env map[string]string) template.FuncMap {
	return template.FuncMap{
		"env": func(keys ...string) string {
			v := ""
			for _, k := range keys {
				if v, ok := env[k]; ok {
					return v
				}
				if v := os.Getenv(k); v != "" {
					return v
				}
				v = k
			}
			return v
		},
		"must_env": func(key string) string {
			if v, ok := env[key]; ok {
				return v
			} else if v, ok := os.LookupEnv(key); ok {
				return v
			}
			panic(fmt.Sprintf("environment variable %s is not defined", key))
		},
	}
}

func expandCacheKey(src string, env map[string]string) string {
	if env == nil {
		return src
	}
	return fmt.Sprintf("%s%s", env, src)
}

func expandPlaceHolder(src string, host *mackerel.Host, env map[string]string) (string, error) {
	var err error

	if !strings.Contains(src, "{{") {
		// no need to expand
		return src, nil
	}

	var tmpl *template.Template
	key := expandCacheKey(src, env)
	if _tmpl, ok := expandCache.Load(key); ok {
		slog.Debug("expand cache HIT", "key", key)
		tmpl = _tmpl.(*template.Template)
	} else {
		slog.Debug("expand cache MISS", "key", key)
		tmpl, err = template.New(key).Funcs(newFuncMap(env)).Parse(src)
		if err != nil {
			return "", err
		}
		expandCache.Store(src, tmpl)
	}
	var b strings.Builder
	b.Grow(len(src))
	err = tmpl.Execute(&b, templateParam{Host: host})
	return b.String(), err
}

func (pd *ProbeDefinition) RunProbes(ctx context.Context, client *Client, chs *Channels, stats *StatsCollector, wg *sync.WaitGroup) {
	defer wg.Done()
	if pd.IsServiceMetric {
		for _, m := range pd.RunServiceProbes(ctx, client, stats) {
			chs.SendServiceMetric(m)
		}
	} else {
		for _, m := range pd.RunHostProbes(ctx, client, stats) {
			chs.SendHostMetric(m)
		}
	}
}

func (pd *ProbeDefinition) RunHostProbes(ctx context.Context, client *Client, stats *StatsCollector) []HostMetric {
	slog.Debug("probes finding hosts", "service", pd.Service, "roles", pd.Roles, "statuses", pd.Statuses)
	roles := exStrings(pd.Roles)
	statuses := exStrings(pd.Statuses)
	ms := []HostMetric{}

	hosts, err := client.FindHosts(&mackerel.FindHostsParam{
		Service:  pd.Service.String(),
		Roles:    roles,
		Statuses: statuses,
	})
	if err != nil {
		slog.Error("probes find host failed", "error", err)
		return nil
	}
	slog.Debug("probes hosts found", "count", len(hosts))
	// Update target hosts count for stats
	stats.SetTargetCounts(int64(len(hosts)), 0)
	if len(hosts) == 0 {
		return nil
	}

	spawnInterval := time.Duration(int64(ProbeInterval) / int64(len(hosts)) / 2)
	if spawnInterval > time.Second {
		spawnInterval = time.Second
	}

	wg := &sync.WaitGroup{}
	for _, host := range hosts {
		time.Sleep(spawnInterval)
		slog.Debug("probes preparing host", "hostID", host.ID, "hostName", host.Name)
		wg.Add(1)
		go func(host *mackerel.Host) {
			lock()
			defer unlock()
			defer wg.Done()
			for _, probe := range pd.GenerateProbes(host, client.mackerel) {
				slog.Debug("probing host", "hostID", host.ID, "hostName", host.Name, "probe", probe)
				metrics, err := probe.Run(ctx)
				
				// Update probe execution stats
				if err != nil {
					slog.Warn("probe failed", "error", err, "hostID", host.ID, "hostName", host.Name, "probe", probe)
				}
				stats.RecordProbeExecution(ctx, probe, err)
				for _, m := range metrics {
					m.Attribute = &Attribute{
						Service: pd.Service.String(),
						HostID:  host.ID,
					}
					m.Attribute.SetExtra(pd.Attributes, host)
					ms = append(ms, m.HostMetric(host.ID))
					
					// Update metrics collected counter
					stats.RecordMetricCollected(ctx)
				}
			}
		}(host)
	}
	wg.Wait()
	return ms
}

func (pd *ProbeDefinition) RunServiceProbes(ctx context.Context, client *Client, stats *StatsCollector) []ServiceMetric {
	serviceName := pd.Service.String()
	slog.Debug("probes for service metric", "service", serviceName)
	// Update target services count for stats (set to 1 for this service)
	stats.SetTargetCounts(0, 1)
	lock()
	defer unlock()
	host := &mackerel.Host{
		Name: serviceName,
		ID:   serviceName,
	}
	ms := []ServiceMetric{}
	for _, probe := range pd.GenerateProbes(host, client.mackerel) {
		slog.Debug("probing service", "service", serviceName, "probe", probe)
		metrics, err := probe.Run(ctx)
		
		// Update probe execution stats
		if err != nil {
			slog.Warn("probe failed", "error", err, "service", serviceName, "probe", probe)
		}
		stats.RecordProbeExecution(ctx, probe, err)
		for _, m := range metrics {
			m.Attribute = &Attribute{
				Service: serviceName,
			}
			m.Attribute.SetExtra(pd.Attributes, host)
			ms = append(ms, m.ServiceMetric(serviceName))
			
			// Update metrics collected counter
			stats.RecordMetricCollected(ctx)
		}
	}
	return ms
}
