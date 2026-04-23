package scanner

import (
	"math"

	"github.com/chuckmcgut/2R-Scan/internal/api"
	"github.com/chuckmcgut/2R-Scan/internal/grading"
)

// ProcessImage analyzes the card image and returns the grading result.
// Wires grading.GradeImage (real heuristics) into the API response.
func ProcessImage(img *image.Image) api.ScanResult {
	grade := grading.GradeImage(*img)

	// Map float breakdown (0.0-1.0) to int sub-scores (1-10)
	c := clampInt(int(math.Round(grade.Breakdown.Centering*10)), 1, 10)
	co := clampInt(int(math.Round(grade.Breakdown.Corners*10)), 1, 10)
	e := clampInt(int(math.Round(grade.Breakdown.Edges*10)), 1, 10)
	s := clampInt(int(math.Round(grade.Breakdown.Surface*10)), 1, 10)

	return api.ScanResult{
		Name:  grade.CardName,
		C:     c,
		Co:    co,
		E:     e,
		S:     s,
		Grade: grade.Overall,
	}
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
