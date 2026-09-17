package service

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Limits of a self-hosted installation.
//
// ucode is open-core. A self-hosted installation serves one organisation with
// one project inside it; several of either is what the hosted product is for.
//
// That boundary used to exist by accident. Both guards read
// `cfg.Environment != PRODUCTION`, so they applied everywhere *except*
// production — and ENVIRONMENT defaults to production, which meant a stock
// installation had no limit at all. They fired locally only because the
// compose file sets ENVIRONMENT=debug. That is a developer's guard against
// stray companies on a laptop, not a product rule, and it was the wrong way
// round.
//
// Each limit is now its own setting, named for what it is, visible in
// deploy/.env.example, and independent of any debug flag. Zero means no
// limit: an operator who needs more says so deliberately instead of changing
// an unrelated mode.
type selfHostLimit struct {
	noun    string // what is being counted, singular
	setting string // the environment variable that sets it
	max     int    // 0 or less means unlimited
}

// reached returns the error to refuse a create with, or nil if there is room.
// The message is for a person reading an API response, so it says what the
// boundary is and how to move it — a limit nobody explained reads as a bug.
func (l selfHostLimit) reached(existing int) error {
	if l.max <= 0 || existing < l.max {
		return nil
	}

	noun := l.noun
	if l.max != 1 {
		noun += "s"
	}

	return status.Error(codes.FailedPrecondition, fmt.Sprintf(
		"this installation is set up for %d %s (%s=%d). Raise %s if you need more.",
		l.max, noun, l.setting, l.max, l.setting))
}
