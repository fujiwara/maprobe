package maprobe

import (
	"fmt"
	"log"
	"strings"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"

	otelattribute "go.opentelemetry.io/otel/attribute"
	otelmetricdata "go.opentelemetry.io/otel/sdk/metric/metricdata"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
)

type Metric struct {
	Name      string
	Value     float64
	Timestamp time.Time
	Attribute *Attribute
}

func (m Metric) Otel() otelmetricdata.Metrics {
	return otelmetricdata.Metrics{
		Name: m.Name,
		Data: otelmetricdata.Gauge[float64]{
			DataPoints: []otelmetricdata.DataPoint[float64]{
				{
					Attributes: *m.Attribute.Otel(),
					Time:       m.Timestamp,
					Value:      m.Value,
				},
			},
		},
	}
}

func (m Metric) OtelString() string {
	// promhttp_metric_handler_requests_total{code="200"} 988
	return fmt.Sprintf("%s{%s} %f", m.Name, m.Attribute, m.Value)
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

type Attribute struct {
	Service string
	HostID  string
	Extra   map[string]string
}

func (a *Attribute) SetExtra(ex map[string]string, host *mackerel.Host) {
	if len(ex) == 0 {
		return
	}
	a.Extra = make(map[string]string, len(ex))
	for k, v := range ex {
		vv, err := expandPlaceHolder(v, host, nil)
		if err != nil {
			log.Printf("[error] cannot expand placeholder %s: %s", v, err)
			continue
		}
		a.Extra[k] = vv
	}
}

func (a *Attribute) Otel() *otelattribute.Set {
	kvs := make([]otelattribute.KeyValue, 0, len(a.Extra)+2)
	for k, v := range a.Extra {
		kvs = append(kvs, otelattribute.String(k, v))
	}
	kvs = append(kvs,
		semconv.ServiceName(a.Service),
		semconv.HostID(a.HostID),
	)
	s := otelattribute.NewSet(kvs...)
	return &s
}

func (a Attribute) String() string {
	return a.Otel().Encoded(otelattribute.DefaultEncoder())
}
