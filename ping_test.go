package maprobe_test

import (
	"context"
	"testing"
	"time"

	"github.com/fujiwara/maprobe"
	mackerel "github.com/mackerelio/mackerel-client-go"
)

var pingTimeout = 100 * time.Millisecond
var pingProbesConfig = []*maprobe.PingProbeConfig{
	&maprobe.PingProbeConfig{Address: "8.8.8.8", Count: 3, Timeout: pingTimeout},
	&maprobe.PingProbeConfig{Address: "google-public-dns-b.google.com", Count: 3, Timeout: pingTimeout},
	&maprobe.PingProbeConfig{Address: "1.1.1.1", Count: 3, Timeout: pingTimeout},
	&maprobe.PingProbeConfig{Address: "1dot1dot1dot1.cloudflare-dns.com", Count: 3, Timeout: pingTimeout},
	&maprobe.PingProbeConfig{Address: "noname.example.com", Count: 3, Timeout: pingTimeout},
}

func TestPing(t *testing.T) {
	for _, pc := range pingProbesConfig {
		probe, _ := pc.GenerateProbe(&mackerel.Host{ID: "test"})
		ms, err := probe.Run(context.Background())
		if err != nil {
			t.Error(err)
		}
		t.Log(probe.Address, ms.String())
	}
}
