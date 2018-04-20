package maprobe

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"text/template"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"
)

type Probe interface {
	Run(ctx context.Context) (Metrics, error)
	HostID() string
	MetricName(string) string
}

type ProbeConfig interface {
	GenerateProbe(host *mackerel.Host) (Probe, error)
}

type Metrics []Metric

func (ms Metrics) String() string {
	var b strings.Builder
	for _, m := range ms {
		b.WriteString(m.String())
		b.WriteString("\n")
	}
	return b.String()
}

type Metric struct {
	HostID    string
	Name      string
	Value     float64
	Timestamp time.Time
}

func (m Metric) HostMetricValue() *mackerel.HostMetricValue {
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

func (m Metric) String() string {
	return fmt.Sprintf("%s\t%f\t%d", m.Name, m.Value, m.Timestamp.Unix())
}

var (
	expandMutex sync.Mutex
	expandCache = make(map[string]*template.Template)
)

func expandPlaceHolder(src string, host *mackerel.Host) (string, error) {
	var err error

	if strings.Index(src, "{{") == -1 {
		// no need to expand
		return src, nil
	}

	expandMutex.Lock()
	defer expandMutex.Unlock()

	tmpl := expandCache[src]
	if tmpl == nil {
		log.Println("[trace] expand cache MISS", src)
		tmpl, err = template.New(src).Parse(src)
		if err != nil {
			return "", err
		}
		expandCache[src] = tmpl
	} else {
		log.Println("[trace] expand cache HIT", src)
	}
	var b strings.Builder
	b.Grow(len(src))
	err = tmpl.Execute(&b, templateParam{Host: host})
	return b.String(), err
}

func newMetric(p Probe, name string, value float64) Metric {
	return Metric{
		HostID:    p.HostID(),
		Name:      p.MetricName(name),
		Value:     value,
		Timestamp: time.Now(),
	}
}
