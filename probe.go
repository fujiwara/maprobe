package maprobe

import (
	"context"
	"fmt"
	"strings"
	"text/template"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"
)

type Probe interface {
	Run(ctx context.Context) (Metrics, error)
	HostID() string
	MetricName(string) string
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

func expandPlaceHolder(src string, data interface{}) (string, error) {
	if strings.Index(src, "{{") == -1 {
		// no need to expand
		return src, nil
	}
	tmpl, err := template.New(src).Parse(src)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.Grow(len(src))
	err = tmpl.Execute(&b, data)
	return b.String(), err
}

func newMetric(p Probe, name string, value float64, ts time.Time) Metric {
	return Metric{
		HostID:    p.HostID(),
		Name:      p.MetricName(name),
		Value:     value,
		Timestamp: ts,
	}
}
