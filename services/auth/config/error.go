package config

import "errors"

var (
	ErrNilServicePool = errors.New("ServicePool cannot be nil")
	ErrNilService     = errors.New("ServiceManagerI cannot be nil")
	ErrNodeExists     = errors.New("namespace already exists with this name")
	ErrNodeNotExists  = errors.New("namespace does not exist with this name")
	ErrPasswordLength = errors.New("password must not be less than 6 characters")
	ErrFailedUpdate   = errors.New("password updated in auth but failed to update in object builder")
	ErrInvalidEmail   = errors.New("email is not valid")

	ErrProjectIdValid     = errors.New("project id is not valid")
	ErrEnvironmentIdValid = errors.New("environment id is not valid")
)

const (
	ErrPasswordHash             = "Failed to hash password. Please ensure the password is valid."
	ErrClientTypeRoleIDRequired = "Client Type ID and Role ID are required fields."
	ErrInvalidUserEmail         = "Email is not valid."
	ErrUserExists               = "User already exists."
	ErrIncorrectLoginOrPassword = "Incorrect login or password."
	ErrInvalidJSON              = "Invalid JSON."
	ErrWrong                    = "Something went wrong."
	ErrOutOfWork                = "Temporarily out of work."
	ErrGoogle                   = "This Google account is not registered."
	ErrEmailExists              = "This email already exists."
	ErrPhoneExists              = "This phone already exists."
)

var (
	EmailConstraint = "user_unq_email"
	PhoneConstraint = "user_unq_phone"
)
