package maprobe

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var testConfigExpected = &Config{
	PostProbedMetrics: false,
	Probes: []*ProbeDefinition{
		{
			Service:  exString{"prod"},
			Role:     exString{"EC2"},
			Roles:    []exString{{"EC2"}},
			Statuses: []exString{{"working"}, {"standby"}},
			Ping: &PingProbeConfig{
				Address: "{{ .ipAddresses.eth0 }}",
				Count:   3,
				Timeout: 5 * time.Second,
			},
		},
		{
			Service: exString{"prod"},
			Role:    exString{"prod-NLB"},
			Roles:   []exString{{"prod-NLB"}},
			TCP: &TCPProbeConfig{
				Host:          "{{ .customIdentifier }}",
				Port:          "11211",
				Send:          "VERSION\r\n",
				ExpectPattern: "^VERSION ",
				Timeout:       3 * time.Second,
			},
		},
		{
			Service: exString{"prod"},
			Role:    exString{"ALB"},
			Roles:   []exString{{"ALB"}},
			HTTP: &HTTPProbeConfig{
				URL:    "{{ .metadata.probe.url }}?service={{ env `SERVICE` }}",
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
		{
			Service:         exString{"prod"},
			IsServiceMetric: true,
			HTTP: &HTTPProbeConfig{
				URL:    "{{ .metadata.probe.url }}",
				Method: "GET",
			},
		},
	},
	PostAggregatedMetrics: false,
	Aggregates: []*AggregateDefinition{
		{
			Service: exString{"prod"},
			Role:    exString{"web"},
			Roles:   []exString{{"web"}},
			Metrics: []*MetricConfig{
				{
					Name: exString{"custom.nginx.requests.requests"},
					Outputs: []*OutputConfig{
						{
							Func: exString{"sum"},
							Name: exString{"custom.nginx.requests.sum_requests"},
						},
						{
							Func: exString{"avg"},
							Name: exString{"custom.nginx.requests.avg_requests"},
						},
					},
				},
				{
					Name: exString{"custom.nginx.connections.connections"},
					Outputs: []*OutputConfig{
						{
							Func: exString{"avg"},
							Name: exString{"custom.nginx.connections.avg_connections"},
						},
						{
							Func: exString{"median"},
							Name: exString{"custom.nginx.connections.median_connections"},
						},
					},
				},
			},
		},
	},
}

func TestConfig(t *testing.T) {
	if s, found := os.LookupEnv("SERVICE"); found {
		defer func() {
			os.Setenv("SERVICE", s)
		}()
	}
	os.Setenv("SERVICE", "prod")

	conf, d1, err := LoadConfig(context.Background(), "test/config.yaml")
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
		opt := cmpopts.IgnoreUnexported(OutputConfig{})
		if diff := cmp.Diff(a, b, opt); diff != "" {
			t.Errorf("unexpected aggregates %d\n%s", i, diff)
		}
	}
	_, d2, err := LoadConfig(context.Background(), "test/config.copy.yaml")
	if err != nil {
		t.Error(err)
	}
	if d1 != d2 {
		t.Errorf("digest is not match %s != %s", d1, d2)
	}

	_, d3, err := LoadConfig(context.Background(), "test/config.mod.yaml")
	if err != nil {
		t.Error(err)
	}
	if d1 == d3 {
		t.Errorf("digest must be changed %s != %s", d1, d3)
	}
}
