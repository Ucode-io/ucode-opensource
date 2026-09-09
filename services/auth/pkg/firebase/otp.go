package firebase

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
)

type VerifyPhoneRequest struct {
	SessionInfo string `json:"sessionInfo"`
	Code        string `json:"code"`
}

func VerifyPhoneCode(cfg config.BaseConfig, sessionInfo, code string) error {
	url := fmt.Sprintf("%v/v1/accounts:signInWithPhoneNumber?key=%s", cfg.FirebaseBaseUrl, cfg.FirebaseAPIKey)

	req := VerifyPhoneRequest{
		SessionInfo: sessionInfo,
		Code:        code,
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := errors.New("failed to verify")
		return err
	}

	return nil
}
