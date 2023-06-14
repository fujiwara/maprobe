package maprobe

import (
	"fmt"
	"strings"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"
)

type Metric struct {
	Name      string
	Value     float64
	Timestamp time.Time
}

func (m Metric) String() string {
	return fmt.Sprintf("%s\t%f\t%d", m.Name, m.Value, m.Timestamp.Unix())
}

func (m Metric) ServiceMetric(service string) ServiceMetric {
	return ServiceMetric{
		Service: service,
		Metric:  m,
	}
}

func (m Metric) HostMetric(hostID string) HostMetric {
	return HostMetric{
		HostID: hostID,
		Metric: m,
	}
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
	HostID string
	Metric
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

type ServiceMetrics []ServiceMetric

func (ms ServiceMetrics) String() string {
	var b strings.Builder
	for _, m := range ms {
		b.WriteString(m.String())
		b.WriteString("\n")
	}
	return b.String()
}

type ServiceMetric struct {
	Service string
	Metric
}

func (m ServiceMetric) MetricValue() *mackerel.MetricValue {
	return &mackerel.MetricValue{
		Name:  m.Name,
		Time:  m.Timestamp.Unix(),
		Value: m.Value,
	}
}

func (m ServiceMetric) String() string {
	return fmt.Sprintf("%s\t%f\t%d", m.Name, m.Value, m.Timestamp.Unix())
}
