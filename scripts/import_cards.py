#!/usr/bin/env python3
"""
Import Lorcana cards from Scryfall into the SQLite card database.
Run this once to populate the DB, then weekly via cron.

Usage:
    python3 scripts/import_cards.py [db_path]
"""

import sqlite3
import sys
import time
import json
import urllib.request

CARDS_URL = "https://api.scryfall.com/cards/search?q=set:lorcana&include_extras=true&page=1"

def get_db(db_path: str) -> sqlite3.Connection:
    conn = sqlite3.connect(db_path)
    conn.execute("""
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
        )
    """)
    conn.execute("CREATE INDEX IF NOT EXISTS idx_cards_name ON cards(name)")
    conn.execute("CREATE INDEX IF NOT EXISTS idx_cards_set ON cards(set_code, card_number)")
    conn.execute("""
        CREATE TABLE IF NOT EXISTS ingest_log (
            id INTEGER PRIMARY KEY,
            run_at INTEGER NOT NULL,
            cards_added INTEGER NOT NULL,
            cards_updated INTEGER NOT NULL,
            cards_total INTEGER NOT NULL
        )
    """)
    conn.commit()
    return conn

def normalize_rarity(r: str) -> str:
    return {"mythic": "Legendary", "rare": "Rare", "uncommon": "Uncommon"}.get(r.lower(), "Common")

def infer_ink_type(type_line: str) -> str:
    upper = type_line.upper()
    for ink in ["AMBER", "AMETHYST", "EMERALD", "RUBY", "SAPPHIRE", "STEEL"]:
        if ink in upper:
            return ink
    return "Unknown"

def extract_image_url(card_raw: dict) -> str:
    uris = card_raw.get("image_uris", {}) or {}
    return uris.get("normal") or uris.get("large") or ""

def import_page(conn: sqlite3.Connection, page_url: str) -> tuple[list, bool, int]:
    req = urllib.request.Request(page_url, headers={"User-Agent": "2R-Scan/1.0"})
    with urllib.request.urlopen(req) as resp:
        data = json.loads(resp.read())

    cards = []
    for raw in data.get("data", []):
        c = {
            "name": raw["name"],
            "set_code": raw["set"],
            "set_name": raw["set_name"],
            "ink_type": infer_ink_type(raw.get("type_line", "")),
            "type_line": raw.get("type_line", ""),
            "rarity": normalize_rarity(raw.get("rarity", "")),
            "card_number": raw.get("collector_number", ""),
            "full_art": int(bool(raw.get("full_art", False))),
            "alt_art": int(bool(raw.get("alternative", False))),
            "signed": 0,
            "first_edition": int(bool(raw.get("first_edition", False))),
            "image_url": extract_image_url(raw),
        }
        cards.append(c)
    return cards, data.get("has_more", False), data.get("total_cards", 0)

def upsert_card(conn: sqlite3.Connection, c: dict) -> bool:
    cur = conn.execute(
        "SELECT 1 FROM cards WHERE set_code=? AND card_number=?",
        (c["set_code"], c["card_number"])
    )
    exists = cur.fetchone() is not None

    if exists:
        conn.execute("""
            UPDATE cards SET name=?, set_name=?, ink_type=?, type_line=?, rarity=?,
            full_art=?, alt_art=?, signed=?, first_edition=?, image_url=?
            WHERE set_code=? AND card_number=?
        """, (c["name"], c["set_name"], c["ink_type"], c["type_line"], c["rarity"],
              c["full_art"], c["alt_art"], c["signed"], c["first_edition"], c["image_url"],
              c["set_code"], c["card_number"]))
    else:
        conn.execute("""
            INSERT INTO cards (name, set_code, set_name, ink_type, type_line, rarity, card_number,
            full_art, alt_art, signed, first_edition, image_url)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        """, (c["name"], c["set_code"], c["set_name"], c["ink_type"], c["type_line"],
              c["rarity"], c["card_number"], c["full_art"], c["alt_art"], c["signed"],
              c["first_edition"], c["image_url"]))
    return not exists

def main():
    db_path = sys.argv[1] if len(sys.argv) > 1 else "internal/carddb/cards.db"
    conn = get_db(db_path)

    page_url = CARDS_URL
    page = 1
    total_added = 0
    total_updated = 0
    total_cards = 0

    print(f"Starting Lorcana card import into {db_path}...")

    while True:
        print(f"  Fetching page {page}...")
        try:
            cards, has_more, total = import_page(conn, page_url)
        except Exception as e:
            print(f"  ERROR fetching page {page}: {e}")
            break

        for c in cards:
            added = upsert_card(conn, c)
            if added:
                total_added += 1
            else:
                total_updated += 1

        total_cards = total
        print(f"  Page {page}: {len(cards)} cards (total: {total_cards}, added: {total_added}, updated: {total_updated})")

        if not has_more:
            break

        conn.commit()
        page += 1
        time.sleep(50 / 1000)  # Scryfall rate limit

    conn.commit()

    conn.execute("""
        INSERT INTO ingest_log (run_at, cards_added, cards_updated, cards_total)
        VALUES (?, ?, ?, ?)
    """, (int(time.time()), total_added, total_updated, total_cards))
    conn.commit()

    print(f"\nDone! Added {total_added}, updated {total_updated}, total in DB: {total_cards}")
    conn.close()

if __name__ == "__main__":
    main()