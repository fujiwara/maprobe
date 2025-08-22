package maprobe_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fujiwara/maprobe"
	"github.com/google/go-cmp/cmp"
	mackerel "github.com/mackerelio/mackerel-client-go"
)

var commandProbesExpect = [][]maprobe.Metric{
	{
		{
			Name:      "custom.test.test.ok",
			Value:     1,
			Timestamp: time.Unix(1523261168, 0),
		},
	},
	{
		{
			Name:      "test.test.ok",
			Value:     1,
			Timestamp: time.Unix(1523261168, 0),
		},
	},
	{
		{
			Name:      "test.envfoo.ok",
			Value:     1,
			Timestamp: time.Unix(1523261168, 0),
		},
	},
	{
		{
			Name:      "test.foofoo.ok",
			Value:     1,
			Timestamp: time.Unix(1523261168, 0),
		},
	},
	{
		{
			Name:      "test.barbar.ok",
			Value:     1,
			Timestamp: time.Unix(1523261168, 0),
		},
	},
	{
		{
			Name:      "test.attr.ok",
			Value:     1,
			Timestamp: time.Unix(1523261168, 0),
			Attribute: &maprobe.Attribute{
				Extra: map[string]string{
					"attr_foo": "foo",
				},
			},
		},
	},
}

func TestCommand(t *testing.T) {
	c, _, err := maprobe.LoadConfig(context.Background(), "test/command.yaml")
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
			expected := commandProbesExpect[i][j]
			if d := cmp.Diff(m.String(), expected.String()); d != "" {
				t.Errorf("unexpected response %s", d)
			}
			if d := cmp.Diff(m.OtelString(), expected.OtelString()); d != "" {
				t.Errorf("unexpected response %s", d)
			}
		}
		t.Log(ms.String())
	}
}

func TestCommandFail(t *testing.T) {
	c, _, err := maprobe.LoadConfig(context.Background(), "test/command_fail.yaml")
	if err == nil {
		t.Errorf("must be failed but got %#v", c)
	}
}

var ngInputs = []struct {
	Title string
	Line  string
}{
	{
		Title: "invalid value",
		Line:  strings.Join([]string{"test.foo", "x", "1523261168"}, "\t"),
	},
	{
		Title: "invalid timestamp",
		Line:  strings.Join([]string{"test.foo", "1", "x"}, "\t"),
	},
	{
		Title: "Empty",
		Line:  "",
	},
	{
		Title: "No name",
		Line:  strings.Join([]string{"", "1", "1523261168"}, "\t"),
	},
}

func TestParseMetricLineNG(t *testing.T) {
	for _, c := range ngInputs {
		_, err := maprobe.ParseMetricLine(c.Line)
		if err == nil {
			t.Errorf("%s: must be failed", c.Title)
		}
	}
}
