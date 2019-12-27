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
	Run(ctx context.Context) (HostMetrics, error)
	HostID() string
	MetricName(string) string
}

type ProbeConfig interface {
	GenerateProbe(host *mackerel.Host) (Probe, error)
}

type HostMetrics []HostMetric

func (ms HostMetrics) String() string {
	var b strings.Builder
	for _, m := range ms {
		b.WriteString(m.String())
		b.WriteString("\n")
	}
	return b.String()
}

type HostMetric struct {
	HostID    string
	Name      string
	Value     float64
	Timestamp time.Time
}

func (m HostMetric) HostMetricValue() *mackerel.HostMetricValue {
	mv := &mackerel.MetricValue{
		Name:  m.Name,
		Time:  m.Timestamp.Unix(),
		Value: m.Value,
	}
	return &mackerel.HostMetricValue{
		HostID:      m.HostID,
		MetricValue: mv,
	}
}

func (m HostMetric) String() string {
	return fmt.Sprintf("%s\t%f\t%d", m.Name, m.Value, m.Timestamp.Unix())
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

	if strings.Index(src, "{{") == -1 {
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

func newMetric(p Probe, name string, value float64) HostMetric {
	return HostMetric{
		HostID:    p.HostID(),
		Name:      p.MetricName(name),
		Value:     value,
		Timestamp: time.Now(),
	}
}
