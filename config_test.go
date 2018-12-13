package maprobe_test

import (
	"testing"
	"time"

	"github.com/fujiwara/maprobe"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var testConfigExpected = &maprobe.Config{
	APIKey:            "DUMMY",
	PostProbedMetrics: false,
	Probes: []*maprobe.ProbeDefinition{
		&maprobe.ProbeDefinition{
			Service:  "prod",
			Role:     "EC2",
			Roles:    []string{"EC2"},
			Statuses: []string{"working", "standby"},
			Ping: &maprobe.PingProbeConfig{
				Address: "{{ .ipAddresses.eth0 }}",
				Count:   3,
				Timeout: 5 * time.Second,
			},
		},
		&maprobe.ProbeDefinition{
			Service: "prod",
			Role:    "NLB",
			Roles:   []string{"NLB"},
			TCP: &maprobe.TCPProbeConfig{
				Host:          "{{ .customIdentifier }}",
				Port:          "11211",
				Send:          "VERSION\r\n",
				ExpectPattern: "^VERSION ",
				Timeout:       3 * time.Second,
			},
		},
		&maprobe.ProbeDefinition{
			Service: "prod",
			Role:    "ALB",
			Roles:   []string{"ALB"},
			HTTP: &maprobe.HTTPProbeConfig{
				URL:    "{{ .metadata.probe.url }}",
				Method: "POST",
				Headers: map[string]string{
					"User-Agent":    "maprobe/0.0.1",
					"Cache-Control": "no-cache",
					"Content-Type":  "application/json",
				},
				Body:               `{"hello":"world"}`,
				ExpectPattern:      "ok",
				NoCheckCertificate: true,
			},
		},
	},
	PostAggregatedMetrics: false,
	Aggregates: []*maprobe.AggregateDefinition{
		&maprobe.AggregateDefinition{
			Service: "prod",
			Role:    "web",
			Roles:   []string{"web"},
			Metrics: []*maprobe.MetricConfig{
				&maprobe.MetricConfig{
					Name: "custom.nginx.requests.requests",
					Outputs: []*maprobe.OutputConfig{
						&maprobe.OutputConfig{
							Func: "sum",
							Name: "custom.nginx.requests.sum_requests",
						},
						&maprobe.OutputConfig{
							Func: "avg",
							Name: "custom.nginx.requests.avg_requests",
						},
					},
				},
				&maprobe.MetricConfig{
					Name: "custom.nginx.connections.connections",
					Outputs: []*maprobe.OutputConfig{
						&maprobe.OutputConfig{
							Func: "avg",
							Name: "custom.nginx.connections.avg_connections",
						},
					},
				},
			},
		},
	},
}

func TestConfig(t *testing.T) {
	conf, d1, err := maprobe.LoadConfig("test/config.yaml")
	if err != nil {
		t.Error(err)
	}
	for i, p := range conf.Probes {
		if diff := cmp.Diff(p, testConfigExpected.Probes[i]); diff != "" {
			t.Errorf("unexpected probes %d\n%s", i, diff)
		}
	}

	for i, a := range conf.Aggregates {
		b := testConfigExpected.Aggregates[i]
		opt := cmpopts.IgnoreUnexported(maprobe.OutputConfig{})
		if diff := cmp.Diff(a, b, opt); diff != "" {
			t.Errorf("unexpected aggregates %d\n%s", i, diff)
		}
	}
	_, d2, err := maprobe.LoadConfig("test/config.copy.yaml")
	if err != nil {
		t.Error(err)
	}
	if d1 != d2 {
		t.Errorf("digest is not match %s != %s", d1, d2)
	}

	_, d3, err := maprobe.LoadConfig("test/config.mod.yaml")
	if err != nil {
		t.Error(err)
	}
	if d1 == d3 {
		t.Errorf("digest must be changed %s != %s", d1, d3)
	}
}
