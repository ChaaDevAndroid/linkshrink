package link

import "time"

type Link struct {
	ShortCode  string
	LongURL    string
	CreatedAt  time.Time
	ClickCount int64
}
