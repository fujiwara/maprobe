package maprobe

import (
	"math"
	"sort"
)

func sum(values []float64) (value float64) {
	for _, v := range values {
		value = value + v
	}
	return
}

func min(values []float64) (value float64) {
	for _, v := range values {
		value = math.Min(v, value)
	}
	return
}

func max(values []float64) (value float64) {
	for _, v := range values {
		value = math.Max(v, value)
	}
	return
}

func count(values []float64) (value float64) {
	return float64(len(values))
}

func avg(values []float64) (value float64) {
	return sum(values) / count(values)
}

func median(values []float64) (value float64) {
	len := len(values)

	sort.Float64s(values)

	if len%2 == 0 {
		return (values[len/2-1] + values[len/2]) / 2.0
	}
	return values[(len-1)/2]

}
