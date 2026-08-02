package ratelimit

import (
	"testing"
	"time"
)

func TestDailyQuota_ReserveAndExhaust(t *testing.T) {
	q := NewDailyQuota(2, 5)

	if err := q.Reserve(ClassPoll); err != nil {
		t.Fatalf("reserve 1: %v", err)
	}
	if err := q.Reserve(ClassPoll); err != nil {
		t.Fatalf("reserve 2: %v", err)
	}
	if err := q.Reserve(ClassPoll); err == nil {
		t.Fatal("expected exhaustion on 3rd poll reserve")
	}

	// Classes are independent budgets.
	if err := q.Reserve(ClassSearch); err != nil {
		t.Fatalf("search reserve should be independent: %v", err)
	}
	if got := q.Remaining(ClassPoll); got != 0 {
		t.Errorf("remaining(poll) = %d, want 0", got)
	}
	if got := q.Remaining(ClassSearch); got != 4 {
		t.Errorf("remaining(search) = %d, want 4", got)
	}
}

func TestDailyQuota_Release(t *testing.T) {
	q := NewDailyQuota(1, 0)
	_ = q.Reserve(ClassPoll)
	if err := q.Reserve(ClassPoll); err == nil {
		t.Fatal("expected exhaustion")
	}
	q.Release(ClassPoll)
	if err := q.Reserve(ClassPoll); err != nil {
		t.Fatalf("reserve after release should succeed: %v", err)
	}
}

func TestDailyQuota_ResetsAtMidnight(t *testing.T) {
	now := time.Date(2026, 8, 2, 23, 59, 0, 0, time.UTC)
	q := newDailyQuotaWithClock(1, 1, func() time.Time { return now })

	if err := q.Reserve(ClassPoll); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := q.Reserve(ClassPoll); err == nil {
		t.Fatal("expected exhaustion before reset")
	}

	now = time.Date(2026, 8, 3, 0, 0, 1, 0, time.UTC) // cross midnight
	if err := q.Reserve(ClassPoll); err != nil {
		t.Fatalf("reserve after midnight reset should succeed: %v", err)
	}
}
