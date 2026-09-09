package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLegacySecrets(t *testing.T) {
	require.Equal(t, []string{"legacy", "older"}, legacySecrets("primary", "legacy", "legacy, older, primary"))
	require.Empty(t, legacySecrets("same", "same", ""))
}

func TestFirstNonEmpty(t *testing.T) {
	require.Equal(t, "preferred", firstNonEmpty(" ", " preferred ", "fallback"))
}
