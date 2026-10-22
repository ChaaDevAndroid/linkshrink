package link

import "errors"

// Domain errors. Adapters translate them: REST maps them to HTTP statuses,
// the Postgres adapter produces ErrNotFound and ErrCodeTaken from SQL errors.
var (
	ErrNotFound   = errors.New("link not found")
	ErrCodeTaken  = errors.New("short code already taken")
	ErrInvalidURL = errors.New("url must be a valid http or https URL")
)
