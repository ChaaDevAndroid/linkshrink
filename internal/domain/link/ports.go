package link

import "context"

// Service is the inbound port: what adapters (REST, gRPC, CLI) can ask the domain to do.
type Service interface {
	// Create generates a short code for longURL and stores the link.
	Create(ctx context.Context, longURL string) (Link, error)

	// Resolve finds a link by its short code.
	//
	// Errors:
	//   - ErrNotFound if no link has that code.
	Resolve(ctx context.Context, code string) (Link, error)

	// Stats finds a link by its short code WITHOUT recording a visit.
	//
	// Errors:
	//   - ErrNotFound if no link has that code.
	Stats(ctx context.Context, code string) (Link, error)
}

// Repository is the outbound port: what the domain needs from storage.
type Repository interface {
	// Save persists a new link.
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
}
