package helper

import (
	"context"
	"fmt"
	"github.com/Ucode-io/ucode-opensource/services/auth/api/models"

	"github.com/Timothylock/go-signin-with-apple/apple"
)

func GetAppleUserInfo(code string, c *models.AppleConfig) (*models.AppleUserPayload, error) {

	// Generate the client secret used to authenticate with Apple's validation servers
	secretKey := fmt.Sprintf(`-----BEGIN PRIVATE KEY-----
	%s
-----END PRIVATE KEY-----`, c.SecretKey)
	secret, err := apple.GenerateClientSecret(secretKey, c.TeamId, c.ClientId, c.KeyId)
	if err != nil {

		return nil, err
	}

	// Generate a new validation client
	client := apple.New()

	vReq := apple.AppValidationTokenRequest{
		ClientID:     c.ClientId,
		ClientSecret: secret,
		Code:         code,
	}

	var resp apple.ValidationResponse

	// Do the verification
	err = client.VerifyAppToken(context.Background(), vReq, &resp)
	if err != nil {

		return nil, err
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("apple returned an error: %s - %s", resp.Error, resp.ErrorDescription)
	}

	// Get the email
	claim, err := apple.GetClaims(resp.IDToken)
	if err != nil {

		return nil, err
	}

	email := (*claim)["email"].(string)

	return &models.AppleUserPayload{
		Email: email,
	}, nil
}
