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

var funcMap = template.FuncMap{
	"env": func(keys ...string) string {
		v := ""
		for _, k := range keys {
			v = os.Getenv(k)
			if v != "" {
				return v
			}
			v = k
		}
		return v
	},
	"must_env": func(key string) string {
		if v, ok := os.LookupEnv(key); ok {
			return v
		}
		panic(fmt.Sprintf("environment variable %s is not defined", key))
	},
}

func expandPlaceHolder(src string, host *mackerel.Host) (string, error) {
	var err error

	if strings.Index(src, "{{") == -1 {
		// no need to expand
		return src, nil
	}

	var tmpl *template.Template
	if _tmpl, ok := expandCache.Load(src); ok {
		log.Println("[trace] expand cache HIT", src)
		tmpl = _tmpl.(*template.Template)
	} else {
		log.Println("[trace] expand cache MISS", src)
		tmpl, err = template.New(src).Funcs(funcMap).Parse(src)
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
