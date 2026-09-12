package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

/*
Демо-проект: небольшая таблица с данными, чтобы первый экран не был пустым.

Заводится через тот же публичный API, которым пользуется человек, а не записью
в базу напрямую. Так демо заодно проверяет, что путь «создать таблицу → создать
запись» действительно работает на этой установке — если он сломан, это видно
сразу при первом запуске, а не когда пользователь ткнёт первую кнопку.
*/

const demoTableSlug = "contacts"

type demoField struct {
	Label string `json:"label"`
	Slug  string `json:"slug"`
	Type  string `json:"type"`
}

// Поля подобраны так, чтобы показать разные типы: строка, почта, число и флаг.
var demoFields = []demoField{
	{Label: "Name", Slug: "name", Type: "SINGLE_LINE"},
	{Label: "Email", Slug: "email", Type: "EMAIL"},
	{Label: "Company", Slug: "company", Type: "SINGLE_LINE"},
	{Label: "Active", Slug: "active", Type: "SWITCH"},
}

// Данные заведомо ненастоящие: домен example.com зарезервирован стандартом
// именно под примеры, и никто не спутает их с чужими контактами.
var demoRows = []map[string]any{
	{"name": "Ada Lovelace", "email": "ada@example.com", "company": "Analytical Engines", "active": true},
	{"name": "Alan Turing", "email": "alan@example.com", "company": "Bletchley", "active": true},
	{"name": "Grace Hopper", "email": "grace@example.com", "company": "Naval Systems", "active": true},
	{"name": "Katherine Johnson", "email": "katherine@example.com", "company": "Flight Research", "active": false},
	{"name": "Barbara Liskov", "email": "barbara@example.com", "company": "Substitution Ltd", "active": true},
}

// newUUID returns a v4 identifier. The field endpoint expects the client to
// choose the id, the same way the admin panel does.
func newUUID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}

type apiClient struct {
	http        *http.Client
	token       string
	project     string
	environment string
}

func (c *apiClient) do(method, path string, body any) (map[string]any, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, gatewayURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("project-id", c.project)
	req.Header.Set("environment-id", c.environment)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s: HTTP %d: %s", method, path, resp.StatusCode, bytes.TrimSpace(payload))
	}

	var decoded map[string]any
	_ = json.Unmarshal(payload, &decoded)
	return decoded, nil
}

// seedDemo creates the demo table and its rows.
//
// Failure here is reported but never fatal: an empty workspace is a worse first
// impression than a missing demo, but it is still a working installation.
func seedDemo() error {
	client, err := newAPIClient()
	if err != nil {
		return err
	}

	if exists, err := client.tableExists(demoTableSlug); err != nil {
		return err
	} else if exists {
		return nil
	}

	if _, err := client.do("POST", "/v1/table", map[string]any{
		"label":        "Contacts",
		"slug":         demoTableSlug,
		"show_in_menu": true,
		"attributes":   map[string]any{},
	}); err != nil {
		return fmt.Errorf("creating the demo table: %w", err)
	}

	for _, field := range demoFields {
		if _, err := client.do("POST", "/v2/fields/"+demoTableSlug, map[string]any{
			"id":    newUUID(),
			"label": field.Label,
			"slug":  field.Slug,
			"type":  field.Type,
			/*
			 * Имя ключа врёт дважды: это не id и не поле, а слаг ТАБЛИЦЫ.
			 * Бэкенд ищет `WHERE slug = $1`, когда значение не uuid — так же
			 * это делает и админка (apps/admin: toCreateBody).
			 */
			"table_id":   demoTableSlug,
			"index":      "",
			"is_visible": true,
			"show_label": true,
			"required":   false,
			"unique":     false,
			"attributes": map[string]any{},
		}); err != nil {
			return fmt.Errorf("adding field %q: %w", field.Slug, err)
		}
	}

	for _, row := range demoRows {
		if _, err := client.do("POST", "/v2/items/"+demoTableSlug, map[string]any{"data": row}); err != nil {
			return fmt.Errorf("adding a demo record: %w", err)
		}
	}

	return nil
}

// tableExists keeps a second `ucode start` from stacking another demo on top of
// the first one.
func (c *apiClient) tableExists(slug string) (bool, error) {
	body, err := c.do("GET", "/v1/table?limit=200", nil)
	if err != nil {
		return false, err
	}

	data, _ := body["data"].(map[string]any)
	for _, key := range []string{"tables", "response"} {
		list, _ := data[key].([]any)
		for _, entry := range list {
			table, _ := entry.(map[string]any)
			if table["slug"] == slug {
				return true, nil
			}
		}
	}
	return false, nil
}

// newAPIClient signs in as the admin the bootstrap created and resolves the
// project and environment every later call needs.
func newAPIClient() (*apiClient, error) {
	httpClient := &http.Client{Timeout: 30 * time.Second}

	body, _ := json.Marshal(map[string]string{"username": adminLogin, "password": adminPassword})
	resp, err := httpClient.Post(authURL+"/v3/multicompany/default-login", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("signing in as the demo admin: %w", err)
	}
	defer resp.Body.Close()

	var login struct {
		Data struct {
			Response struct {
				Token struct {
					AccessToken string `json:"access_token"`
				} `json:"token"`
			} `json:"response"`
			ProjectData struct {
				ProjectID string `json:"project_id"`
			} `json:"project_data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&login); err != nil {
		return nil, err
	}

	token := login.Data.Response.Token.AccessToken
	project := login.Data.ProjectData.ProjectID
	if token == "" || project == "" {
		return nil, fmt.Errorf("the sign-in response carried no token or project")
	}

	client := &apiClient{http: httpClient, token: token, project: project}

	envelope, err := client.do("GET", "/v1/environment?project_id="+project+"&limit=1", nil)
	if err != nil {
		return nil, fmt.Errorf("reading the environment: %w", err)
	}
	data, _ := envelope["data"].(map[string]any)
	list, _ := data["environments"].([]any)
	if len(list) == 0 {
		return nil, fmt.Errorf("the project has no environment")
	}
	first, _ := list[0].(map[string]any)
	client.environment, _ = first["id"].(string)

	return client, nil
}
