// Package grading provides image analysis heuristics for PSA card grading.
package grading

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

// GradeBreakdown holds per-category scores (0.0 to 1.0).
type GradeBreakdown struct {
	Centering float64 `json:"centering"`
	Corners   float64 `json:"corners"`
	Edges     float64 `json:"edges"`
	Surface   float64 `json:"surface"`
}

// GradeResult is the full output of grading a card image.
type GradeResult struct {
	Overall    float64        `json:"overall"`
	Breakdown  GradeBreakdown `json:"breakdown"`
	CardName   string         `json:"card_name,omitempty"`
	Confidence float64        `json:"confidence"`
}

// GradeImage analyzes a card image and returns a grade estimate.
func GradeImage(img image.Image) GradeResult {
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()

	centering := analyzeCentering(img, width, height)
	corners := analyzeCorners(img, width, height)
	edges := analyzeEdges(img, width, height)
	surface := analyzeSurface(img, width, height)

	overall := (centering*0.20 + corners*0.20 + edges*0.25 + surface*0.35) * 10
	overall = math.Round(overall*10) / 10

	confidence := calculateConfidence(centering, corners, edges, surface)

	return GradeResult{
		Overall:    overall,
		Breakdown:  GradeBreakdown{centering, corners, edges, surface},
		Confidence: confidence,
	}
}

func analyzeCentering(img image.Image, w, h int) float64 {
	top := findTopEdge(img, 0, w, 0, h/2)
	bottom := findBottomEdge(img, 0, w, h-1, h/2)
	left := findLeftEdge(img, 0, h, 0, w/2)
	right := findRightEdge(img, 0, h, w-1, w/2)

	if top == 0 || bottom == 0 || left == 0 || right == 0 {
		return 0.5
	}

	topDev := math.Abs(float64(top) - float64(bottom))
	leftDev := math.Abs(float64(left) - float64(right))
	maxDev := math.Max(topDev/float64(h), leftDev/float64(w))

	score := 1.0 - (maxDev / 0.10)
	if score < 0 {
		return 0
	}
	return math.Round(score*100) / 100
}

func findTopEdge(img image.Image, x1, x2, y, midY int) int {
	for y := y; y < midY; y++ {
		for x := x1; x < x2; x++ {
			if isSignificantPixel(img.At(x, y)) {
				return y
			}
		}
	}
	return 0
}

func findBottomEdge(img image.Image, x1, x2, y, midY int) int {
	for y := y; y > midY; y-- {
		for x := x1; x < x2; x++ {
			if isSignificantPixel(img.At(x, y)) {
				return y
			}
		}
	}
	return 0
}

func findLeftEdge(img image.Image, y1, y2, x, midX int) int {
	for x := x; x < midX; x++ {
		for y := y1; y < y2; y++ {
			if isSignificantPixel(img.At(x, y)) {
				return x
			}
		}
	}
	return 0
}

func findRightEdge(img image.Image, y1, y2, x, midX int) int {
	for x := x; x > midX; x-- {
		for y := y1; y < y2; y++ {
			if isSignificantPixel(img.At(x, y)) {
				return x
			}
		}
	}
	return 0
}

func isSignificantPixel(p color.Color) bool {
	r, g, b, a := p.RGBA()
	if a < 128<<8 {
		return false
	}
	avg := (r + g + b) / 3
	contrast := math.Abs(float64(r>>8)-float64(avg>>8)) +
		math.Abs(float64(g>>8)-float64(avg>>8)) +
		math.Abs(float64(b>>8)-float64(avg>>8))
	return contrast > 15
}

func analyzeCorners(img image.Image, w, h int) float64 {
	cornerSize := w / 6
	corners := []struct{ x, y int }{
		{0, 0}, {w - cornerSize, 0}, {0, h - cornerSize}, {w - cornerSize, h - cornerSize},
	}
	scores := make([]float64, 4)
	for i, c := range corners {
		scores[i] = scoreCorner(img, c.x, c.y, cornerSize, cornerSize, w, h)
	}
	avg := (scores[0] + scores[1] + scores[2] + scores[3]) / 4
	return math.Round(avg*100) / 100
}

