package maprobe_test

import (
	"testing"
	"time"

	"github.com/fujiwara/maprobe"
	"github.com/google/go-cmp/cmp"
)

var metricTests = []struct {
	input    string
	expected maprobe.Metric
}{
	{
		input:    "foo.bar\t42\t1755680137",
		expected: maprobe.Metric{Name: "foo.bar", Value: 42, Timestamp: time.Unix(1755680137, 0)},
	},
	{
		input:    "foo.bar.baz\t42.123\t1755680137",
		expected: maprobe.Metric{Name: "foo.bar.baz", Value: float64(42.123), Timestamp: time.Unix(1755680137, 0)},
	},
	{
		input: "foo.bar.baz\t42.123\t1755680137.888\tattr1=value1\tattr2=value2",
		expected: maprobe.Metric{
			Name:      "foo.bar.baz",
			Value:     float64(42.123),
			Timestamp: time.Unix(1755680137, 0),
			Attribute: &maprobe.Attribute{
				Extra: map[string]string{
					"attr1": "value1",
					"attr2": "value2",
				},
			},
		},
	},
}

func TestParseMetricLine(t *testing.T) {
	for _, test := range metricTests {
		t.Run(test.input, func(t *testing.T) {
			m, err := maprobe.ParseMetricLine(test.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(test.expected, m); diff != "" {
				t.Errorf("unexpected diff (-want +got):\n%s", diff)
			}
			t.Log(m.OtelString())
		})
	}
}
