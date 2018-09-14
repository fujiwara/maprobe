package maprobe_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/fujiwara/maprobe"
	yaml "gopkg.in/yaml.v2"
)

var testConfigExpected = &maprobe.Config{
	APIKey: "DUMMY",
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
}

func TestConfig(t *testing.T) {
	conf, err := maprobe.LoadConfig("test/config.yaml")
	if err != nil {
		t.Error(err)
	}
	for i, p := range conf.Probes {
		if !reflect.DeepEqual(p, testConfigExpected.Probes[i]) {
			t.Errorf("unexpected probes %d", i)
			got, _ := yaml.Marshal(p)
			expect, _ := yaml.Marshal(testConfigExpected.Probes[i])
			t.Log(string(got))
			t.Log(string(expect))
		}
	}
	t.Logf("%#v", conf)
}
