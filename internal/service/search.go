// Package service holds the application's domain logic — the layer the HTTP
// handlers call. It is deliberately thin in Phase 1 (just search) and grows to
// own watch creation, evaluation wiring, etc. in later phases.
package service

import (
	"context"
	"errors"

	"ticket-watcher/internal/ticketmaster"
)

// ErrEmptyQuery is returned when a search is attempted with no keyword.
var ErrEmptyQuery = errors.New("search query must not be empty")

// SearchService performs interactive event searches against Ticketmaster.
type SearchService struct {
	tm *ticketmaster.Client
}

// NewSearchService constructs a SearchService.
func NewSearchService(tm *ticketmaster.Client) *SearchService {
	return &SearchService{tm: tm}
}

// Search validates the query and returns matching events.
func (s *SearchService) Search(ctx context.Context, q string) ([]ticketmaster.EventSnapshot, error) {
	if q == "" {
		return nil, ErrEmptyQuery
	}
	return s.tm.Search(ctx, q)
}
