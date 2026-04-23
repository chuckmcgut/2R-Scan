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

	topBorder := top
	bottomBorder := h - bottom
	leftBorder := left
	rightBorder := w - right
	topDev := math.Abs(float64(topBorder) - float64(bottomBorder)) / float64(h)
	leftDev := math.Abs(float64(leftBorder) - float64(rightBorder)) / float64(w)
	maxDev := math.Max(topDev, leftDev)

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
	return contrast > 8
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
	// Score corner by how closely pixels match expected card cream color (240, 235, 230)
	cardR, cardG, cardB := 240, 235, 230
	cardPixels := 0
	totalSamples := 0

	maxX := min(x+cw, imgW)
	maxY := min(y+ch, imgH)

	for sy := y; sy < maxY; sy++ {
		for sx := x; sx < maxX; sx++ {
			r, g, b, a := img.At(sx, sy).RGBA()
			if a < 128<<8 {
				continue
			}
			r8, g8, b8 := r>>8, g>>8, b>>8

			// Pixel matches card color if within tolerance of cream
			// White background pixels (r8>245 && g8>245 && b8>245) are NOT card
			// Card pixels are those close to (240, 235, 230)
			if r8 < 245 || g8 < 245 || b8 < 245 {
				// Not pure white - treat as card
				dev := math.Abs(float64(r8)-float64(cardR)) +
					math.Abs(float64(g8)-float64(cardG)) +
					math.Abs(float64(b8)-float64(cardB))
				if dev < 25 {
					cardPixels++
				}
			}
			totalSamples++
		}
	}

	if totalSamples == 0 {
		return 0.5
	}
	score := float64(cardPixels) / float64(totalSamples)
	return math.Round(score*100) / 100
}

func analyzeEdges(img image.Image, w, h int) float64 {
	// Sample edge quality by checking pixels near card borders
	// Card occupies roughly center of image with margins
	margin := 15 // expected card margin from image edge
	edgeBand := w / 15 // thin band near card edge

	// Score based on whether pixels near card border are uniform (good)
	// vs scattered (damaged/white intruding)
	cardR, cardG, cardB := 240, 235, 230

	topScore := scoreEdgeBand(img, margin, margin, w - margin, edgeBand, cardR, cardG, cardB)
	bottomScore := scoreEdgeBand(img, margin, h - edgeBand - margin, w - margin, edgeBand, cardR, cardG, cardB)
	leftScore := scoreEdgeBand(img, margin, margin, edgeBand, h - 2*margin, cardR, cardG, cardB)
	rightScore := scoreEdgeBand(img, w - edgeBand - margin, margin, edgeBand, h - 2*margin, cardR, cardG, cardB)

	avg := (topScore + bottomScore + leftScore + rightScore) / 4
	return math.Round(avg*100) / 100
}

func scoreEdgeBand(img image.Image, x, y, cw, ch, cardR, cardG, cardB int) float64 {
	// Score a band of pixels - count how many match card color vs white/damaged
	cardPixels := 0
	totalSamples := 0

	maxX := min(x+cw, img.Bounds().Dx())
	maxY := min(y+ch, img.Bounds().Dy())

	for sy := y; sy < maxY; sy++ {
		for sx := x; sx < maxX; sx++ {
			r, g, b, a := img.At(sx, sy).RGBA()
			if a < 128<<8 {
				continue
			}
			r8, g8, b8 := r>>8, g>>8, b>>8

			// Count as card pixel if close to expected card cream
			dev := math.Abs(float64(r8)-float64(cardR)) +
				math.Abs(float64(g8)-float64(cardG)) +
				math.Abs(float64(b8)-float64(cardB))
			if dev < 30 {
				cardPixels++
			}
			totalSamples++
		}
	}

	if totalSamples == 0 {
		return 0.5
	}
	return float64(cardPixels) / float64(totalSamples)
}

func analyzeSurface(img image.Image, w, h int) float64 {
	// Sample the center region of the card to detect surface damage
	// Card occupies roughly center of 300x420 image with 15px margins
	// Sample region: x=75 to 225, y=85 to 335 (avoiding card edges)

	damageCount := 0
	samples := 0

	// First pass: compute card baseline (average non-white pixel in sample area)
	totalVal := 0
	totalSamples := 0
	for sy := 85; sy < 335; sy++ {
		for sx := 75; sx < 225; sx++ {
			r, g, b, a := img.At(sx, sy).RGBA()
			if a < 128<<8 {
				continue
			}
			r8 := r >> 8
			g8 := g >> 8
			b8 := b >> 8
			// Only count non-white pixels as card area
			if r8 < 250 || g8 < 250 || b8 < 250 {
				totalVal += int(r8) + int(g8) + int(b8)
				totalSamples++
			}
		}
	}
	if totalSamples == 0 {
		return 0.5
	}
	cardAvg := totalVal / (3 * totalSamples)

	// Second pass: detect scratches by finding pixels that deviate from cardAvg
	for sy := 15; sy < 405; sy++ {
		for sx := 15; sx < 285; sx++ {
			r, g, b, a := img.At(sx, sy).RGBA()
			if a < 128<<8 {
				continue
			}
			pixelAvg := (int(r>>8) + int(g>>8) + int(b>>8)) / 3 // divide by 8 (3 adds)


			// Scratch: pixel is significantly lighter than card average
			if pixelAvg > cardAvg+15 {
				damageCount++
			}
			// Water damage: pixel is significantly darker
			if pixelAvg < cardAvg-40 {
				damageCount++
			}
			samples++
		}
	}


	if samples == 0 {
		return 0.5
	}
	damageRatio := float64(damageCount) / float64(samples)
	score := 1.0 - (damageRatio * 25)
	if score < 0 {
		return 0
	}
	return math.Round(score*100) / 100
}

func calculateConfidence(c, co, e, s float64) float64 {
	avg := (c + co + e + s) / 4
	spread := math.Abs(c-avg) + math.Abs(co-avg) + math.Abs(e-avg) + math.Abs(s-avg)
	conf := avg * (1.0 - spread/4)
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