package main

import (
	"encoding/json"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestCSVRow(t *testing.T) {
	row := csvRow(userBook{
		DateAdded:    "2026-07-02",
		LastReadDate: "2026-07-01",
		Rating:       "4.5",
		ReadCount:    1,
		Review:       "good",
		StatusID:     3,
		Book:         book{Title: "Dune", ReleaseDate: "1965-08-01", Pages: json.Number("412")},
		Edition: &edition{
			CachedContributors: []any{map[string]any{"author": map[string]any{"name": "Frank Herbert"}}},
			ISBN13:             "9780441172719",
			Pages:              json.Number("535"),
			Publisher:          &publisher{Name: "Ace"},
		},
	})

	checks := map[int]string{
		1:  "Dune",
		2:  "Frank Herbert",
		6:  `="9780441172719"`,
		7:  "5",
		14: "2026/07/01",
		18: "read",
	}
	for column, want := range checks {
		if row[column] != want {
			t.Fatalf("column %d = %q, want %q", column, row[column], want)
		}
	}
}

func TestTokenForExportNormalizesTypedToken(t *testing.T) {
	got, err := tokenForExport("abc123")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Bearer abc123" {
		t.Fatalf("token = %q", got)
	}
}

func TestTokenForExportUsesSavedToken(t *testing.T) {
	keyring.MockInit()
	if err := keyring.Set(keyringService, keyringUser, "Bearer saved"); err != nil {
		t.Fatal(err)
	}

	got, err := tokenForExport("")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Bearer saved" {
		t.Fatalf("token = %q", got)
	}
}
