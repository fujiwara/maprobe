package maprobe

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"
	"fmt"
	fping "github.com/tatsushid/go-fastping"
)

var (
	DefaultPingTimeout    = time.Second
	DefaultPingCount      = 3
	DefaultPingMetricName = "ping"
)

type PingProbeConfig struct {
	Address         string        `yaml:"address"`
	Count           int           `yaml:"count"`
	Timeout         time.Duration `yaml:"timeout"`
	MetricKeyPrefix string        `yaml:"metric_key_prefix"`
}

func (pc *PingProbeConfig) GenerateProbe(host *mackerel.Host) (Probe, error) {
	p := &PingProbe{
		metricKeyPrefix: pc.MetricKeyPrefix,
		Count:           pc.Count,
		Timeout:         pc.Timeout,
	}
	if addr, err := expandPlaceHolder(pc.Address, host, nil); err != nil {
		return nil, err
	} else {
		p.Address = addr
	}
	if p.Address == "" {
		return nil, fmt.Errorf("no address")
	}

	if p.Count == 0 {
		p.Count = DefaultPingCount
	}
	if p.Timeout == 0 {
		p.Timeout = DefaultPingTimeout
	}
	if p.metricKeyPrefix == "" {
		p.metricKeyPrefix = DefaultPingMetricName
	}
	return p, nil
}

type PingProbe struct {
	metricKeyPrefix string

	Address string
	Count   int
	Timeout time.Duration
}

func (p *PingProbe) MetricName(name string) string {
	return p.metricKeyPrefix + "." + name
}

func (p *PingProbe) String() string {
	b, _ := json.Marshal(p)
	return string(b)
}

func (p *PingProbe) Run(ctx context.Context) (Metrics, error) {
	var ms Metrics

	slog.Debug("run ping", "address", p.Address)
	pinger := fping.NewPinger()
	ipaddr, err := net.ResolveIPAddr("ip", p.Address)
	if err != nil {
		ms = append(ms, newMetric(p, "count.success", 0))
		ms = append(ms, newMetric(p, "count.failure", 1))
		return ms, fmt.Errorf("resolve failed: %w", err)
	}
	slog.Debug("address resolved", "address", p.Address, "ipaddr", ipaddr)
	pinger.AddIPAddr(ipaddr)

	var min, max, total, avg time.Duration
	var successCount, failureCount int
	pinger.MaxRTT = p.Timeout
	pinger.OnRecv = func(addr *net.IPAddr, rtt time.Duration) {
		slog.Debug("ping response received", "rtt", rtt)
		successCount++
		if min == 0 || max == 0 {
			min = rtt
			max = rtt
		}
		if rtt < min {
			min = rtt
		}
		if max < rtt {
			max = rtt
		}
		total = total + rtt
	}
	for i := 0; i < p.Count; i++ {
		select {
		case <-ctx.Done():
			return ms, nil
		default:
		}
		err := pinger.Run()
		if err != nil {
			failureCount++
			slog.Warn("ping failed", "address", p.Address, "ipaddr", ipaddr, "error", err)
		}
	}
	if successCount != 0 {
		avg = time.Duration(int64(total) / int64(successCount))
	}

	ms = append(ms, newMetric(p, "count.success", float64(successCount)))
	ms = append(ms, newMetric(p, "count.failure", float64(failureCount)))
	if min > 0 || max > 0 || avg > 0 {
		ms = append(ms, newMetric(p, "rtt.min", min.Seconds()))
		ms = append(ms, newMetric(p, "rtt.max", max.Seconds()))
		ms = append(ms, newMetric(p, "rtt.avg", avg.Seconds()))
	}
	slog.Debug("ping probe completed", "metrics", ms.String())

	return ms, nil
}
