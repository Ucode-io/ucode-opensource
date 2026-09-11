package service

import (
	"context"

	pbc "github.com/Ucode-io/ucode-opensource/services/auth/genproto/company_service"
)

// Per-seat charging belongs to the billing module, which is not part of the
// open-source build. Creating a user no longer costs anything, so reserving a
// seat always succeeds and there is never a charge to release.
//
// The call sites in user_service_v2.go keep their shape: a distribution that
// reinstates billing only has to fill these two in again.

type userSeatCharge struct{}

func (s *userService) reserveUserSeat(_ context.Context, _ *pbc.Project, _ string) (*userSeatCharge, error) {
	return nil, nil
}

func (s *userService) releaseUserSeat(_ *userSeatCharge) {}
