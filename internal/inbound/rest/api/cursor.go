package api

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"golinkshrink.com/linkshrink/internal/domain/link"
)

var errBadCursor = errors.New("malformed cursor")

// encodeCursor turns a position into an opaque URL-safe string.
// Clients must treat it as a black box and pass it back unchanged.
func encodeCursor(c link.Cursor) string {
	raw := c.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + c.ShortCode
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (*link.Cursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, errBadCursor
	}

	ts, code, ok := strings.Cut(string(b), "|")
	if !ok || code == "" || len(code) > 64 {
		return nil, errBadCursor
	}

	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return nil, errBadCursor
	}
	return &link.Cursor{CreatedAt: t, ShortCode: code}, nil
}
