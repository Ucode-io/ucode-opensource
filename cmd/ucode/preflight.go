package main

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
)

// portsInUse lists the host ports the stack publishes, with the service that
// wants each one so a collision report says something useful.
var publishedPorts = []struct {
	port int
	what string
}{
	{8000, "gateway"},
	{9103, "auth (gRPC)"},
	{9104, "auth (HTTP)"},
	{8092, "company"},
	{7107, "object-builder"},
	{5433, "postgres"},
	{6380, "redis"},
	{9010, "minio"},
	{9011, "minio console"},
}

// preflight refuses to start rather than letting docker fail with a raw error
// halfway through bringing containers up.
//
// The port check is skipped when our own stack already holds them: `ucode
// start` on a running installation is a reasonable thing to type, and it used
// to fail telling you to stop whatever was using the ports — which was ucode.
func preflight(stackRunning bool) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker is not installed.\n" +
			"ucode runs the platform in containers, so Docker is the one thing\n" +
			"you have to install yourself: https://docs.docker.com/get-docker/")
	}

	if err := exec.Command("docker", "info").Run(); err != nil {
		return fmt.Errorf("docker is installed but not running.\nStart Docker Desktop (or the docker daemon) and try again")
	}

	if stackRunning {
		return nil
	}

	var taken []string
	for _, p := range publishedPorts {
		if !portFree(p.port) {
			taken = append(taken, fmt.Sprintf("  %d — needed by %s", p.port, p.what))
		}
	}
	if len(taken) > 0 {
		return fmt.Errorf("these ports are already in use:\n%s\n\nStop whatever holds them, or change the UCODE_*_PORT values in ~/.ucode/.env",
			strings.Join(taken, "\n"))
	}

	return nil
}

func portFree(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err != nil {
		return true
	}
	_ = conn.Close()
	return false
}
