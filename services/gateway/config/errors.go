package config

import "errors"

var (
	ErrNilServicePool = errors.New("ServicePool cannot be nil")
	ErrNilService     = errors.New("ServiceManagerI cannot be nil")
	ErrNodeExists     = errors.New("namespace already exists with this name")
	ErrNodeNotExists  = errors.New("namespace does not exist with this name")

	ErrProjectIdValid     = errors.New("project id is not valid")
	ErrEnvironmentIdValid = errors.New("environment id is not valid")
	ErrIdValid            = errors.New("id is not valid")
	ErrTokenFormat        = errors.New("token error: wrong format")
)
