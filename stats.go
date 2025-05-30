package maprobe

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"

	otelattribute "go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
	otelsdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// StatsCollector manages maprobe internal metrics
type StatsCollector struct {
	meter                   otelmetric.Meter
	probeConfigsGauge       otelmetric.Int64ObservableGauge
	targetHostsGauge        otelmetric.Int64ObservableGauge
	targetServicesGauge     otelmetric.Int64ObservableGauge
	metricsCollectedCounter otelmetric.Int64Counter
	probeExecutionsCounter  otelmetric.Int64Counter

	// atomic values
	currentProbeConfigs   int64
	currentTargetHosts    int64
	currentTargetServices int64
}

// SetProbeConfigs sets the number of configured probes
func (s *StatsCollector) SetProbeConfigs(count int64) {
	if s == nil {
		return
	}
	atomic.StoreInt64(&s.currentProbeConfigs, count)
	slog.Debug("stats: probe configs updated", "count", count)
}

// SetTargetCounts sets the number of target hosts and services
func (s *StatsCollector) SetTargetCounts(hosts, services int64) {
	if s == nil {
		return
	}
	atomic.StoreInt64(&s.currentTargetHosts, hosts)
	atomic.StoreInt64(&s.currentTargetServices, services)
	slog.Debug("stats: target counts updated", "hosts", hosts, "services", services)
}

// RecordProbeExecution records a probe execution result
func (s *StatsCollector) RecordProbeExecution(ctx context.Context, probe Probe, probeErr error) {
	if s == nil || s.probeExecutionsCounter == nil {
		return
	}

	probeType := getProbeType(probe)
	status := "success"
	if probeErr != nil {
		status = "error"
	}

	s.probeExecutionsCounter.Add(ctx, 1,
		otelmetric.WithAttributes(
			otelattribute.String("status", status),
			otelattribute.String("probe_type", probeType)))
	slog.Debug("stats: probe execution recorded", "status", status, "probe_type", probeType)
}

// RecordMetricCollected records that a metric was collected
func (s *StatsCollector) RecordMetricCollected(ctx context.Context) {
	if s == nil || s.metricsCollectedCounter == nil {
		return
	}
	s.metricsCollectedCounter.Add(ctx, 1)
	slog.Debug("stats: metric collected")
}

// getProbeType returns the probe type string
func getProbeType(probe Probe) string {
	switch probe.(type) {
	case *HTTPProbe:
		return "http"
	case *TCPProbe:
		return "tcp"
	case *PingProbe:
		return "ping"
	case *CommandProbe:
		return "command"
	default:
		return "unknown"
	}
}

// NewStatsCollector creates a new StatsCollector
func NewStatsCollector(provider *otelsdkmetric.MeterProvider, attrs map[string]string) (*StatsCollector, error) {
	if provider == nil {
		return nil, nil
	}
	meterOpts := make([]otelmetric.MeterOption, 0, len(attrs))
	for k, v := range attrs {
		meterOpts = append(meterOpts, otelmetric.WithInstrumentationAttributes(otelattribute.String(k, v)))
	}

	s := &StatsCollector{
		meter: provider.Meter("maprobe/stats", meterOpts...),
	}

	var err error
	s.probeConfigsGauge, err = s.meter.Int64ObservableGauge(
		"maprobe_probe_configs",
		otelmetric.WithDescription("Number of configured probes"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create probe_configs gauge: %w", err)
	}

	s.targetHostsGauge, err = s.meter.Int64ObservableGauge(
		"maprobe_target_hosts",
		otelmetric.WithDescription("Number of target hosts"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create target_hosts gauge: %w", err)
	}

	s.targetServicesGauge, err = s.meter.Int64ObservableGauge(
		"maprobe_target_services",
		otelmetric.WithDescription("Number of target services"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create target_services gauge: %w", err)
	}

	s.metricsCollectedCounter, err = s.meter.Int64Counter(
		"maprobe_metrics_collected_total",
		otelmetric.WithDescription("Total number of metrics collected"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics_collected counter: %w", err)
	}

	s.probeExecutionsCounter, err = s.meter.Int64Counter(
		"maprobe_probe_executions_total",
		otelmetric.WithDescription("Total number of probe executions"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create probe_executions counter: %w", err)
	}

	// Register observable gauge callbacks
	_, err = s.meter.RegisterCallback(
		func(ctx context.Context, o otelmetric.Observer) error {
			o.ObserveInt64(s.probeConfigsGauge, atomic.LoadInt64(&s.currentProbeConfigs))
			o.ObserveInt64(s.targetHostsGauge, atomic.LoadInt64(&s.currentTargetHosts))
			o.ObserveInt64(s.targetServicesGauge, atomic.LoadInt64(&s.currentTargetServices))
			return nil
		},
		s.probeConfigsGauge, s.targetHostsGauge, s.targetServicesGauge,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register gauge callbacks: %w", err)
	}

	return s, nil
}
