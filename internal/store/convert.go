package store

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// TS wraps a time.Time as a valid pgtype.Timestamptz.
func TS(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// TSPtr wraps an optional time.Time; a nil pointer becomes a NULL timestamptz.
func TSPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// TimePtr converts a pgtype.Timestamptz back to an optional time.Time.
func TimePtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}
