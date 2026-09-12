package config

import "errors"

var (
	ErrNilServicePool            = errors.New("ServicePool cannot be nil")
	ErrNilService                = errors.New("ServiceManagerI cannot be nil")
	ErrNodeExists                = errors.New("namespace already exists with this name")
	ErrNodeNotExists             = errors.New("namespace does not exist with this name")
	ErrBalanceInsuffient         = errors.New("balance + credit limit is less than fare price")
	ErrFareAlreadyExists         = errors.New("fare already exists")
	ErrUcodePlanChangeNotAllowed = errors.New("ucode plans cannot be changed via AttachFare")
	ErrSamePlanAlreadyActive     = errors.New("project is already subscribed to this plan")
	ErrBillingPeriodPlanChange   = errors.New("billing period cannot be changed together with plan change")
	ErrSubscriptionCancelPending = errors.New("subscription is scheduled to cancel at period end")
	ErrDowngradePeriodEnded      = errors.New("cannot cancel downgrade after subscription period ended")
	ErrMinioNotConfigured        = errors.New("minio storage is not configured")
	ErrTokenPackNotFound         = errors.New("token pack not found or inactive")
	ErrInvalidTokenPackAmount    = errors.New("token pack amount must be greater than zero")
	ErrInvalidChargeAmount       = errors.New("charge amount must be greater than zero")
	ErrProjectNotFound           = errors.New("project not found")
	ErrTransactionNotFound       = errors.New("transaction not found")
	ErrTransactionNotRefundable  = errors.New("transaction is not a refundable charge")

	ErrBadRequest = "Bad request"
)
