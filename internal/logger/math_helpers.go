package logger

import (
	"math"
	"sort"
)

// PercentileInt64 computes the given percentile from a slice of int64 values.
func PercentileInt64(values []int64, percentile int) float64 {
	if len(values) == 0 {
		return 0
	}
	if percentile <= 0 {
		return float64(values[0])
	}
	if percentile >= 100 {
		return float64(values[len(values)-1])
	}

	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	rank := float64(percentile) / 100.0 * float64(len(sorted)-1)
	low := int(math.Floor(rank))
	high := int(math.Ceil(rank))
	if low == high {
		return float64(sorted[low])
	}

	weight := rank - float64(low)
	return float64(sorted[low])*(1-weight) + float64(sorted[high])*weight
}

// PercentileFloat64 computes the given percentile from a slice of float64 values.
func PercentileFloat64(values []float64, percentile int) float64 {
	if len(values) == 0 {
		return 0
	}
	if percentile <= 0 {
		return values[0]
	}
	if percentile >= 100 {
		return values[len(values)-1]
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	rank := float64(percentile) / 100.0 * float64(len(sorted)-1)
	low := int(math.Floor(rank))
	high := int(math.Ceil(rank))
	if low == high {
		return sorted[low]
	}

	weight := rank - float64(low)
	return sorted[low]*(1-weight) + sorted[high]*weight
}

// MeanAndStdDev computes the mean and population standard deviation.
func MeanAndStdDev(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	if len(values) == 1 {
		return mean, 0
	}

	var variance float64
	for _, v := range values {
		d := v - mean
		variance += d * d
	}
	variance = variance / float64(len(values))
	return mean, math.Sqrt(variance)
}

// MeanFloat64 computes the arithmetic mean.
func MeanFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}
