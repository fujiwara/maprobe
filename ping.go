package maprobe

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"
	"github.com/pkg/errors"
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

func (pc *PingProbeConfig) GenerateProbe(host *mackerel.Host) (*PingProbe, error) {
	p := &PingProbe{
		hostID:          host.ID,
		metricKeyPrefix: pc.MetricKeyPrefix,
		Count:           pc.Count,
		Timeout:         pc.Timeout,
	}
	if addr, err := expandPlaceHolder(pc.Address, host); err != nil {
		return nil, err
	} else {
		p.Address = addr
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
	hostID          string
	metricKeyPrefix string

	Address string
	Count   int
	Timeout time.Duration
}

func (p *PingProbe) HostID() string {
	return p.hostID
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

	log.Printf("[debug] run ping to %s", p.Address)
	pinger := fping.NewPinger()
	ipaddr, err := net.ResolveIPAddr("ip", p.Address)
	if err != nil {
		ms = append(ms, newMetric(p, "count.success", 0))
		ms = append(ms, newMetric(p, "count.failure", 1))
		return ms, errors.Wrap(err, "resolve failed")
	}
	log.Printf("[debug] %s resolved to %s", p.Address, ipaddr)
	pinger.AddIPAddr(ipaddr)

	var min, max, total, avg time.Duration
	var successCount, failureCount int
	pinger.MaxRTT = p.Timeout
	pinger.OnRecv = func(addr *net.IPAddr, rtt time.Duration) {
		log.Println("[debug] OnRecv RTT", rtt)
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
			log.Printf("[warn] ping failed to %s(%s): %s", p.Address, ipaddr, err)
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
	log.Println("[trace]", ms.String())

	return ms, nil
}
