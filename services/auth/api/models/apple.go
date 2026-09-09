package models

type AppleConfig struct {
	TeamId    string `json:"team_id" binding:"required"`
	ClientId  string `json:"client_id" binding:"required"`
	KeyId     string `json:"key_id" binding:"required"`
	SecretKey string `json:"secret_key" binding:"required"`
}

// Your 10-character Team ID
// TeamID := "XXXXXXXXXX"

// ClientID is the "Services ID" value that you get when navigating to your "sign in with Apple"-enabled service ID
// ClientID := "com.your.app"

// Find the 10-char Key ID value from the portal
// KeyID := "XXXXXXXXXX"

// The contents of the p8 file/key you downloaded when you made the key in the portal
// SecretKey := `-----BEGIN PRIVATE KEY-----
// YOUR_SECRET_PRIVATE_KEY
// -----END PRIVATE KEY-----`

//ClienSecret store depenedencies especially uses for create jwt client secrets
type ClientSecret struct {
	ISS, AUD, SUB, KID string
	IAT, EXP           int
}

type AppleLoginResponse struct {
	IDToken string `json:"id_token"`
}

//UserPaylod store AppleAppleLoginResponse parsed data
type AppleUserPayload struct {
	Email          string `json:"email"`
	EmailVerified  bool   `json:"email_verified"`
	IsPrivateEmail bool   `json:"is_private_email"`
	SUB            string `json:"sub"`
	Name           struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
	} `json:"name"`
}
