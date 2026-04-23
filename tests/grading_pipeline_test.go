package scanner

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

// TestAnalyzeCardImage_WithRealGrading verifies the real grading pipeline
// using ProcessImage which calls the actual GradeImage.
func TestAnalyzeCardImage_WithRealGrading(t *testing.T) {
	w, h := 300, 420
	img := blankImage(w, h, color.RGBA{255, 255, 255, 255})
	addCard(img, 15, 15, 270, 390, color.RGBA{240, 235, 230, 255})

	result := ProcessImage(&img)

	// All sub-scores must be valid 1-10
	for name, score := range map[string]int{"C": result.C, "Co": result.Co, "E": result.E, "S": result.S} {
		if score < 1 || score > 10 {
			t.Errorf("%s=%d, want 1-10", name, score)
		}
	}

	// Overall grade must be in valid range
	if result.Grade < 0 || result.Grade > 10 {
		t.Errorf("Grade=%.1f out of range [0.0, 10.0]", result.Grade)
	}
}

// TestAnalyzeCardImage_LowQualityImage tests that smaller images still grade.
func TestAnalyzeCardImage_LowQualityImage(t *testing.T) {
	w, h := 200, 280
	img := blankImage(w, h, color.RGBA{255, 255, 255, 255})
	addCard(img, 10, 10, 180, 260, color.RGBA{240, 235, 230, 255})

	result := ProcessImage(&img)
	if result.Grade < 0 || result.Grade > 10 {
		t.Errorf("small image Grade=%.1f out of range", result.Grade)
	}
}

// TestPNGRoundtrip verifies that a saved PNG card can be reloaded
// and still produces valid grading results.
func TestPNGRoundtrip(t *testing.T) {
	tmp := t.TempDir() + "/test_card.png"
	w, h := 300, 420

	img := blankImage(w, h, color.RGBA{255, 255, 255, 255})
	addCard(img, 15, 15, 270, 390, color.RGBA{240, 235, 230, 255})

	f, err := os.Create(tmp)
	if err != nil {
		t.Fatalf("Create(%s) = %v", tmp, err)
	}
	err = png.Encode(f, img)
	if err != nil {
		t.Fatalf("png.Encode() = %v", err)
	}
	f.Close()

	f2, err := os.Open(tmp)
	if err != nil {
		t.Fatalf("Open(%s) = %v", tmp, err)
	}
	reloaded, err := png.Decode(f2)
	if err != nil {
		t.Fatalf("png.Decode() = %v", err)
	}
	f2.Close()

	result := ProcessImage(&reloaded)
	if result.Grade < 0 || result.Grade > 10 {
		t.Errorf("reloaded image Overall=%.1f, want >= 0.0", result.Grade)
	}
}
