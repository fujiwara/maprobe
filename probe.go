package maprobe

import (
	"context"
	"fmt"
	"log"
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
		log.Println("[trace] expand cache HIT", key)
		tmpl = _tmpl.(*template.Template)
	} else {
		log.Println("[trace] expand cache MISS", key)
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

func (pd *ProbeDefinition) RunProbes(ctx context.Context, client *Client, chs *Channels, wg *sync.WaitGroup) {
	defer wg.Done()
	if pd.IsServiceMetric {
		for _, m := range pd.RunServiceProbes(ctx, client) {
			m.Attribute.Service = pd.Service.String()
			chs.SendServiceMetric(m)
		}
	} else {
		for _, m := range pd.RunHostProbes(ctx, client) {
			m.Attribute.Service = pd.Service.String()
			m.Attribute.Role = pd.Role.String()
			m.Attribute.HostID = m.HostID
			chs.SendHostMetric(m)
		}
	}
}

func (pd *ProbeDefinition) RunHostProbes(ctx context.Context, client *Client) []HostMetric {
	log.Printf(
		"[debug] probes finding hosts service:%s roles:%s statuses:%v",
		pd.Service,
		pd.Roles,
		pd.Statuses,
	)
	roles := exStrings(pd.Roles)
	statuses := exStrings(pd.Statuses)
	ms := []HostMetric{}

	hosts, err := client.FindHosts(&mackerel.FindHostsParam{
		Service:  pd.Service.String(),
		Roles:    roles,
		Statuses: statuses,
	})
	if err != nil {
		log.Println("[error] probes find host failed", err)
		return nil
	}
	log.Printf("[debug] probes %d hosts found", len(hosts))
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
		log.Printf("[debug] probes preparing host id:%s name:%s", host.ID, host.Name)
		wg.Add(1)
		go func(host *mackerel.Host) {
			lock()
			defer unlock()
			defer wg.Done()
			for _, probe := range pd.GenerateProbes(host, client.mackerel) {
				log.Printf("[debug] probing host id:%s name:%s probe:%s", host.ID, host.Name, probe)
				metrics, err := probe.Run(ctx)
				if err != nil {
					log.Printf("[warn] probe failed. %s host id:%s name:%s probe:%s", err, host.ID, host.Name, probe)
				}
				for _, m := range metrics {
					ms = append(ms, m.HostMetric(host.ID))
				}
			}
		}(host)
	}
	wg.Wait()
	return ms
}

func (pd *ProbeDefinition) RunServiceProbes(ctx context.Context, client *Client) []ServiceMetric {
	serviceName := pd.Service.String()
	log.Printf(
		"[debug] probes for service metric service:%s",
		serviceName,
	)
	lock()
	defer unlock()
	host := &mackerel.Host{
		Name: serviceName,
		ID:   serviceName,
	}
	ms := []ServiceMetric{}
	for _, probe := range pd.GenerateProbes(host, client.mackerel) {
		log.Printf("[debug] probing service:%s probe:%s", serviceName, probe)
		metrics, err := probe.Run(ctx)
		if err != nil {
			log.Printf("[warn] probe failed. %s service:%s probe:%s", err, serviceName, probe)
		}
		for _, m := range metrics {
			ms = append(ms, m.ServiceMetric(serviceName))
		}
	}
	return ms
}
