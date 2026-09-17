package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSelfHostLimitAllowsUpToTheMaximum(t *testing.T) {
	limit := selfHostLimit{noun: "company", setting: "MAX_COMPANIES", max: 1}

	require.NoError(t, limit.reached(0), "the first company must be allowed")
	require.Error(t, limit.reached(1), "the second must not")
}

func TestSelfHostLimitZeroMeansUnlimited(t *testing.T) {
	// The escape hatch an operator has to reach for deliberately. It must not
	// depend on ENVIRONMENT, which is the mistake this replaces.
	limit := selfHostLimit{noun: "project", setting: "MAX_PROJECTS", max: 0}

	require.NoError(t, limit.reached(0))
	require.NoError(t, limit.reached(500))
}

func TestSelfHostLimitRefusalIsReadable(t *testing.T) {
	limit := selfHostLimit{noun: "company", setting: "MAX_COMPANIES", max: 1}
	err := limit.reached(1)

	st, ok := status.FromError(err)
	require.True(t, ok, "the refusal must be a gRPC status")
	// Not codes.Internal: nothing went wrong, the request asked for something
	// this installation does not do.
	require.Equal(t, codes.FailedPrecondition, st.Code())
	// The CLI matches on the setting name to tell a re-run from a real
	// failure, so it has to survive rewording of the sentence around it.
	require.Contains(t, st.Message(), "MAX_COMPANIES")
	require.Contains(t, st.Message(), "1 company")
}

func TestSelfHostLimitPluralisesAboveOne(t *testing.T) {
	limit := selfHostLimit{noun: "project", setting: "MAX_PROJECTS", max: 3}

	require.NoError(t, limit.reached(2))
	require.Contains(t, status.Convert(limit.reached(3)).Message(), "3 projects")
}
