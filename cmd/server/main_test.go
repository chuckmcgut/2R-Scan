package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()
	os.Exit(code)
}

func TestHealthEndpoint(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleHealth(w, r)
	}))
	defer server.Close()

	// Make request
	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check response body
	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", result["status"])
	}
}

func TestSearchEndpoint(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleSearch(w, r)
	}))
	defer server.Close()

	// Make request
	resp, err := http.Get(server.URL + "/api/v1/cards/search?name=Mickey")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check response body
	var result []Card
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify we got Mickey Mouse
	found := false
	for _, card := range result {
		if card.Name == "Mickey Mouse" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected to find Mickey Mouse in results, got: %v", result)
	}
}

func TestSearchEndpointNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleSearch(w, r)
	}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/cards/search?name=NonExistent")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result []Card
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty results, got %d cards", len(result))
	}
}

func TestScanEndpoint(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleScan(w, r)
	}))
	defer server.Close()

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
	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/scan", &buf)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check response body
	var result ScanResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.Grade == 0 {
		t.Errorf("Expected non-zero grade, got %f", result.Grade)
	}

	t.Logf("Scan result: %+v", result)
}

func TestRateLimit(t *testing.T) {
	// Create a server that does nothing
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Empty handler
	}))
	defer server.Close()

	// Make 11 requests quickly (should hit rate limit after 10)
	for i := 0; i < 12; i++ {
		resp, err := http.Get(server.URL)
		if err != nil {
			t.Fatalf("Failed to make request %d: %v", i+1, err)
		}
		resp.Body.Close()
	}

	// Make 11th request (should be rate limited)
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request 12: %v", err)
	}
	defer resp.Body.Close()

	// Check for rate limit response
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", resp.StatusCode)
	}
}

func TestDecodeImage(t *testing.T) {
	// Test with valid JPEG data
	testJPEG := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43, 0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09, 0x09, 0x08, 0x0A, 0x0C, 0x14, 0x0D, 0x0C, 0x0B, 0x0B, 0x0C, 0x19, 0x12, 0x13, 0x0F, 0x14, 0x1D, 0x1A, 0x1F, 0x1E, 0x1D, 0x1A, 0x1C, 0x1C, 0x20, 0x24, 0x2E, 0x27, 0x20, 0x22, 0x2C, 0x23, 0x1C, 0x1C, 0x28, 0x37, 0x29, 0x2C, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1F, 0x27, 0x39, 0x3D, 0x38, 0x32, 0x3C, 0x2E, 0x33, 0x34, 0x32, 0xFF, 0xC0, 0x00, 0x0B, 0x08, 0x00, 0x01, 0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xFF, 0xC4, 0x00, 0x1F, 0x00, 0x00, 0x01, 0x05, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0xFF, 0xC4, 0x00, 0xB5, 0x10, 0x00, 0x02, 0x01, 0x03, 0x03, 0x02, 0x04, 0x03, 0x05, 0x05, 0x04, 0x04, 0x00, 0x00, 0x01, 0x7D, 0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12, 0x21, 0x31, 0x41, 0x06, 0x13, 0x51, 0x61, 0x07, 0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xA1, 0x08, 0x23, 0x42, 0xB1, 0xC1, 0x15, 0x52, 0xD1, 0xF0, 0x24, 0x33, 0x62, 0x72, 0x82, 0x09, 0x0A, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2A, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3A, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4A, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5A, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6A, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79, 0x7A, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89, 0x8A, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9A, 0xA2, 0xA3, 0xA4, 0xA5, 0xA6, 0xA7, 0xA8, 0xA9, 0xAA, 0xB2, 0xB3, 0xB4, 0xB5, 0xB6, 0xB7, 0xB8, 0xB9, 0xBA, 0xC2, 0xC3, 0xC4, 0xC5, 0xC6, 0xC7, 0xC8, 0xC9, 0xCA, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7, 0xD8, 0xD9, 0xDA, 0xE1, 0xE2, 0xE3, 0xE4, 0xE5, 0xE6, 0xE7, 0xE8, 0xE9, 0xEA, 0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3F, 0x00, 0xFB, 0xD5, 0xDB, 0x20, 0x00, 0x18, 0xFF, 0xD9, 0xFF, 0xD9,
	}

	img, err := decodeImage(bytes.NewReader(testJPEG))
	if err != nil {
		t.Fatalf("Failed to decode test JPEG: %v", err)
	}

	if img == nil {
		t.Fatal("Expected non-nil image")
	}

	t.Logf("Decoded image size: %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
}

func TestDecodeImageInvalid(t *testing.T) {
	invalidData := []byte("not an image")

	_, err := decodeImage(bytes.NewReader(invalidData))
	if err == nil {
		t.Error("Expected error for invalid image data")
	}
}

func TestGetCards(t *testing.T) {
	cards := getCards()

	if len(cards) != 3 {
		t.Errorf("Expected 3 cards, got %d", len(cards))
	}

	foundMickey := false
	foundDonald := false
	foundGoofy := false

	for _, card := range cards {
		switch card.Name {
		case "Mickey Mouse":
			foundMickey = true
		case "Donald Duck":
			foundDonald = true
		case "Goofy":
			foundGoofy = true
		}
	}

	if !foundMickey || !foundDonald || !foundGoofy {
		t.Errorf("Expected to find all cards, got: %v", cards)
	}
}

func TestImportCards(t *testing.T) {
	// Create a test JSON file
	tmpfile, err := os.CreateTemp("", "test_cards_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	testData := []byte(`[{"name": "Test Card 1", "c": 10, "co": 10, "e": 10, "s": 10, "grade": 10.0, "image_url": "http://example.com/1.jpg"}]`)
	if _, err := tmpfile.Write(testData); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	tmpfile.Close()

	// Import the cards
	importCards(tmpfile.Name())
}
