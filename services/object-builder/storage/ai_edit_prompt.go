package storage

import "errors"

var (
	ErrAiEditPromptNotFound         = errors.New("ai edit prompt override not found")
	ErrAiEditPromptRevisionConflict = errors.New("ai edit prompt revision conflict")
	ErrAiEditPromptTableMissing     = errors.New("ai edit prompt table is missing")
	ErrInvalidAiEditPromptKind      = errors.New("invalid ai edit prompt kind")
	ErrInvalidAiEditPromptContent   = errors.New("ai edit prompt content must not be empty")
	ErrAiEditPromptContentTooLarge  = errors.New("ai edit prompt content is too large")
	ErrInvalidAiEditPromptRevision  = errors.New("ai edit prompt revision must not be negative")
)
