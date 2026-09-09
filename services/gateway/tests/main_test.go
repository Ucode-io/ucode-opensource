package tests

import (
	"os"
	"testing"
	"time"

	"github.com/manveru/faker"
	"github.com/stretchr/testify/assert"
	sdk "github.com/ucode-io/ucode_sdk"
)

const (
	requestTimeout = time.Second * 5

	appId       = "P-NE2ZNy8XN4wmQVSSyxnpU6HyrzO61Gk4"
	BaseUrl     = "https://api.admin.u-code.io"
	BaseAuthUrl = "https://api.auth.u-code.io"

	ProjectId     = "43cd093f-0cb9-48d5-81a5-cb2b55cca131"
	EnvironmentId = "1ef390ab-a388-46c1-8702-fc428fbf6d47"

	Login    = "Fazliddin!1Test12"
	Password = "Fazliddin!1Test12"
)

var (
	UcodeApi = sdk.New(
		&sdk.Config{
			BaseURL:        BaseUrl,
			RequestTimeout: requestTimeout,
			AppId:          appId,
			ProjectId:      ProjectId,
			BaseAuthUrl:    BaseAuthUrl,
		},
	)

	AccessToken string

	UserId string

	fakeData *faker.Faker
)

func TestMain(m *testing.M) {
	fakeData, _ = faker.New("en")

	os.Exit(m.Run())
}

func TestE2EFlow(t *testing.T) {

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
