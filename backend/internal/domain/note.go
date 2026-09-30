package domain

import (
	"fmt"
	"time"
	"unicode/utf8"
)

// Bounds for a valid note. Titles are measured in runes so that
// multi-byte languages are not penalised.
const (
	MinTitleLength = 3
	MaxTitleLength = 100
)

// Note represents a simple note entity in the system.
type Note struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate enforces the note's invariants. It returns an error wrapping
// ErrInvalidInput so that transports can map it to their own status codes
// without duplicating the rules.
func (n *Note) Validate() error {
	title := n.Title
	switch length := utf8.RuneCountInString(title); {
	case length == 0:
		return fmt.Errorf("%w: title is required", ErrInvalidInput)
	case length < MinTitleLength:
		return fmt.Errorf("%w: title must be at least %d characters", ErrInvalidInput, MinTitleLength)
	case length > MaxTitleLength:
		return fmt.Errorf("%w: title must be at most %d characters", ErrInvalidInput, MaxTitleLength)
	}

	if n.Content == "" {
		return fmt.Errorf("%w: content is required", ErrInvalidInput)
	}

	return nil
}
