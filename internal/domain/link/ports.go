package link

import "context"

// Service is the inbound port: what adapters (REST, gRPC, CLI) can ask the domain to do.
type Service interface {
	// Create validates longURL, generates a short code and stores the link.
	//
	// Errors:
	//   - ErrInvalidURL if longURL is not a valid http or https URL.
	Create(ctx context.Context, longURL string) (Link, error)

	// Resolve finds a link by its short code and records a visit.
	//
	// Errors:
	//   - ErrNotFound if no link has that code.
	Resolve(ctx context.Context, code string) (Link, error)

	// Stats finds a link by its short code WITHOUT recording a visit.
	//
	// Errors:
	//   - ErrNotFound if no link has that code.
	Stats(ctx context.Context, code string) (Link, error)

	// List returns up to limit links, newest first, starting after the given cursor.
	// A nil cursor means "from the newest".
	//
	// Errors:
	//   - ErrInvalidLimit if limit is not between 1 and MaxPageSize.
	List(ctx context.Context, limit int, after *Cursor) (Page, error)
}

// Repository is the outbound port: what the domain needs from storage.
type Repository interface {
	// Save persists a new link.
	//
	// Errors:
	//   - ErrCodeTaken if the short code already exists.
	Save(ctx context.Context, l Link) error

	// FindByCode returns the link with that code.
	//
	// Errors:
	//   - ErrNotFound if no link has that code.
	FindByCode(ctx context.Context, code string) (Link, error)

	// RecordVisit atomically increments the link's click count and logs the visit.
	//
	// Errors:
	//   - ErrNotFound if no link has that code.
	RecordVisit(ctx context.Context, code string) error

	// List returns up to limit links ordered by created_at DESC, short_code DESC,
	// strictly after the cursor. A nil cursor starts at the newest link.
	List(ctx context.Context, after *Cursor, limit int) ([]Link, error)
}
