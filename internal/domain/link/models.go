package link

import "time"

type Link struct {
	ShortCode  string
	LongURL    string
	CreatedAt  time.Time
	ClickCount int64
}

// Cursor marks a position in the list: "continue after this link".
type Cursor struct {
	CreatedAt time.Time
	ShortCode string
}

// Page is one slice of the list. Next is nil on the last page.
type Page struct {
	Links []Link
	Next  *Cursor
}
