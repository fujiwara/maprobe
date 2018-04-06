package maprobe

import (
	"context"
	"log"
	"net"
	"time"

	mackerel "github.com/mackerelio/mackerel-client-go"
	"github.com/pkg/errors"
	fping "github.com/tatsushid/go-fastping"
)

var (
	DefaultPingTimeout = time.Second
	DefaultPingCount   = 3
)

type PingProbeConfig struct {
	Address string        `yaml:"address"`
	Count   int           `yaml:"count"`
	Timeout time.Duration `yaml:"timeout"`
}

func (pc *PingProbeConfig) GenerateProbe(host *mackerel.Host) (*PingProbe, error) {
	p := &PingProbe{
		hostID:  host.ID,
		Count:   pc.Count,
		Timeout: pc.Timeout,
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
	return p, nil
}

type PingProbe struct {
	hostID  string        `json:"host_id" yaml:"host_id"`
	Address string        `json:"address" yaml:"address"`
	Count   int           `json:"count" yaml:"count"`
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

func (p *PingProbe) HostID() string {
	return p.hostID
}

func (p *PingProbe) Run(ctx context.Context) (Metrics, error) {
	var ms Metrics

	pinger := fping.NewPinger()
	ipaddr, err := net.ResolveIPAddr("ip", p.Address)
	if err != nil {
		now := time.Now()
		ms = append(ms, newMetric(p, "ping.count.success", 0, now))
		ms = append(ms, newMetric(p, "ping.count.failure", 1, now))
		return ms, errors.Wrap(err, "resolve failed")
	}
	pinger.AddIPAddr(ipaddr)

	var min, max, total, avg time.Duration
	var successCount, failureCount int
	pinger.MaxRTT = p.Timeout
	pinger.OnRecv = func(addr *net.IPAddr, rtt time.Duration) {
		log.Println("[debug] OnRecv")
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
			log.Printf("ping error to %s(%s): %s", p.Address, ipaddr, err)
		}
	}
	if successCount != 0 {
		avg = time.Duration(int64(total) / int64(successCount))
	}

	now := time.Now()
	ms = append(ms, newMetric(p, "ping.count.success", float64(successCount), now))
	ms = append(ms, newMetric(p, "ping.count.failure", float64(failureCount), now))
	if min > 0 || max > 0 || avg > 0 {
		ms = append(ms, newMetric(p, "ping.rtt.min", min.Seconds(), now))
		ms = append(ms, newMetric(p, "ping.rtt.max", max.Seconds(), now))
		ms = append(ms, newMetric(p, "ping.rtt.avg", avg.Seconds(), now))
	}
	return ms, nil
}