func scoreCorner(img image.Image, x, y, cw, ch, imgW, imgH int) float64 {
	damageCount := 0
	samples := 0

	maxX := min(x+cw, imgW)
	maxY := min(y+ch, imgH)

	for sy := y; sy < maxY; sy++ {
		for sx := x; sx < maxX; sx++ {
			r, g, b, a := img.At(sx, sy).RGBA()
			if a < 128<<8 {
				continue
			}
			avg := (r + g + b) / 3

			if avg>>8 > 200 {
				damageCount++
			}
			contrast := math.Abs(float64(r>>8)-float64(avg>>8)) +
				math.Abs(float64(g>>8)-float64(avg>>8)) +
				math.Abs(float64(b>>8)-float64(avg>>8))
			if contrast > 40 {
				damageCount++
			}
			samples++
		}
	}

	if samples == 0 {
		return 0.5
	}
	damageRatio := float64(damageCount) / float64(samples)
	score := 1.0 - (damageRatio * 3)
	if score < 0 {
		return 0
	}
	return score
}

func analyzeEdges(img image.Image, w, h int) float64 {
	edgeWidth := w / 20
	regions := []struct{ x, y, cw, ch int }{
		{0, 0, w, edgeWidth},
		{0, h - edgeWidth, w, edgeWidth},
		{0, 0, edgeWidth, h},
		{w - edgeWidth, 0, edgeWidth, h},
	}
	scores := make([]float64, 4)
	for i, r := range regions {
		scores[i] = scoreEdgeRegion(img, r.x, r.y, r.cw, r.ch, w, h)
	}
	avg := (scores[0] + scores[1] + scores[2] + scores[3]) / 4
	return math.Round(avg*100) / 100
}

func scoreEdgeRegion(img image.Image, x, y, cw, ch, imgW, imgH int) float64 {
	damageCount := 0
	samples := 0

	maxX := min(x+cw, imgW)
	maxY := min(y+ch, imgH)

	for sy := y; sy < maxY; sy++ {
		for sx := x; sx < maxX; sx++ {
			r, g, b, a := img.At(sx, sy).RGBA()
			if a < 128<<8 {
				continue
			}
			avg := (r + g + b) / 3

			if avg>>8 > 210 {
				damageCount++
			}
			contrast := math.Abs(float64(r>>8)-float64(avg>>8)) +
				math.Abs(float64(g>>8)-float64(avg>>8)) +
				math.Abs(float64(b>>8)-float64(avg>>8))
			if contrast > 35 {
				damageCount++
			}
			samples++
		}
	}

	if samples == 0 {
		return 0.5
	}
	damageRatio := float64(damageCount) / float64(samples)
	score := 1.0 - (damageRatio * 4)
	if score < 0 {
		return 0
	}
	return score
}

func analyzeSurface(img image.Image, w, h int) float64 {
	centerX := w / 2
	centerY := h / 2
	sampleSize := w / 3
	startX := centerX - sampleSize/2
	startY := centerY - sampleSize/2

	damageCount := 0
	samples := 0

	maxX := min(startX+sampleSize, w)
	maxY := min(startY+sampleSize, h)

	for sy := startY; sy < maxY; sy++ {
		for sx := startX; sx < maxX; sx++ {
			if sx <= 0 {
				continue
			}
			r, g, b, a := img.At(sx, sy).RGBA()
			if a < 128<<8 {
				continue
			}
			avg := (r + g + b) / 3

			// Scratch detection
			pr, pg, pb, _ := img.At(sx-1, sy).RGBA()
			pavg := (pr + pg + pb) / 3
			if math.Abs(float64(avg>>8)-float64(pavg>>8)) > 30 {
				damageCount++
			}

			// Water damage
			if avg>>8 < 50 {
				damageCount++
			}

			samples++
		}
	}

	if samples == 0 {
		return 0.5
	}
	damageRatio := float64(damageCount) / float64(samples)
	score := 1.0 - (damageRatio * 5)
	if score < 0 {
		return 0
	}
	return math.Round(score*100) / 100
}

func calculateConfidence(c, co, e, s float64) float64 {
	avg := (c + co + e + s) / 4
	spread := math.Abs(c-avg) + math.Abs(co-avg) + math.Abs(e-avg) + math.Abs(s-avg)
	conf := avg * (1.0 - spread/2)
	if conf < 0.5 {
		return 0.5
	}
	return math.Round(conf*100) / 100
}

// FormatResult returns a human-readable grade string.
func FormatResult(r GradeResult) string {
	return fmt.Sprintf("PSA %.1f (C:%.1f | Co:%.1f | E:%.1f | S:%.1f)",
		r.Overall,
		r.Breakdown.Centering,
		r.Breakdown.Corners,
		r.Breakdown.Edges,
		r.Breakdown.Surface,
	)
}