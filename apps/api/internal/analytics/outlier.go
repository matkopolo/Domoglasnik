package analytics

import "math"

// IsStatisticalOutlier mirrors the versioned SQL rule. IQR and MAD are both
// evaluated; either robust signal may flag an observation.
func IsStatisticalOutlier(value, q1, q3, median, mad float64) bool {
	effectiveIQR := math.Max(q3-q1, math.Abs(median)*0.20)
	iqrOutlier := median > 0 &&
		(value < q1-1.5*effectiveIQR || value > q3+1.5*effectiveIQR)
	effectiveMAD := math.Max(mad, math.Abs(median)*0.10)
	madOutlier := median > 0 &&
		0.6745*math.Abs(value-median)/effectiveMAD > 3.5
	return iqrOutlier || madOutlier
}
