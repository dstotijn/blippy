package store

import "database/sql"

// NewNullString creates a sql.NullString from a string.
// If the string is empty, it returns an invalid NullString.
func NewNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
