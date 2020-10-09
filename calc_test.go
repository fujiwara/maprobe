package maprobe

import (
	"fmt"
	"testing"
)

type calcTest struct {
	values []float64
	sum    float64
	min    float64
	max    float64
	avg    float64
	median float64
	count  float64
}

var calcTests = []calcTest{
	{
		values: []float64{},
		sum:    0,
		min:    0,
		max:    0,
		avg:    0,
		median: 0,
		count:  0,
	},
	{
		values: []float64{3.3},
		sum:    3.3,
		min:    3.3,
		max:    3.3,
		avg:    3.3,
		median: 3.3,
		count:  1,
	},
	{
		values: []float64{1, 3, 2},
		sum:    6,
		min:    1,
		max:    3,
		avg:    2,
		median: 2,
		count:  3,
	},
	{
		values: []float64{1, 3, 2, 4},
		sum:    10,
		min:    1,
		max:    4,
		avg:    2.5,
		median: 2.5,
		count:  4,
	},
	{
		values: []float64{8, 7.4, 2.2, 3.9, 0, 9.1, 6.2},
		sum:    36.800000000000004,
		min:    0,
		max:    9.1,
		avg:    5.257142857142858,
		median: 6.2,
		count:  7,
	},
}

func f2s(f float64) string {
	return fmt.Sprintf("%.6f", f)
}

func TestCalc(t *testing.T) {
	for _, ts := range calcTests {
		v := ts.values
		if r := sum(v); f2s(r) != f2s(ts.sum) {
			t.Errorf("failed to calc sum(%v)=%f != %f", r, v, ts.sum)
		}
		if r := min(v); f2s(r) != f2s(ts.min) {
			t.Errorf("failed to calc min(%v)=%f != %f", r, v, ts.min)
		}
		if r := max(v); f2s(r) != f2s(ts.max) {
			t.Errorf("failed to calc max(%v)=%f != %f", r, v, ts.max)
		}
		if r := avg(v); f2s(r) != f2s(ts.avg) {
			t.Errorf("failed to calc avg(%v)=%f != %f", r, v, ts.avg)
		}
		if r := median(v); f2s(r) != f2s(ts.median) {
			t.Errorf("failed to calc median(%v)=%f != %f", r, v, ts.median)
		}
		if r := count(v); f2s(r) != f2s(ts.count) {
			t.Errorf("failed to calc count(%v)=%f != %f", r, v, ts.count)
		}
	}
}
