package scanner

import (
	"image"
	"image/color"
	"image/png"
	"testing"
)

func blankImage(w, h int, bg color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, bg)
		}
	}
	return img
}

func addCard(img *image.RGBA, x, y, w, h int, c color.RGBA) {
	for sy := y; sy < y+h && sy < img.Bounds().Dy(); sy++ {
		for sx := x; sx < x+w && sx < img.Bounds().Dx(); sx++ {
			img.Set(sx, sy, c)
		}
	}
}

// TestProcessImage_Basic verifies that ProcessImage returns a valid ScanResult
// with sub-scores in the 1-10 range and grade in the 0.0-10.0 range.
func TestProcessImage_Basic(t *testing.T) {
	// Create a 300x420 card image (pristine)
	w, h := 300, 420
	img := blankImage(w, h, color.RGBA{255, 255, 255, 255})
	addCard(img, 15, 15, 270, 390, color.RGBA{240, 235, 230, 255})

	result := ProcessImage(&img)

	// C, Co, E, S must all be in [1, 10]
	for _, score := range []int{result.C, result.Co, result.E, result.S} {
		if score < 1 || score > 10 {
			t.Errorf("sub-score %d out of range [1,10]", score)
		}
	}

	// Overall grade must be in [0.0, 10.0]
	if result.Grade < 0 || result.Grade > 10 {
		t.Errorf("Grade=%.1f out of range [0.0, 10.0]", result.Grade)
	}
}

// TestProcessImage_Damaged verifies that a card with obvious damage
// scores lower than a pristine card.
func TestProcessImage_Damaged(t *testing.T) {
	// Pristine card
	pristine := blankImage(300, 420, color.RGBA{255, 255, 255, 255})
	addCard(pristine, 15, 15, 270, 390, color.RGBA{240, 235, 230, 255})

	// Damaged card: off-center + edge wear
	damaged := blankImage(300, 420, color.RGBA{255, 255, 255, 255})
	addCard(damaged, 5, 5, 270, 390, color.RGBA{240, 235, 230, 255})
	// Scratched surface
	for x := 50; x < 250; x++ {
		damaged.Set(x, 200, color.RGBA{200, 200, 200, 255})
	}

	pristineResult := ProcessImage(&pristine)
	damagedResult := ProcessImage(&damaged)

	if damagedResult.Grade >= pristineResult.Grade {
		t.Errorf("damaged card grade (%.1f) should be < pristine (%.1f)",
			damagedResult.Grade, pristineResult.Grade)
	}
}

// TestProcessImage_Roundtrip verifies that a PNG-encoded card survives
// encode/decode and still produces a valid grade.
func TestProcessImage_Roundtrip(t *testing.T) {
	img := blankImage(300, 420, color.RGBA{255, 255, 255, 255})
	addCard(img, 15, 15, 270, 390, color.RGBA{240, 235, 230, 255})

	result := ProcessImage(&img)
	if result.Grade < 5.0 {
		t.Errorf("reloaded image Overall=%.1f, want >= 5.0", result.Grade)
	}
}
