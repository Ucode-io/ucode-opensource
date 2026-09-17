package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// waitReady blocks until auth-service answers HTTP at all. It is a courtesy
// so the common case reports progress quickly; real readiness is established
// by bootstrap's retries.
func waitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(authURL + "/ping")
		if err == nil {
			_ = resp.Body.Close()
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("auth-service did not answer within %s.\nRun \"ucode logs auth\" to see why", timeout)
}

type registerRequest struct {
	Name     string       `json:"name"`
	UserInfo registerUser `json:"user_info"`
}

type registerUser struct {
	Login    string `json:"login"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
}

// bootstrap creates the first company through the same endpoint the sign-up
// form uses. That one call also creates the project, its environment, the
// admin, the project's own database, its migrations and the default roles.
//
// It retries, because readiness cannot be observed from outside: Docker binds
// a published port as soon as the container starts, so a successful TCP
// connect says nothing about whether the process inside is listening yet. The
// call itself is the only honest signal.
// errAlreadyBootstrapped means the platform is already set up — the marker
// file was lost or never written, but the company exists.
var errAlreadyBootstrapped = errors.New("already bootstrapped")

func bootstrap(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		err := registerCompany()
		if err == nil {
			return nil
		}
		if alreadyBootstrapped(err) {
			return errAlreadyBootstrapped
		}
		if !retryable(err) {
			return err
		}
		lastErr = err
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("the services never became ready within %s.\nLast error:\n%v", timeout, lastErr)
}

// alreadyBootstrapped distinguishes "this installation is set up" from a real
// failure. company-service refuses a second company once MAX_COMPANIES is
// reached, and a re-run that lost its marker file looks exactly like that.
//
// The error arrives as a string: auth flattens the gRPC status into its HTTP
// response, so the code is gone by the time it reaches here. MAX_COMPANIES is
// the stable part of that message and is what to match on; the wording around
// it can change. The old phrasing is still recognised so that a CLI from this
// version can talk to images published before the limit became a setting.
func alreadyBootstrapped(err error) bool {
	text := err.Error()
	return strings.Contains(text, "MAX_COMPANIES") ||
		strings.Contains(text, "only one company allowed")
}

// retryable reports whether an attempt failed because something was still
// starting, as opposed to a refusal the platform will keep repeating.
func retryable(err error) bool {
	text := err.Error()
	for _, sign := range []string{
		"connection refused",
		"Unavailable",
		"EOF",
		"connection reset",
		"no such host",
	} {
		if strings.Contains(text, sign) {
			return true
		}
	}
	return false
}

// verifySetup confirms the installation actually works, by doing what the
// first user does: signing in as the admin and resolving a project.
//
// This is not belt and braces. Creating the first company writes the company
// row before it registers the project's builder resource on object-builder,
// and those are not one transaction. If object-builder is not listening yet,
// the company survives and the resource does not — and every later attempt is
// refused with "only one company allowed", which is indistinguishable from a
// finished installation. That combination used to be reported as success, and
// the user met it as a login that failed with "user project not found".
//
// The compose file now makes company wait for object-builder to answer, which
// should stop it happening. This catches it if it happens anyway, because a
// wrong "ucode is running" costs more than a slow failure.
func verifySetup(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error

	for {
		if _, err := newAPIClient(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf(`the platform was created but the admin cannot sign in:
%v

Part of the first-run setup did not finish. Start over with:

    ucode reset
    ucode start

If it happens again, "ucode logs company" and "ucode logs object-builder"
will show which step failed`, lastErr)
}

func registerCompany() error {
	body, err := json.Marshal(registerRequest{
		Name: "ucode",
		UserInfo: registerUser{
			Login:    adminLogin,
			Password: adminPassword,
			Email:    adminLogin,
			Phone:    adminPhone,
		},
	})
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Post(authURL+"/company", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("cannot reach auth-service: %w", err)
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("creating the first company failed (HTTP %d): %s",
			resp.StatusCode, bytes.TrimSpace(payload))
	}
	return nil
}

// openBrowser is best effort: a headless machine or an unusual desktop is not
// a reason to fail the command.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	_ = cmd.Start()
}
