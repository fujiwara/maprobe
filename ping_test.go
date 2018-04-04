package maprobe_test

import (
	"context"
	"testing"
	"time"

	"github.com/fujiwara/maprobe"
)

var pingTimeout = 100 * time.Millisecond
var pingProbes = []maprobe.PingProbe{
	maprobe.PingProbe{Address: "8.8.8.8", Count: 3, Timeout: pingTimeout},
	maprobe.PingProbe{Address: "google-public-dns-b.google.com", Count: 3, Timeout: pingTimeout},
	maprobe.PingProbe{Address: "1.1.1.1", Count: 3, Timeout: pingTimeout},
	maprobe.PingProbe{Address: "1dot1dot1dot1.cloudflare-dns.com", Count: 3, Timeout: pingTimeout},
	maprobe.PingProbe{Address: "noname.example.com", Count: 3, Timeout: pingTimeout},
}

func TestPing(t *testing.T) {
	for _, probe := range pingProbes {
		ms, err := probe.Run(context.Background())
		if err != nil {
			t.Error(err)
		}
		t.Log(probe.Address, ms.String())
	}
}
