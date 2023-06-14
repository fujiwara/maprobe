package maprobe

import (
	"fmt"
	"strings"

	mackerel "github.com/mackerelio/mackerel-client-go"
)

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
