package domain

import "errors"

var (
	ErrNoteNotFound = errors.New("note not found")
	ErrInvalidInput = errors.New("invalid input provided")
	ErrInternal     = errors.New("internal server error")
)
