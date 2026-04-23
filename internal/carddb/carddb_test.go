package carddb

import (
	"os"
	"testing"
)

func TestNewDB(t *testing.T) {
	tmp := t.TempDir() + "/test_cards.db"
	db, err := New(tmp)
	if err != nil {
		t.Fatalf("New() = %v, want nil", err)
	}

	count, err := db.Count()
	if err != nil {
		t.Fatalf("Count() = %v", err)
	}
	if count != 0 {
		t.Errorf("Count() = %d, want 0 (empty DB)", count)
	}
}

func TestUpsertAndFind(t *testing.T) {
	tmp := t.TempDir() + "/test_upsert.db"
	db, err := New(tmp)
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	card := &Card{
		Name:       "Mickey Mouse",
		SetCode:    "tle",
		SetName:    "The Lorcana Elsie",
		InkType:    "Amber",
		TypeLine:   "Character",
		Rarity:     "Rare",
		CardNumber: "1",
		FullArt:    false,
		AltArt:     false,
	}

	added, err := db.upsertCard(card)
	if err != nil {
		t.Fatalf("upsertCard() = %v", err)
	}
	if !added {
		t.Errorf("upsertCard() added=%v, want true (first insert)", added)
	}

	added, err = db.upsertCard(card)
	if err != nil {
		t.Fatalf("upsertCard() 2nd = %v", err)
	}
	if added {
		t.Errorf("upsertCard() added=%v, want false (update)", added)
	}

	count, _ := db.Count()
	if count != 1 {
		t.Errorf("Count() = %d, want 1", count)
	}
}

func TestFindByName(t *testing.T) {
	tmp := t.TempDir() + "/test_find.db"
	db, err := New(tmp)
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	cards := []*Card{
		{Name: "Mickey Mouse", SetCode: "tle", CardNumber: "1", InkType: "Amber", TypeLine: "Character", Rarity: "Rare"},
		{Name: "Mickey Mouse", SetCode: "roc", CardNumber: "5", InkType: "Sapphire", TypeLine: "Character", Rarity: "Uncommon"},
		{Name: "Simba", SetCode: "tle", CardNumber: "2", InkType: "Amber", TypeLine: "Character", Rarity: "Legendary"},
	}
	for _, c := range cards {
		db.upsertCard(c)
	}

	matches, err := db.FindByName("Mickey")
	if err != nil {
		t.Fatalf("FindByName() = %v", err)
	}
	if len(matches) != 2 {
		t.Errorf("FindByName(Mickey) returned %d matches, want 2", len(matches))
	}

	matches, err = db.FindByName("Simba")
	if err != nil {
		t.Fatalf("FindByName() = %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("FindByName(Simba) returned %d matches, want 1", len(matches))
	}

	matches, err = db.FindByName("notacard")
	if err != nil {
		t.Fatalf("FindByName() = %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("FindByName(notacard) returned %d matches, want 0", len(matches))
	}
}

func TestFindExact(t *testing.T) {
	tmp := t.TempDir() + "/test_exact.db"
	db, err := New(tmp)
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	card := &Card{Name: "Aladdin's Flying Carpet", SetCode: "tle", CardNumber: "180", InkType: "Amber", TypeLine: "Item", Rarity: "Rare"}
	db.upsertCard(card)

	found, err := db.FindExact("tle", "180")
	if err != nil {
		t.Fatalf("FindExact() = %v", err)
	}
	if found == nil {
		t.Fatal("FindExact(tle, 180) = nil, want card")
	}
	if found.Name != card.Name {
		t.Errorf("found.Name = %q, want %q", found.Name, card.Name)
	}

	notFound, err := db.FindExact("tle", "999")
	if err != nil {
		t.Fatalf("FindExact() unexpected error = %v", err)
	}
	if notFound != nil {
		t.Errorf("FindExact(tle, 999) = %v, want nil", notFound)
	}
}

func TestListAll(t *testing.T) {
	tmp := t.TempDir() + "/test_list.db"
	db, err := New(tmp)
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	for i := 0; i < 5; i++ {
		db.upsertCard(&Card{Name: "Test Card", SetCode: "set", CardNumber: string(rune('0'+i)), InkType: "Amber", TypeLine: "Item", Rarity: "Common"})
	}

	cards, err := db.ListAll(10)
	if err != nil {
		t.Fatalf("ListAll() = %v", err)
	}
	if len(cards) != 5 {
		t.Errorf("ListAll() returned %d cards, want 5", len(cards))
	}

	cards, err = db.ListAll(2)
	if err != nil {
		t.Fatalf("ListAll(2) = %v", err)
	}
	if len(cards) != 2 {
		t.Errorf("ListAll(2) returned %d cards, want 2", len(cards))
	}
}

func TestNormalizeRarity(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"mythic", "Legendary"},
		{"rare", "Rare"},
		{"uncommon", "Uncommon"},
		{"common", "Common"},
	}
	for _, tt := range tests {
		got := normalizeRarity(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeRarity(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestInferInkType(t *testing.T) {
	tests := []struct {
		typeLine string
		expected string
	}{
		{"Character — Amber", "Amber"},
		{"Character — Amethyst", "Amethyst"},
		{"Action", "Unknown"},
	}
	for _, tt := range tests {
		got := inferInkType(tt.typeLine)
		if got != tt.expected {
			t.Errorf("inferInkType(%q) = %q, want %q", tt.typeLine, got, tt.expected)
		}
	}
}

func TestDBPathCreate(t *testing.T) {
	tmp := t.TempDir()
	dbPath := tmp + "/new_cards.db"

	if _, err := os.Stat(dbPath); err == nil {
		t.Skip("db already exists")
	}

	_, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(%s) = %v, want nil", dbPath, err)
	}

	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("expected db file at %s", dbPath)
	}
}