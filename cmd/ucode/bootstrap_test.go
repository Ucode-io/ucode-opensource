package main

import (
	"errors"
	"testing"
)

func TestAlreadyBootstrapped(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "the installation limit, which is what a lost marker looks like",
			err:  errors.New(`creating the first company failed (HTTP 500): {"data":"rpc error: code = FailedPrecondition desc = this installation is set up for 1 company (MAX_COMPANIES=1). Raise MAX_COMPANIES if you need more."}`),
			want: true,
		},
		{
			// Images published before the limit became a setting.
			name: "the older wording",
			err:  errors.New("rpc error: code = Internal desc = only one company allowed"),
			want: true,
		},
		{
			name: "a service that is still starting is not a finished setup",
			err:  errors.New("cannot reach auth-service: dial tcp 127.0.0.1:9104: connect: connection refused"),
			want: false,
		},
		{
			name: "a genuine failure must not be reported as success",
			err:  errors.New(`creating the first company failed (HTTP 500): {"data":"invalid username"}`),
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := alreadyBootstrapped(tc.err); got != tc.want {
				t.Fatalf("alreadyBootstrapped(%q) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
