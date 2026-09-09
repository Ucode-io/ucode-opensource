package main

import (
	"bufio"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The stack definition travels inside the binary so `ucode start` works
// without cloning the repository. deploy/ holds the canonical copies; `make
// cli-assets` refreshes these and CI fails if they drift.
//
//go:embed assets/docker-compose.yml assets/postgres-init.sql
var assets embed.FS

const (
	composeProject = "ucode"
	adminLogin     = "admin@ucode.local"
	adminPassword  = "UcodeAdmin1!"
	adminPhone     = "+19995550100"

	gatewayURL = "http://127.0.0.1:8000"
	authURL    = "http://127.0.0.1:9104"
	adminURL   = "http://127.0.0.1:3000"
)

// stack owns the directory the CLI writes to and the docker compose calls.
type stack struct {
	dir string
}

func newStack() (*stack, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot find your home directory: %w", err)
	}
	return &stack{dir: filepath.Join(home, ".ucode")}, nil
}

func (s *stack) composePath() string { return filepath.Join(s.dir, "docker-compose.yml") }
func (s *stack) envPath() string     { return filepath.Join(s.dir, ".env") }
func (s *stack) markerPath() string  { return filepath.Join(s.dir, ".bootstrapped") }

// materialise writes the compose file and, on the very first run, an .env with
// a freshly generated signing key. An existing .env is left alone: it may carry
// edits, and regenerating SECRET_KEY would invalidate every issued token.
func (s *stack) materialise() error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("cannot create %s: %w", s.dir, err)
	}

	for _, name := range []string{"docker-compose.yml", "postgres-init.sql"} {
		data, err := assets.ReadFile("assets/" + name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(s.dir, name), data, 0o644); err != nil {
			return fmt.Errorf("cannot write %s: %w", name, err)
		}
	}

	if _, err := os.Stat(s.envPath()); err == nil {
		return nil
	}

	key, err := randomKey()
	if err != nil {
		return err
	}
	return os.WriteFile(s.envPath(), []byte(defaultEnv(key)), 0o600)
}

func randomKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("cannot generate a signing key: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// compose runs docker compose against the CLI's own directory, so it never
// picks up a compose file from wherever the user happens to stand.
func (s *stack) compose(args ...string) *exec.Cmd {
	full := append([]string{
		"compose",
		"--project-name", composeProject,
		"--file", s.composePath(),
		"--env-file", s.envPath(),
	}, args...)
	cmd := exec.Command("docker", full...)
	cmd.Dir = s.dir
	return cmd
}

func (s *stack) run(args ...string) error {
	cmd := s.compose(args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *stack) output(args ...string) (string, error) {
	out, err := s.compose(args...).Output()
	return string(out), err
}

func (s *stack) bootstrapped() bool {
	_, err := os.Stat(s.markerPath())
	return err == nil
}

func (s *stack) markBootstrapped() error {
	return os.WriteFile(s.markerPath(), []byte("first run completed\n"), 0o644)
}

// envValue reads one key out of the generated .env, so callers do not have to
// parse the file themselves.
func (s *stack) envValue(key string) string {
	f, err := os.Open(s.envPath())
	if err != nil {
		return ""
	}
	defer f.Close()

	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(k) == key {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
