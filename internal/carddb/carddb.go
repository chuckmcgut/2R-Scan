// Package carddb provides Lorcana card data management with Scryfall import.
package carddb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Card represents a Lorcana trading card.
type Card struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	SetCode    string  `json:"set_code"`
	SetName    string  `json:"set_name"`
	InkType    string  `json:"ink_type"`
	TypeLine   string  `json:"type_line"`
	Rarity     string  `json:"rarity"`
	CardNumber string  `json:"card_number"`
	FullArt    bool    `json:"full_art"`
	AltArt     bool    `json:"alt_art"`
	Signed     bool    `json:"signed"`
	FirstEdit  bool    `json:"first_edition"`
	ImageURL   string  `json:"image_url"`
}

// IngestResult tracks what a Scryfall import did.
type IngestResult struct {
	CardsAdded   int   `json:"cards_added"`
	CardsUpdated int   `json:"cards_updated"`
	CardsTotal   int   `json:"cards_total"`
	RunAt        int64 `json:"run_at"`
}

// DB wraps a SQLite database of Lorcana cards.
type DB struct {
	db *sql.DB
}

// New creates/opens the card database at the given path.
func New(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening db: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := createSchema(db); err != nil {
		return nil, fmt.Errorf("creating schema: %w", err)
	}
	return &DB{db: db}, nil
}

func createSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS cards (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		set_code TEXT NOT NULL,
		set_name TEXT NOT NULL,
		ink_type TEXT NOT NULL,
		type_line TEXT NOT NULL,
		rarity TEXT NOT NULL,
		card_number TEXT NOT NULL,
		full_art INTEGER NOT NULL DEFAULT 0,
		alt_art INTEGER NOT NULL DEFAULT 0,
		signed INTEGER NOT NULL DEFAULT 0,
		first_edition INTEGER NOT NULL DEFAULT 0,
		image_url TEXT,
		UNIQUE(set_code, card_number)
	);
	CREATE INDEX IF NOT EXISTS idx_cards_name ON cards(name);
	CREATE INDEX IF NOT EXISTS idx_cards_set ON cards(set_code, card_number);
	CREATE TABLE IF NOT EXISTS ingest_log (
		id INTEGER PRIMARY KEY,
		run_at INTEGER NOT NULL,
		cards_added INTEGER NOT NULL,
		cards_updated INTEGER NOT NULL,
		cards_total INTEGER NOT NULL
	);
	`
	_, err := db.Exec(schema)
	return err
}

// IngestFromScryfall pulls all Lorcana cards from the Scryfall API and upserts them.
func (d *DB) IngestFromScryfall() (*IngestResult, error) {
	result := &IngestResult{RunAt: time.Now().Unix()}
	page := 1

	for {
		url := fmt.Sprintf("https://api.scryfall.com/cards/search?q=set:lorcana&include_extras=true&page=%d", page)
		cards, hasMore, count, err := fetchScryfallPage(url)
		if err != nil {
			return nil, fmt.Errorf("fetching page %d: %w", page, err)
		}

		for _, c := range cards {
			added, err := d.upsertCard(&c)
			if err != nil {
				return nil, fmt.Errorf("upserting card %s: %w", c.Name, err)
			}
			if added {
				result.CardsAdded++
			} else {
				result.CardsUpdated++
			}
		}

		result.CardsTotal = count
		if !hasMore {
			break
		}
		page++
		time.Sleep(50 * time.Millisecond) // Scryfall rate limit: 10 req/sec
	}

	_, _ = d.db.Exec(
		"INSERT INTO ingest_log (run_at, cards_added, cards_updated, cards_total) VALUES (?, ?, ?, ?)",
		result.RunAt, result.CardsAdded, result.CardsUpdated, result.CardsTotal,
	)
	return result, nil
}

func (d *DB) upsertCard(c *Card) (bool, error) {
	var existing int
	err := d.db.QueryRow("SELECT 1 FROM cards WHERE set_code=? AND card_number=?", c.SetCode, c.CardNumber).Scan(&existing)
	if err == nil {
		_, err = d.db.Exec(`
			UPDATE cards SET name=?, set_name=?, ink_type=?, type_line=?, rarity=?,
			full_art=?, alt_art=?, signed=?, first_edition=?, image_url=?
			WHERE set_code=? AND card_number=?`,
			c.Name, c.SetName, c.InkType, c.TypeLine, c.Rarity,
			btoi(c.FullArt), btoi(c.AltArt), btoi(c.Signed), btoi(c.FirstEdit), c.ImageURL,
			c.SetCode, c.CardNumber,
		)
		return false, err
	}
	if err != sql.ErrNoRows {
		return false, err
	}
	_, err = d.db.Exec(`
		INSERT INTO cards (name, set_code, set_name, ink_type, type_line, rarity, card_number,
		full_art, alt_art, signed, first_edition, image_url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.SetCode, c.SetName, c.InkType, c.TypeLine, c.Rarity, c.CardNumber,
		btoi(c.FullArt), btoi(c.AltArt), btoi(c.Signed), btoi(c.FirstEdit), c.ImageURL,
	)
	return true, err
}

