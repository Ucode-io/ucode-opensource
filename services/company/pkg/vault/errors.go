package vaultclient

import "errors"

var (
	ErrTokenNotRenewable   = errors.New("token is not renewable")
	ErrInitLifetimeWatcher = errors.New("unable to initialize new lifetime watcher for renewing auth token")
)
