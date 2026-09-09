package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"

	"github.com/stretchr/testify/require"
)

type registrationUserRepoStub struct {
	storage.UserRepoI
	err error
}

func (s *registrationUserRepoStub) AddUserToProject(context.Context, *pb.AddUserToProjectReq) (*pb.AddUserToProjectRes, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &pb.AddUserToProjectRes{}, nil
}

func TestAddUserProjectForRegistrationCreated(t *testing.T) {
	created, err := addUserProjectForRegistration(
		context.Background(),
		&registrationUserRepoStub{},
		&pb.AddUserToProjectReq{},
	)

	require.NoError(t, err)
	require.True(t, created)
}

func TestAddUserProjectForRegistrationContinuesExistingMembership(t *testing.T) {
	created, err := addUserProjectForRegistration(
		context.Background(),
		&registrationUserRepoStub{err: errors.New("duplicate key violates unique constraint " + config.UserProjectIdConstraint)},
		&pb.AddUserToProjectReq{},
	)

	require.NoError(t, err)
	require.False(t, created)
}

func TestAddUserProjectForRegistrationReturnsUnexpectedError(t *testing.T) {
	expectedErr := errors.New("database unavailable")
	created, err := addUserProjectForRegistration(
		context.Background(),
		&registrationUserRepoStub{err: expectedErr},
		&pb.AddUserToProjectReq{},
	)

	require.ErrorIs(t, err, expectedErr)
	require.False(t, created)
}
