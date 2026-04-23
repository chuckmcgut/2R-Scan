package grading

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
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

func TestGradeImage_Basic(t *testing.T) {
	w, h := 300, 420
	img := blankImage(w, h, color.RGBA{255, 255, 255, 255})
	addCard(img, 15, 15, 270, 390, color.RGBA{240, 235, 230, 255})

	result := GradeImage(img)

	if result.Overall < 7.0 {
		t.Errorf("pristine card Overall=%.1f, want >= 7.0", result.Overall)
	}
	if result.Confidence < 0.5 {
		t.Errorf("Confidence=%.2f, want >= 0.5", result.Confidence)
	}
}

func TestCenteringDetection(t *testing.T) {
	w, h := 300, 420
	img := blankImage(w, h, color.RGBA{255, 255, 255, 255})
	addCard(img, 5, 5, 220, 340, color.RGBA{240, 235, 230, 255})

	result := GradeImage(img)

	if result.Breakdown.Centering > 0.8 {
		t.Errorf("off-center card centering=%.2f, want <= 0.8", result.Breakdown.Centering)
	}
}

func TestEdgeDamage(t *testing.T) {
	w, h := 300, 420
	img := blankImage(w, h, color.RGBA{255, 255, 255, 255})
	addCard(img, 15, 15, 270, 390, color.RGBA{240, 235, 230, 255})

	for y := 15; y < 405; y++ {
		for x := 275; x < 285; x++ {
			img.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}

	result := GradeImage(img)

	if result.Breakdown.Edges > 0.85 {
		t.Errorf("edge-damaged card edges=%.2f, want <= 0.85", result.Breakdown.Edges)
	}
}

func TestCornerDamage(t *testing.T) {
	w, h := 300, 420
	img := blankImage(w, h, color.RGBA{255, 255, 255, 255})
	addCard(img, 15, 15, 270, 390, color.RGBA{240, 235, 230, 255})

	for y := 360; y < 390; y++ {
		for x := 260; x < 290; x++ {
			img.Set(x, y, color.RGBA{50, 50, 50, 255})
		}
	}

	result := GradeImage(img)

	if result.Breakdown.Corners > 0.9 {
		t.Errorf("corner-damaged card corners=%.2f, want <= 0.9", result.Breakdown.Corners)
	}
}

func TestSurfaceScratch(t *testing.T) {
	w, h := 300, 420
	img := blankImage(w, h, color.RGBA{255, 255, 255, 255})
	addCard(img, 15, 15, 270, 390, color.RGBA{240, 235, 230, 255})

	for y := 100; y < 320; y++ {
		img.Set(50, y, color.RGBA{255, 255, 255, 255})
		img.Set(51, y, color.RGBA{255, 255, 255, 255})
	}

	result := GradeImage(img)

	if result.Breakdown.Surface > 0.9 {
		t.Errorf("scratched card surface=%.2f, want <= 0.9", result.Breakdown.Surface)
	}
}

func TestScoreIs01Range(t *testing.T) {
	w, h := 300, 420
	img := blankImage(w, h, color.RGBA{255, 255, 255, 255})
	addCard(img, 15, 15, 270, 390, color.RGBA{240, 235, 230, 255})

	result := GradeImage(img)

	for label, score := range map[string]float64{
		"Centering": result.Breakdown.Centering,
		"Corners":   result.Breakdown.Corners,
		"Edges":     result.Breakdown.Edges,
		"Surface":   result.Breakdown.Surface,
	} {
		if score < 0 || score > 1 {
			t.Errorf("%s=%.3f outside [0,1] range", label, score)
		}
	}
}

func TestConfidenceConsistent(t *testing.T) {
	w, h := 300, 420
	img := blankImage(w, h, color.RGBA{255, 255, 255, 255})
	addCard(img, 15, 15, 270, 390, color.RGBA{240, 235, 230, 255})

	result := GradeImage(img)

	if result.Confidence < 0.7 {
		t.Errorf("pristine card confidence=%.2f, want >= 0.7", result.Confidence)
	}
}

func TestOverallWeightedAverage(t *testing.T) {
	w, h := 300, 420
	img := blankImage(w, h, color.RGBA{255, 255, 255, 255})
	addCard(img, 15, 15, 270, 390, color.RGBA{240, 235, 230, 255})

	result := GradeImage(img)

	expected := (result.Breakdown.Centering*0.20 +
		result.Breakdown.Corners*0.20 +
		result.Breakdown.Edges*0.25 +
		result.Breakdown.Surface*0.35) * 10

	if math.Abs(result.Overall-expected) > 0.02 {
		t.Errorf("Overall=%.2f, want weighted sum %.2f", result.Overall, expected)
	}
}

func TestFormatResult(t *testing.T) {
	r := GradeResult{Overall: 8.5, Breakdown: GradeBreakdown{0.9, 0.85, 0.8, 0.82}}
	s := FormatResult(r)
	if s == "" {
		t.Error("FormatResult returned empty string")
	}
}

func TestPNGRoundtrip(t *testing.T) {
	tmp := t.TempDir() + "/test_card.png"
	w, h := 300, 420

	img := image.NewRGBA(image.Rect(0, 0, w, h))
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

	result := GradeImage(reloaded)
	if result.Overall < 5.0 {
		t.Errorf("reloaded image Overall=%.1f, want >= 5.0", result.Overall)
	}
}

func BenchmarkGradeImage(b *testing.B) {
	w, h := 300, 420
	img := blankImage(w, h, color.RGBA{255, 255, 255, 255})
	addCard(img, 15, 15, 270, 390, color.RGBA{240, 235, 230, 255})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GradeImage(img)
	}
}