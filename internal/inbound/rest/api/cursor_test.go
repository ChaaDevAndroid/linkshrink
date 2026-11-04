package api

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"golinkshrink.com/linkshrink/internal/domain/link"
)

func TestCursor_RoundTrip(t *testing.T) {
	// A non-UTC zone and nanoseconds on purpose: encodeCursor converts to UTC, and the
	// decoded value must still be the SAME INSTANT.
	zone := time.FixedZone("UTC+1", 3600)
	orig := link.Cursor{
		CreatedAt: time.Date(2026, 11, 2, 13, 0, 0, 123456789, zone),
		ShortCode: "abc123",
	}

	got, err := decodeCursor(encodeCursor(orig))

	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	// Equal compares the instant. == would also compare the time zone and fail.
	if !got.CreatedAt.Equal(orig.CreatedAt) || got.ShortCode != orig.ShortCode {
		t.Errorf("round trip = %+v, want %+v", got, orig)
	}
}

func TestDecodeCursor_Malformed(t *testing.T) {
	enc := base64.RawURLEncoding.EncodeToString // short alias

	tests := []struct {
		name   string
		cursor string
	}{
		{"empty", ""},
		{"not base64", "not base64 !!"},
		{"no separator", enc([]byte("2026-11-02T12:00:00Z"))},
		{"empty short code", enc([]byte("2026-11-02T12:00:00Z|"))},
		{"bad timestamp", enc([]byte("yesterday|abc123"))},
		{"short code too long", enc([]byte("2026-11-02T12:00:00Z|" + strings.Repeat("a", 65)))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if c, err := decodeCursor(tt.cursor); err == nil {
				t.Errorf("decodeCursor(%q) = %+v, want an error", tt.cursor, c)
			}
		})
	}
}
