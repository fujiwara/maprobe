package maprobe

import "math"

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
