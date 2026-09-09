//go:build integration

// Package tests drives the platform end to end through the public SDK.
//
// It needs a running ucode stack and is configured entirely from the
// environment — there are deliberately no defaults, so a stray `go test` can
// never reach someone's real installation:
//
//	UCODE_TEST_BASE_URL        e.g. http://localhost:8000
//	UCODE_TEST_AUTH_URL        e.g. http://localhost:9104
//	UCODE_TEST_APP_ID
//	UCODE_TEST_PROJECT_ID
//	UCODE_TEST_ENVIRONMENT_ID
//	UCODE_TEST_LOGIN
//	UCODE_TEST_PASSWORD
//
// When any of them is unset the suite skips instead of failing, so it stays out
// of the way until a local stack exists.
package tests

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/manveru/faker"
	"github.com/stretchr/testify/assert"
	sdk "github.com/ucode-io/ucode_sdk"
)

const requestTimeout = time.Second * 5

var (
	appId         string
	BaseUrl       string
	BaseAuthUrl   string
	ProjectId     string
	EnvironmentId string
	Login         string
	Password      string

	UcodeApi sdk.UcodeApis

	AccessToken string
	UserId      string

	fakeData *faker.Faker

	// missing lists the environment variables the suite needs but did not get.
	missing []string
)

func env(name string, target *string) {
	value := os.Getenv(name)
	if strings.TrimSpace(value) == "" {
		missing = append(missing, name)
		return
	}
	*target = value
}

func TestMain(m *testing.M) {
	env("UCODE_TEST_BASE_URL", &BaseUrl)
	env("UCODE_TEST_AUTH_URL", &BaseAuthUrl)
	env("UCODE_TEST_APP_ID", &appId)
	env("UCODE_TEST_PROJECT_ID", &ProjectId)
	env("UCODE_TEST_ENVIRONMENT_ID", &EnvironmentId)
	env("UCODE_TEST_LOGIN", &Login)
	env("UCODE_TEST_PASSWORD", &Password)

	if len(missing) == 0 {
		UcodeApi = sdk.New(&sdk.Config{
			BaseURL:        BaseUrl,
			RequestTimeout: requestTimeout,
			AppId:          appId,
			ProjectId:      ProjectId,
			BaseAuthUrl:    BaseAuthUrl,
		})
	}

	fakeData, _ = faker.New("en")

	os.Exit(m.Run())
}

// requireStack skips the calling test unless the suite is fully configured.
func requireStack(t *testing.T) {
	t.Helper()
	if len(missing) > 0 {
		t.Skipf("no ucode stack configured; set %s", strings.Join(missing, ", "))
	}
}

func TestE2EFlow(t *testing.T) {
	requireStack(t)

	t.Run("AuthFlow", func(t *testing.T) {
		TestAuthItemsFlow(t)
	})

	AccessToken = "Bearer " + AccessToken

	t.Run("ItemsFlow", func(t *testing.T) {
		TestItemsFlow(t)
	})

	t.Run("Delete user", func(t *testing.T) {
		_, err := UcodeApi.Items("teset_login").Delete().Single(UserId).Exec()
		assert.NoError(t, err)
	})
}
