package security

import (
	"errors"
	"strings"
	"time"

	"maps"

	"github.com/golang-jwt/jwt/v5"
)

// GenerateJWT ...
func GenerateJWT(m map[string]any, tokenExpireTime time.Duration, tokenSecretKey string) (tokenString string, err error) {
	var token *jwt.Token = jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)

	maps.Copy(claims, m)

	claims["iat"] = time.Now().Unix()
	claims["exp"] = time.Now().Add(tokenExpireTime).Unix()

	tokenString, err = token.SignedString([]byte(tokenSecretKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ExtractClaims extracts claims from given token
func ExtractClaims(tokenString string, tokenSecretKey string) (jwt.MapClaims, error) {
	var (
		token *jwt.Token
		err   error
	)

	token, err = jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		return []byte(tokenSecretKey), nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !(ok && token.Valid) {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// ExtractToken checks and returns token part of input string
func ExtractToken(bearer string) (token string, err error) {
	strArr := strings.Split(bearer, " ")
	if len(strArr) == 2 {
		return strArr[1], nil
	}
	return token, errors.New("wrong token format")
}

type TokenInfo struct {
	ID             string
	Tables         []Table
	LoginTableSlug string
	UserId         string
	RoleID         string
	ProjectID      string
	ClientID       string
	ClientTypeId   string
	UserIdAuth     string
}

type Table struct {
	TableSlug string
	ObjectID  string
}

func ParseClaims(token string, secretKey string) (result TokenInfo, err error) {
	var ok bool
	var claims jwt.MapClaims

	claims, err = ExtractClaims(token, secretKey)
	if err != nil {
		return result, err
	}
	result.ID, ok = claims["id"].(string)
	result.RoleID = claims["role_id"].(string)
	result.UserId = claims["user_id"].(string)
	result.ClientTypeId = claims["client_type_id"].(string)
	result.UserIdAuth = claims["user_id_auth"].(string)
	if !ok {
		err = errors.New("cannot parse 'id' field")
		return result, err
	}
	if claims["tables"] != nil {
		for _, item := range claims["tables"].([]any) {
			var table Table
			if item != nil {
				if item.(map[string]any)["object_id"] != nil && item.(map[string]any)["table_slug"] != nil {
					table.ObjectID = item.(map[string]any)["object_id"].(string)
					table.TableSlug = item.(map[string]any)["table_slug"].(string)
					result.Tables = append(result.Tables, table)
				}
			}
		}
	}
	loginTableSlug, ok := claims["login_table_slug"]
	if ok {
		result.LoginTableSlug = loginTableSlug.(string)
	}

	return
}
