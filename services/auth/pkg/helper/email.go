package helper

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	net_http "net/http"
	"net/smtp"
	"strings"

	"github.com/pkg/errors"
)

const (
	// server we are authorized to send email through
	host     = "smtp.gmail.com"
	hostPort = ":587"
)

type SendMessageToEmailRequest struct {
	Subject       string
	To            string
	UserId        string
	Email         string
	Password      string
	Username      string
	TempPassword  string
	EnvironmentId string
	ProjectId     string
	ClientTypeId  string
}

func GetGoogleUserInfo(accessToken string) (map[string]any, error) {
	resp, err := net_http.Get("https://www.googleapis.com/oauth2/v3/userinfo?access_token=" + accessToken)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	userInfo := make(map[string]any)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, &userInfo)
	if err != nil {
		return nil, err
	}

	return userInfo, nil
}

func DecodeGoogleIDToken(idToken string) (map[string]any, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT token format")
	}

	payload := parts[1]

	if len(payload)%4 != 0 {
		payload += strings.Repeat("=", 4-len(payload)%4)
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return nil, err
	}

	userInfo := make(map[string]any)
	err = json.Unmarshal(decoded, &userInfo)
	if err != nil {
		return nil, err
	}

	return userInfo, nil
}

func SendCodeToEmail(subject, to, code string, email string, password string) error {
	message := `
		Your verification code is: ` + code

	auth := smtp.PlainAuth("", email, password, host)

	msg := "To: \"" + to + "\" <" + to + ">\n" +
		"From: \"" + email + "\" <" + email + ">\n" +
		"Subject: " + subject + "\n" +
		message + "\n"

	if err := smtp.SendMail(host+hostPort, auth, email, []string{to}, []byte(msg)); err != nil {
		return errors.Wrap(err, "error while sending message to email")
	}

	return nil
}

type loginAuth struct {
	username, password string
}

func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte{}, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(a.username), nil
		case "Password:":
			return []byte(a.password), nil
		default:
			return nil, errors.New("Unkown fromServer")
		}
	}
	return nil, nil
}

func SendCodeToEnvironmentEmail(subject, to, code string, email string, password string) error {

	smtpServer := "outlook.office365.com"
	smtpPort := 587

	message := `
		Your verification code is ` + code

	msg := "To: \"" + to + "\" <" + to + ">\n" +
		"From: \"" + email + "\" <" + email + ">\n" +
		"Subject: " + subject + "\n" +
		message + "\n"

	auth := LoginAuth(email, password)

	err := smtp.SendMail(fmt.Sprintf("%s:%d", smtpServer, smtpPort), auth, email, []string{to}, []byte(msg))
	if err != nil {
		return errors.Wrap(err, "error while sending message to environment email")
	}

	return nil
}
