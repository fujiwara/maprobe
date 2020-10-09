package maprobe

import (
	"math"
	"sort"
)

func sum(values []float64) (value float64) {
	if len(values) == 0 {
		return 0
	}
	for _, v := range values {
		value = value + v
	}
	return
}

func min(values []float64) (value float64) {
	if len(values) == 0 {
		return 0
	}
	value = math.MaxFloat64
	for _, v := range values {
		value = math.Min(v, value)
	}
	return
}

func max(values []float64) (value float64) {
	if len(values) == 0 {
		return 0
	}
	for _, v := range values {
		value = math.Max(v, value)
	}
	return
}

func count(values []float64) (value float64) {
	return float64(len(values))
}

func avg(values []float64) (value float64) {
	if len(values) == 0 {
		return 0
	}
	return sum(values) / count(values)
}

func median(values []float64) (value float64) {
	if len(values) == 0 {
		return 0
	}
	size := len(values)
	sort.Float64s(values)
	if size%2 == 0 {
		return (values[size/2-1] + values[size/2]) / 2
	}
	return values[(size-1)/2]
}
