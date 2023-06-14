package maprobe_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/fujiwara/maprobe"
	mackerel "github.com/mackerelio/mackerel-client-go"
)

var commandProbesExpect = []maprobe.HostMetrics{
	{
		maprobe.HostMetric{
			HostID:    "test",
			Name:      "custom.test.test.ok",
			Value:     1,
			Timestamp: time.Unix(1523261168, 0),
		},
	},
	{
		maprobe.HostMetric{
			HostID:    "test",
			Name:      "test.test.ok",
			Value:     1,
			Timestamp: time.Unix(1523261168, 0),
		},
	},
	{
		maprobe.HostMetric{
			HostID:    "test",
			Name:      "test.envfoo.ok",
			Value:     1,
			Timestamp: time.Unix(1523261168, 0),
		},
	},
	{
		maprobe.HostMetric{
			HostID:    "test",
			Name:      "test.foofoo.ok",
			Value:     1,
			Timestamp: time.Unix(1523261168, 0),
		},
	},
	{
		maprobe.HostMetric{
			HostID:    "test",
			Name:      "test.barbar.ok",
			Value:     1,
			Timestamp: time.Unix(1523261168, 0),
		},
	},
}

func TestCommand(t *testing.T) {
	c, _, err := maprobe.LoadConfig("test/command.yaml")
	if err != nil {
		t.Error(err)
		return
	}
	for i, p := range c.Probes {
		probe, err := p.Command.GenerateProbe(&mackerel.Host{ID: "test"}, nil)
		if err != nil {
			t.Error(err)
		}
		ms, err := probe.Run(context.Background())
		if err != nil {
			t.Error(err)
		}
		for j, m := range ms {
			if !reflect.DeepEqual(m, commandProbesExpect[i][j]) {
				t.Errorf("unexpected response %v expected %v", m, commandProbesExpect[i][j])
			}
		}
		t.Log(ms.String())
	}
}

func TestCommandFail(t *testing.T) {
	c, _, err := maprobe.LoadConfig("test/command_fail.yaml")
	if err == nil {
		t.Errorf("must be failed but got %#v", c)
	}
}
