package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/chuckmcgut/2R-Scan/cmd/server"
)

// TestServer is a test server that wraps the main server
var TestServer *httptest.Server

func init() {
	// Create a test server that wraps the main server
	// Note: In real testing, we'd want to mock the scanner
}

func TestAPIIntegration(t *testing.T) {
	if TestServer == nil {
		// Create a test server using the actual handlers
		mux := http.NewServeMux()
		mux.HandleFunc("/health", serverHandleHealth)
		mux.HandleFunc("/api/v1/cards/search", serverHandleSearch)
		mux.HandleFunc("/api/v1/scan", serverHandleScan)

		TestServer = httptest.NewServer(mux)
		defer TestServer.Close()
	}

	t.Run("Health Check", testHealth)
	t.Run("Search API", testSearchAPI)
	t.Run("Scan API", testScanAPI)
	t.Run("Rate Limiting", testRateLimiting)
}

func testHealth(t *testing.T) {
	resp, err := http.Get(TestServer.URL + "/health")
	if err != nil {
		t.Fatalf("Failed to make health check request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
		return
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", result["status"])
	}
}

func testSearchAPI(t *testing.T) {
	resp, err := http.Get(TestServer.URL + "/api/v1/cards/search?name=Mickey")
	if err != nil {
		t.Fatalf("Failed to make search request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
		return
	}

	var result []server.Card
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(result) == 0 {
		t.Error("Expected at least one card in results")
		return
	}

	found := false
	for _, card := range result {
		if card.Name == "Mickey Mouse" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected to find Mickey Mouse, got: %v", result)
	}
}

func testScanAPI(t *testing.T) {
	// Create a test image
	img := &image.RGBA{
		Pix: make([]byte, 64*64*4),
	}
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			p := img.Pix[y*64*4+x*4]
			pg := img.Pix[y*64*4+x*4+1]
			bp := img.Pix[y*64*4+x*4+2]
			img.Pix[y*64*4+x*4+3] = 255
			img.Pix[y*64*4+x*4] = uint8(x)
			img.Pix[y*64*4+x*4+1] = uint8(y)
			img.Pix[y*64*4+x*4+2] = uint8(100)
		}
	}

	// Encode as JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("Failed to encode test image: %v", err)
	}

	// Make POST request
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest(http.MethodPost, TestServer.URL+"/api/v1/scan", &buf)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make scan request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
		return
	}

	var result server.ScanResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.Grade == 0 {
		t.Errorf("Expected non-zero grade, got %f", result.Grade)
	}

	t.Logf("Scan successful: Grade=%f, Name=%s", result.Grade, result.Name)
}

func testRateLimiting(t *testing.T) {
	// Reset rate limiter by calling rateLimit with same IP
	// This is a simplified test since we can't easily reset the global state
	// In production, we'd want proper cleanup between tests

	// Make 10 successful requests
	for i := 0; i < 10; i++ {
		resp, err := http.Get(TestServer.URL)
		if err != nil {
			t.Fatalf("Failed to make request %d: %v", i+1, err)
		}
		resp.Body.Close()
	}

	// Make 11th request - should be rate limited
	resp, err := http.Get(TestServer.URL)
	if err != nil {
		t.Fatalf("Failed to make rate limit test request: %v", err)
	}
	defer resp.Body.Close()

	// Check for rate limit response
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", resp.StatusCode)
	}
}

// TestFullScanPipeline tests the complete flow from image upload to grade calculation
func TestFullScanPipeline(t *testing.T) {
	t.Run("Upload and Process Image", func(t *testing.T) {
		// Create a realistic test image
		createTestCardImage()

		// Encode as JPEG
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, testImage, nil); err != nil {
			t.Fatalf("Failed to encode test card image: %v", err)
		}

		// Make request
		client := &http.Client{Timeout: 5 * time.Second}
		req, err := http.NewRequest(http.MethodPost, TestServer.URL+"/api/v1/scan", &buf)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d", resp.StatusCode)
		}

		var result server.ScanResult
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode result: %v", err)
		}

		// Validate result structure
		if result.Grade < 0 || result.Grade > 10 {
			t.Errorf("Grade out of range: %f", result.Grade)
		}

		if result.C+result.Co+result.E+result.S != 40 {
			t.Errorf("Breakdown should sum to 40, got %d", result.C+result.Co+result.E+result.S)
		}

		t.Logf("Pipeline test passed: Grade=%f", result.Grade)
	})
}

