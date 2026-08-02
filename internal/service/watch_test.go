package service

import (
	"context"
	"errors"
	"testing"
)

// Create validates its input before touching the DB or Ticketmaster, so the
// validation branches are testable with nil dependencies.
func TestWatchService_CreateValidation(t *testing.T) {
	s := NewWatchService(nil, nil, 1)
	thr := 100.0

	tests := []struct {
		name string
		in   CreateWatchInput
		want error
	}{
		{"missing event id", CreateWatchInput{ConditionType: "price_below", Threshold: &thr}, ErrMissingEventID},
		{"invalid condition", CreateWatchInput{TMEventID: "x", ConditionType: "bogus"}, ErrInvalidCondition},
		{"price_below without threshold", CreateWatchInput{TMEventID: "x", ConditionType: "price_below"}, ErrThresholdRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := s.Create(context.Background(), tt.in); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}
