package maprobe_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/fujiwara/maprobe"
	mackerel "github.com/mackerelio/mackerel-client-go"
)

var commandProbesConfig = []*maprobe.CommandProbeConfig{
	&maprobe.CommandProbeConfig{
		Command: `sh -c 'echo "test.{{ .ID }}.ok\t1\t1523261168"'`,
	},
}

var commandProbesExpect = []maprobe.Metrics{
	maprobe.Metrics{
		maprobe.Metric{
			HostID:    "test",
			Name:      "test.test.ok",
			Value:     1,
			Timestamp: time.Unix(1523261168, 0),
		},
	},
}

func TestCommand(t *testing.T) {
	for i, pc := range commandProbesConfig {
		probe, _ := pc.GenerateProbe(&mackerel.Host{ID: "test"})
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
