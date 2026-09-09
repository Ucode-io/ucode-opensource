package tests

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

func TestAuthItemsFlow(t *testing.T) {

	var (
		newPassword  = fakeData.UserName() + "!1"
		userIdAuth   string
		roleId       string
		clientTypeId string
	)

	t.Run("1 - Get client type ids", func(t *testing.T) {
		response, res, err := UcodeApi.Items("role").GetList().Page(1).Limit(1).Exec()
		if err != nil {
			respByte, _ := json.Marshal(res)
			fmt.Println("ERROR IN GETLIST ROLE", string(respByte))
			t.Error(err)
		}

		if len(response.Data.Data.Response) == 0 {
			t.Error("role ids is empty")
		}

		var ok bool

		if roleId, ok = response.Data.Data.Response[0]["guid"].(string); !ok {
			t.Error("role id is empty")
		}

		if clientTypeId, ok = response.Data.Data.Response[0]["client_type_id"].(string); !ok {
			t.Error("client_type_id is empty")
		}
	})

	t.Run("2 - Register user", func(t *testing.T) {

		var (
			body = map[string]any{
				"guid":           uuid.NewString(),
				"environment_id": EnvironmentId,
				"project_id":     ProjectId,
				"type":           "login",
				"role_id":        roleId,
				"client_type_id": clientTypeId,
				"login":          Login,
				"password":       Password,
			}

			header = map[string]string{
				"project-id":     ProjectId,
				"Environment-Id": EnvironmentId,
			}
		)

		registerData, resp, err := UcodeApi.Auth().Register(map[string]any{"data": body}).Headers(header).Exec()
		if err != nil {
			respByte, _ := json.Marshal(resp)
			fmt.Println("ERROR IN REGISTER", string(respByte))
			t.Error(err)
		}

		AccessToken = registerData.Data.Token.AccessToken
		userIdAuth = registerData.Data.UserIdAuth
		UserId = registerData.Data.UserId
	})

	t.Run("3 - Reset password", func(t *testing.T) {

		var (
			body = map[string]any{
				"user_id":  userIdAuth,
				"password": newPassword,
			}

			header = map[string]string{
				"project-id":     ProjectId,
				"Environment-Id": EnvironmentId,
			}
		)

		response, err := UcodeApi.Auth().ResetPassword(body).Headers(header).Exec()
		if err != nil {
			fmt.Println("ERROR IN Reset password", response.Status, response.Data, response.Error)
			t.Error(err)
		}
	})

	t.Run("4 - Login user", func(t *testing.T) {

		var (
			body = map[string]any{
				"client_type":    clientTypeId,
				"environment_id": EnvironmentId,
				"project_id":     ProjectId,
				"username":       Login,
				"password":       newPassword,
			}

			header = map[string]string{
				"project_id":     ProjectId,
				"environment_id": EnvironmentId,
			}
		)

		registerData, resp, err := UcodeApi.Auth().Login(body).Headers(header).Exec()
		if err != nil {
			respByte, _ := json.Marshal(resp)
			fmt.Println("ERROR IN LOGIN USER", string(respByte))
			t.Error(err)
		}

		AccessToken = registerData.Data.Token.AccessToken
	})

}
