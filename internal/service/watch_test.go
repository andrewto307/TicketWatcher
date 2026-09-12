package service

import (
	"context"
	"errors"
	"testing"
)

// Create validates its input before touching the DB or Ticketmaster, so the
// validation branches are testable with nil dependencies.
func TestWatchService_CreateValidation(t *testing.T) {
	s := NewWatchService(nil, nil, 0)

	tests := []struct {
		name string
		in   CreateWatchInput
		want error
	}{
		{"missing event id", CreateWatchInput{ConditionType: "becomes_available"}, ErrMissingEventID},
		{"invalid condition", CreateWatchInput{TMEventID: "x", ConditionType: "bogus"}, ErrInvalidCondition},
		// price_below was removed with the Discovery API's pricing (D13); a leftover
		// value from an old client must be rejected, not silently accepted.
		{"price_below no longer accepted", CreateWatchInput{TMEventID: "x", ConditionType: "price_below"}, ErrInvalidCondition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := s.Create(context.Background(), 1, tt.in); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}