// FindByName does a fuzzy match on card name.
func (d *DB) FindByName(name string) ([]Card, error) {
	rows, err := d.db.Query(`
		SELECT id, name, set_code, set_name, ink_type, type_line, rarity, card_number,
		       full_art, alt_art, signed, first_edition, image_url
		FROM cards WHERE name LIKE ? ORDER BY name LIMIT 20`,
		"%"+name+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCards(rows)
}

// FindExact returns a card by set_code + card_number.
func (d *DB) FindExact(setCode, cardNumber string) (*Card, error) {
	row := d.db.QueryRow(`
		SELECT id, name, set_code, set_name, ink_type, type_line, rarity, card_number,
		       full_art, alt_art, signed, first_edition, image_url
		FROM cards WHERE set_code=? AND card_number=?`,
		setCode, cardNumber,
	)
	var c Card
	var fullArt, altArt, signed, firstEdit int
	err := row.Scan(&c.ID, &c.Name, &c.SetCode, &c.SetName, &c.InkType, &c.TypeLine,
		&c.Rarity, &c.CardNumber, &fullArt, &altArt, &signed, &firstEdit, &c.ImageURL)
	if err != nil {
		return nil, err
	}
	c.FullArt = fullArt == 1
	c.AltArt = altArt == 1
	c.Signed = signed == 1
	c.FirstEdit = firstEdit == 1
	return &c, nil
}

// ListAll returns all cards with optional limit.
func (d *DB) ListAll(limit int) ([]Card, error) {
	q := "SELECT id, name, set_code, set_name, ink_type, type_line, rarity, card_number, full_art, alt_art, signed, first_edition, image_url FROM cards ORDER BY name"
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := d.db.Query(q)
	if err != nil {
		return nil, err
	}
	return scanCards(rows)
}

// Count returns total card count.
func (d *DB) Count() (int, error) {
	var n int
	err := d.db.QueryRow("SELECT COUNT(*) FROM cards").Scan(&n)
	return n, err
}

// LastIngest returns the most recent ingest result.
func (d *DB) LastIngest() (*IngestResult, error) {
	var r IngestResult
	err := d.db.QueryRow(
		"SELECT run_at, cards_added, cards_updated, cards_total FROM ingest_log ORDER BY run_at DESC LIMIT 1",
	).Scan(&r.RunAt, &r.CardsAdded, &r.CardsUpdated, &r.CardsTotal)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func scanCards(rows *sql.Rows) ([]Card, error) {
	var cards []Card
	for rows.Next() {
		var c Card
		var fullArt, altArt, signed, firstEdit int
		err := rows.Scan(&c.ID, &c.Name, &c.SetCode, &c.SetName, &c.InkType, &c.TypeLine,
			&c.Rarity, &c.CardNumber, &fullArt, &altArt, &signed, &firstEdit, &c.ImageURL)
		if err != nil {
			return nil, err
		}
		c.FullArt = fullArt == 1
		c.AltArt = altArt == 1
		c.Signed = signed == 1
		c.FirstEdit = firstEdit == 1
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

func btoi(b bool) int { if b { return 1 }; return 0 }

// Scryfall API types.
type scryResponse struct {
	Data       []json.RawMessage `json:"data"`
	HasMore    bool              `json:"has_more"`
	TotalCards int               `json:"total_cards"`
}

func fetchScryfallPage(url string) ([]Card, bool, int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, false, 0, err
	}
	req.Header.Set("User-Agent", "2R-Scan/1.0 ( Lorcana card database)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, false, 0, fmt.Errorf("scryfall returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, 0, err
	}

	var sr scryResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, false, 0, fmt.Errorf("parsing scryfall response: %w", err)
	}

	cards := make([]Card, 0, len(sr.Data))
	for _, raw := range sr.Data {
		c, err := parseScryCard(raw)
		if err != nil {
			continue // skip malformed cards
		}
		cards = append(cards, c)
	}

	return cards, sr.HasMore, sr.TotalCards, nil
}

type rawScryCard struct {
	Name         string          `json:"name"`
	Set          string          `json:"set"`
	SetName      string          `json:"set_name"`
	TypeLine     string          `json:"type_line"`
	Rarity       string          `json:"rarity"`
	CollectorNum string          `json:"collector_number"`
	ImageURIs    json.RawMessage `json:"image_uris"`
	Foil         bool            `json:"foil"`
	FullArt      bool            `json:"full_art"`
	AltArt       bool            `json:"alternative"`
}

func parseScryCard(raw json.RawMessage) (Card, error) {
	var rawc rawScryCard
	if err := json.Unmarshal(raw, &rawc); err != nil {
		return Card{}, err
	}

	imageURL := extractImageURL(rawc.ImageURIs)
	inkType := inferInkType(rawc.TypeLine)

	return Card{
		Name:       rawc.Name,
		SetCode:    rawc.Set,
		SetName:    rawc.SetName,
		InkType:    inkType,
		TypeLine:   rawc.TypeLine,
		Rarity:     normalizeRarity(rawc.Rarity),
		CardNumber: rawc.CollectorNum,
		FullArt:    rawc.FullArt,
		AltArt:     rawc.AltArt,
		Signed:     false,
		FirstEdit:  false,
		ImageURL:   imageURL,
	}, nil
}

func extractImageURL(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	var uris map[string]string
	if err := json.Unmarshal(raw, &uris); err != nil {
		return ""
	}
	if url, ok := uris["normal"]; ok {
		return url
	}
	if url, ok := uris["large"]; ok {
		return url
	}
	for _, v := range uris {
		return v
	}
	return ""
}

// inferInkType guesses Lorcana ink type from type line text.
func inferInkType(typeLine string) string {
	upper := strings.ToUpper(typeLine)
	inkWords := []string{"AMBER", "AMETHYST", "EMERALD", "RUBY", "SAPPHIRE", "STEEL"}
	for _, ink := range inkWords {
		if strings.Contains(upper, ink) {
			return strings.Title(strings.ToLower(ink))
		}
	}
	return "Unknown"
}

func normalizeRarity(r string) string {
	switch strings.ToLower(r) {
	case "mythic": return "Legendary"
	case "rare":   return "Rare"
	case "uncommon": return "Uncommon"
	default:       return "Common"
	}
}