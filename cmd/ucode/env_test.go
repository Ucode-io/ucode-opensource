package main

import (
	"strings"
	"testing"
)

// A released CLI has to pin the image tag to its own version, or an install
// silently follows `latest` into images it was never tested against. A build
// from a checkout reports "dev" and must not pin, because no such tag exists.
func TestDefaultEnvPinsImageVersion(t *testing.T) {
	tests := []struct {
		name     string
		cliVer   string
		envVer   string
		wantLine string
		wantNone bool
	}{
		{
			name:     "released build pins its own version",
			cliVer:   "0.1.0",
			wantLine: "UCODE_VERSION=0.1.0",
		},
		{
			name:     "development build leaves the compose default alone",
			cliVer:   "dev",
			wantNone: true,
		},
		{
			name:     "an explicit environment value wins over the pin",
			cliVer:   "0.1.0",
			envVer:   "local",
			wantLine: "UCODE_VERSION=local",
		},
		{
			name:     "an explicit value works for a development build too",
			cliVer:   "dev",
			envVer:   "local",
			wantLine: "UCODE_VERSION=local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// version is the package-level var the linker writes at release.
			original := version
			version = tt.cliVer
			t.Cleanup(func() { version = original })

			if tt.envVer != "" {
				t.Setenv("UCODE_VERSION", tt.envVer)
			} else {
				t.Setenv("UCODE_VERSION", "")
			}

			got := defaultEnv("test-key")

			switch {
			case tt.wantNone:
				if strings.Contains(got, "UCODE_VERSION=") {
					t.Errorf("a dev build pinned a version it cannot have published:\n%s", got)
				}
			default:
				if !strings.Contains(got, tt.wantLine) {
					t.Errorf("want %q in the written .env, got:\n%s", tt.wantLine, got)
				}
			}
		})
	}
}

// The signing key is generated per installation; the template must carry the
// one it was given rather than any baked-in default.
func TestDefaultEnvCarriesTheSecretKey(t *testing.T) {
	t.Setenv("UCODE_VERSION", "")
	if got := defaultEnv("s3cret"); !strings.Contains(got, "SECRET_KEY=s3cret") {
		t.Errorf("SECRET_KEY not written:\n%s", got)
	}
}
