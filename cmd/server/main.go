package main

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/chuckmcgut/2R-Scan/internal/carddb"
	"github.com/chuckmcgut/2R-Scan/internal/grading"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type App struct {
	db      *carddb.DB
	scanDir string
	limiter *rateLimiter
}

type rateLimiter struct {
	mu       sync.RWMutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{requests: make(map[string][]time.Time), limit: limit, window: window}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	windowStart := now.Add(-rl.window)
	var recent []time.Time
	for _, t := range rl.requests[ip] {
		if t.After(windowStart) {
			recent = append(recent, t)
		}
	}
	rl.requests[ip] = recent
	if len(recent) >= rl.limit {
		return false
	}
	rl.requests[ip] = append(rl.requests[ip], now)
	return true
}

func main() {
	dbPath := os.Getenv("DATABASE_URL")
	if dbPath == "" {
		dbPath = "cards.db"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	scanDir := os.Getenv("SCAN_DIR")
	if scanDir == "" {
		scanDir = "/tmp/scans"
	}
	os.MkdirAll(scanDir, 0755)

	db, err := carddb.New(dbPath)
	if err != nil {
		log.Fatalf("opening card db: %v", err)
	}
	defer db.Close()

	cardCount, _ := db.Count()
	log.Printf("Card DB loaded: %d cards", cardCount)

	app := &App{db: db, scanDir: scanDir, limiter: newRateLimiter(10, time.Minute)}

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	e.GET("/health", app.health)
	e.GET("/api/v1/cards", app.listCards)
	e.GET("/api/v1/cards/search", app.searchCards)
	e.POST("/api/v1/scan", app.scanCard)
	e.POST("/api/v1/ingest", app.ingest)

	log.Printf("2R-Scan server starting on :%s", port)
	if err := e.Start(":" + port); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func (a *App) health(c echo.Context) error {
	count, _ := a.db.Count()
	ingest, _ := a.db.LastIngest()
	return c.JSON(200, map[string]interface{}{
		"status":      "ok",
		"version":     "0.1.0",
		"cards_in_db": count,
		"last_ingest": ingest,
	})
}

func (a *App) listCards(c echo.Context) error {
	matches, err := a.db.ListAll(50)
	if err != nil {
		return c.String(500, "database error")
	}
	return c.JSON(200, map[string]interface{}{"cards": matches, "count": len(matches)})
}

func (a *App) searchCards(c echo.Context) error {
	name := c.QueryParam("name")
	if name == "" {
		return c.String(400, "name query required")
	}
	matches, err := a.db.FindByName(name)
	if err != nil {
		return c.String(500, "search error")
	}
	return c.JSON(200, map[string]interface{}{"cards": matches, "count": len(matches)})
}

func (a *App) scanCard(c echo.Context) error {
	ip := c.RealIP()
	if !a.limiter.allow(ip) {
		c.Response().Header().Set("Retry-After", "60")
		return c.String(429, "rate limit exceeded (10 scans/min)")
	}

	const maxSize = 10 << 20
	if err := c.Request().ParseMultipartForm(maxSize); err != nil {
		return c.String(400, "invalid upload: must be multipart form, max 10MB")
	}

	file, header, err := c.Request().FormFile("image")
	if err != nil {
		return c.String(400, "missing 'image' form field")
	}
	defer file.Close()

	if header.Size > maxSize {
		return c.String(400, "image too large (max 10MB)")
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return c.String(500, "failed to read image")
	}

	img, format, err := decodeImage(bytes.NewReader(data))
	if err != nil {
		return c.String(400, fmt.Sprintf("unrecognized image format: %v", err))
	}

	imageID := uuid.New().String()
	savePath := filepath.Join(a.scanDir, imageID+"."+format)
	os.WriteFile(savePath, data, 0644)

	grade := grading.GradeImage(img)

	cardName := c.FormValue("card_name")
	result := map[string]interface{}{
		"grade":     grade,
		"image_id":  imageID,
		"format":    format,
		"timestamp": time.Now().Unix(),
	}

	if cardName != "" {
		if matches, err := a.db.FindByName(cardName); err == nil && len(matches) > 0 {
			result["card_name"] = matches[0].Name
			result["set_code"] = matches[0].SetCode
			result["set_name"] = matches[0].SetName
			result["ink_type"] = matches[0].InkType
			result["rarity"] = matches[0].Rarity
		}
	}

	log.Printf("Scanned %s (grade %.1f, format=%s)", imageID, grade.Overall, format)
	return c.JSON(200, result)
}

func (a *App) ingest(c echo.Context) error {
	result, err := a.db.IngestFromScryfall()
	if err != nil {
		return c.String(500, fmt.Sprintf("ingest failed: %v", err))
	}
	return c.JSON(200, result)
}

// decodeImage auto-detects JPEG or PNG from magic bytes.
func decodeImage(r *bytes.Reader) (image.Image, string, error) {
	head := make([]byte, 4)
	if _, err := r.Read(head); err != nil {
		return nil, "", err
	}
	r.Seek(0, 0)

	if bytes.Equal(head[:2], []byte{0xFF, 0xD8}) {
		img, err := jpeg.Decode(r)
		return img, "jpg", err
	}
	if bytes.Equal(head[:4], []byte{0x89, 0x50, 0x4E, 0x47}) {
		img, err := png.Decode(r)
		return img, "png", err
	}

	return nil, "", fmt.Errorf("unsupported image format (not JPEG or PNG)")
}