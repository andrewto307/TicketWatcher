// Package money converts between the dollars the API/UI speak and the integer
// cents the database stores. Storing money as integer minor units avoids float
// rounding error; conversion happens only at these boundaries.
package money

import "math"

// ToCents converts optional dollars to optional integer cents.
func ToCents(dollars *float64) *int64 {
	if dollars == nil {
		return nil
	}
	c := int64(math.Round(*dollars * 100))
	return &c
}

// ToDollars converts optional integer cents to optional dollars.
func ToDollars(cents *int64) *float64 {
	if cents == nil {
		return nil
	}
	d := float64(*cents) / 100
	return &d
}