func TestConcurrentRequests(t *testing.T) {
	numGoroutines := 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	done := make(chan bool)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			resp, err := http.Get(TestServer.URL)
			if err != nil {
				t.Errorf("Goroutine %d failed: %v", id, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Goroutine %d got status %d", id, resp.StatusCode)
				return
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(done)
	}()

	// Verify all completed
	count := 0
	for range done {
		count++
	}

	if count != numGoroutines {
		t.Errorf("Expected %d completed requests, got %d", numGoroutines, count)
	}
}

func TestInvalidImageHandling(t *testing.T) {
	invalidImages := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"invalid", []byte("not an image")},
		{"too large", make([]byte, 100*1024*1024)}, // 100MB
	}

	for _, test := range invalidImages {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Timeout: 5 * time.Second}
			req, err := http.NewRequest(http.MethodPost, TestServer.URL+"/api/v1/scan", bytes.NewReader(test.data))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			defer resp.Body.Close()

			// Should return error status, not 200
			if resp.StatusCode == http.StatusOK {
				t.Errorf("Expected error status for invalid image, got 200 OK")
			}
		})
	}
}

// createTestCardImage creates a simple test card image
var testImage *image.RGBA

func createTestCardImage() {
	width := 800
	height := 1200
	estImage = &image.RGBA{
		Pix: make([]byte, width*height*4),
	}

	// Draw white background
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			estImage.Pix[y*width*4+x*4] = 255
			estImage.Pix[y*width*4+x*4+1] = 255
			estImage.Pix[y*width*4+x*4+2] = 255
			estImage.Pix[y*width*4+x*4+3] = 255
		}
	}

	// Draw card border
	borderColor := []byte{0, 0, 0, 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if x < 50 || x > width-50 || y < 50 || y > height-50 {
				estImage.Pix[y*width*4+x*4] = borderColor[0]
				estImage.Pix[y*width*4+x*4+1] = borderColor[1]
				estImage.Pix[y*width*4+x*4+2] = borderColor[2]
				estImage.Pix[y*width*4+x*4+3] = borderColor[3]
			}
		}
	}

	// Draw some text pattern
	for y := 100; y < height-100; y += 10 {
		for x := 100; x < width-100; x++ {
			if (x+y)%100 == 0 {
				estImage.Pix[y*width*4+x*4] = 0
				estImage.Pix[y*width*4+x*4+1] = 0
				estImage.Pix[y*width*4+x*4+2] = 0
				estImage.Pix[y*width*4+x*4+3] = 255
			}
		}
	}
}

func TestImportCommand(t *testing.T) {
	// Create a temporary JSON file
	tmpfile, err := os.CreateTemp("", "test_cards_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	testData := []byte(`[
		{"name": "Test Card Alpha", "c": 10, "co": 9, "e": 10, "s": 10, "grade": 8.5, "image_url": "http://example.com/alpha.jpg"},
		{"name": "Test Card Beta", "c": 12, "co": 10, "e": 11, "s": 10, "grade": 9.0, "image_url": "http://example.com/beta.jpg"}
	]`)

	if _, err := tmpfile.Write(testData); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	tmpfile.Close()

	// Test import command
	args := []string{"import", tmpfile.Name()}
	if err := server.Main(args); err != nil {
		t.Errorf("Import command failed: %v", err)
	}
}

// Helper functions
func serverHandleHealth(w http.ResponseWriter, r *http.Request) {
	server.HandleHealth(w, r)
}

func serverHandleSearch(w http.ResponseWriter, r *http.Request) {
	server.HandleSearch(w, r)
}

func serverHandleScan(w http.ResponseWriter, r *http.Request) {
	server.HandleScan(w, r)
}

func init() {
	createTestCardImage()
}
