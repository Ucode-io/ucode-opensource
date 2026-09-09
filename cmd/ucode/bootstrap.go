package main

import (
	"bytes"
	"encoding/json"
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
func bootstrap(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		err := registerCompany()
		if err == nil {
			return nil
		}
		if !retryable(err) {
			return err
		}
		lastErr = err
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("the services never became ready within %s.\nLast error:\n%v", timeout, lastErr)
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
		return fmt.Errorf("creating the first company failed (HTTP %d):\n%s\n\n"+
			"If this says \"only one company allowed\", an earlier attempt left a half-created\n"+
			"company behind. Run \"ucode reset\" and start again",
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
