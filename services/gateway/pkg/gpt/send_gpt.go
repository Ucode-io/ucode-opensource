package gpt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
)

func SendReqToGPT(req []models.Message) ([]models.ToolCall, error) {
	cfg := config.Load()

	requestBody := models.OpenAIRequest{
		Model:        "gpt-4o",
		Messages:     req,
		Functions:    GetDefaultFunctions(),
		FunctionCall: "auto",
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	reqBody, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	reqBody.Header.Set("Content-Type", "application/json")
	reqBody.Header.Set("Authorization", "Bearer "+cfg.OpenAIApiKey)

	client := &http.Client{}
	resp, err := client.Do(reqBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response models.OpenAIResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	if response.Error.Message != "" {
		return nil, fmt.Errorf("not full information given")
	}

	return response.Choices[0].Message.ToolCalls, nil
}

func GetDefaultFunctions() []models.Tool {
	return []models.Tool{
		{
			Type: "function",
			Function: models.FunctionDescription{
				Name:        "create_menu",
				Description: "Create menu with given name",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type":        "string",
							"description": "The name of the menu",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: models.FunctionDescription{
				Name:        "create_table",
				Description: "Create table with given name",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type":        "string",
							"description": "The name of the table",
						},
						"table_slug": map[string]any{
							"type":        "string",
							"description": "A slug generated from the name by translating it to English and converting it to lowercase. Spaces and special characters should be replaced with underscores.",
						},
						"menu": map[string]any{
							"type":        "string",
							"description": "The name of the menu",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: models.FunctionDescription{
				Name:        "update_table",
				Description: "Update table with given name",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"old_name": map[string]any{
							"type":        "string",
							"description": "The current name of the table that needs to be updated",
						},
						"new_name": map[string]any{
							"type":        "string",
							"description": "The new name to be assigned to the table",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: models.FunctionDescription{
				Name:        "delete_menu",
				Description: "Delete menu with given name",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type":        "string",
							"description": "The name of the menu",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: models.FunctionDescription{
				Name:        "update_menu",
				Description: "Update menu with given name",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"old_name": map[string]any{
							"type":        "string",
							"description": "The old name of the menu",
						},
						"new_name": map[string]any{
							"type":        "string",
							"description": "The new name of the menu",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: models.FunctionDescription{
				Name:        "delete_table",
				Description: "Delete table with given name",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type":        "string",
							"description": "The name of the table",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: models.FunctionDescription{
				Name:        "create_field",
				Description: "Create a field with the given name or create many fields",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"label": map[string]any{
							"type":        "string",
							"description": "The label of the field",
						},
						"slug": map[string]any{
							"type":        "string",
							"description": "The slug of the field, derived from the label",
							"transform":   "translate to English, lowercase, replace spaces with underscores",
						},
						"type": map[string]any{
							"type":        "string",
							"description": "The type of the field",
							"enum": []string{"EMAIL", "PHOTO", "TIME", "MULTISELECT", "RANDOM_NUMBERS", "FILE",
								"INCREMENT_NUMBER", "PHONE", "DATE_TIME", "FLOAT_NOLIMIT",
								"MULTI_IMAGE", "DATE_TIME_WITHOUT_TIME_ZONE", "MULTI_LINE", "CHECKBOX",
								"SWITCH", "FORMULA_FRONTEND", "NUMBER", "FLOAT", "FORMULA",
								"SINGLE_LINE", "PASSWORD", "CODABAR", "INTERNATIONAL_PHONE",
								"UUID", "INCREMENT_ID", "DATE", "MAP",
							},
							"inference": "based on label and description default return SINGLE_LINE",
						},
						"table": map[string]any{
							"type":        "string",
							"description": "The name of the table",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: models.FunctionDescription{
				Name:        "update_field",
				Description: "Update field with given parameters",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"old_label": map[string]any{
							"type":        "string",
							"description": "The old label of the field",
						},
						"new_label": map[string]any{
							"type":        "string",
							"description": "The new label of the field",
						},
						"new_type": map[string]any{
							"type":        "string",
							"description": "The new type of the field",
							"enum": []string{"EMAIL", "PHOTO", "TIME", "MULTISELECT", "RANDOM_NUMBERS", "FILE",
								"INCREMENT_NUMBER", "PHONE", "DATE_TIME", "FLOAT_NOLIMIT",
								"MULTI_IMAGE", "DATE_TIME_WITHOUT_TIME_ZONE", "MULTI_LINE", "CHECKBOX",
								"SWITCH", "FORMULA_FRONTEND", "NUMBER", "FLOAT", "FORMULA",
								"SINGLE_LINE", "PASSWORD", "CODABAR", "INTERNATIONAL_PHONE",
								"UUID", "INCREMENT_ID", "DATE", "MAP",
							},
						},
						"table": map[string]any{
							"type":        "string",
							"description": "The table the field belongs to",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: models.FunctionDescription{
				Name:        "delete_field",
				Description: "Delete field with given parameters",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"label": map[string]any{
							"type":        "string",
							"description": "The label of the field",
						},
						"table": map[string]any{
							"type":        "string",
							"description": "The table the field belongs to",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: models.FunctionDescription{
				Name:        "create_relation",
				Description: "Create relation with given parameters",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"table_from": map[string]any{
							"type":        "string",
							"description": "The label of the table from",
						},
						"table_to": map[string]any{
							"type":        "string",
							"description": "The label of the table to",
						},
						"relation_type": map[string]any{
							"type":        "string",
							"description": "The relation type default Many2One. If table_to and table_from is equal return Recursive",
							"enum":        []string{"Many2One", "Many2Many", "Recursive"},
						},
						"view_field": map[string]any{
							"type":        "array",
							"description": "The label of view field",
							"items": map[string]any{
								"type": "string",
							},
						},
						"view_type": map[string]any{
							"type":        "string",
							"description": "When relation_type is Many2Many return INPUT or TABLE looking for description. Default INPUT",
							"enum":        []string{"INPUT", "TABLE"},
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: models.FunctionDescription{
				Name:        "delete_relation",
				Description: "Delete relation with given parameters",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"table_from": map[string]any{
							"type":        "string",
							"description": "The label of the table from",
						},
						"table_to": map[string]any{
							"type":        "string",
							"description": "The label of the table to",
						},
						"relation_type": map[string]any{
							"type":        "string",
							"description": "The type of relation",
							"enum":        []string{"Many2One", "Many2Many", "Recursive"},
							"default":     "",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: models.FunctionDescription{
				Name:        "create_row",
				Description: "Create row or item with given arguments",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"table": map[string]any{
							"type":        "string",
							"description": "The label of the table",
						},
						"arguments": map[string]any{
							"type":        "array",
							"description": "The argument to create row or item, An array of arguments used to create the row or item. Date strings will be dynamically converted to RFC3339 format.",
							"items": map[string]any{
								"type": "string",
							},
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: models.FunctionDescription{
				Name:        "generate_row",
				Description: "Create row or item with given arguments",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"table": map[string]any{
							"type":        "string",
							"description": "The label of the table",
						},
						"count": map[string]any{
							"type":        "string",
							"description": "The count of rows which should generate",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: models.FunctionDescription{
				Name:        "generate_values",
				Description: "Generate values for given columns",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"arguments": map[string]any{
							"type":        "array",
							"description": "Array of object which generated",
							"items": map[string]any{
								"type":        "object",
								"description": "Object which generated",
								"properties": map[string]any{
									"key": map[string]any{
										"type":        "string",
										"description": "this column given in promt",
									},
									"value": map[string]any{
										"type":        "string",
										"description": "generate by yourself value based on key or column",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: models.FunctionDescription{
				Name:        "update_item",
				Description: "Update item for given table",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"old_data": map[string]any{
							"type":        "string",
							"description": "Old data of object",
						},
						"new_data": map[string]any{
							"type":        "string",
							"description": "New data of object",
						},
						"table": map[string]any{
							"type":        "string",
							"description": "The label of the table",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: models.FunctionDescription{
				Name:        "delete_item",
				Description: "Delete item for given table",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"old_data": map[string]any{
							"type":        "string",
							"description": "Old data of object",
						},
						"table": map[string]any{
							"type":        "string",
							"description": "The label of the table",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: models.FunctionDescription{
				Name:        "login_table",
				Description: "Change to Login table with given Table label",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"table": map[string]any{
							"type":        "string",
							"description": "The label of the table",
						},
						"login": map[string]any{
							"type":        "string",
							"description": "The login label of login table",
						},
						"password": map[string]any{
							"type":        "string",
							"description": "The password label of login table",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: models.FunctionDescription{
				Name:        "create_ofs",
				Description: "Create a function and generate code based on provided specifications",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"table": map[string]any{
							"type":        "string",
							"description": "The name of the table to be created or manipulated",
						},
						"prompt": map[string]any{
							"type":        "string",
							"description": "A detailed description of what the function should do",
						},
						"function_name": map[string]any{
							"type":        "string",
							"description": "A generated name for the function based on the prompt, with spaces replaced by dashes, write simple function name not long",
						},
						"function_type": map[string]any{
							"type":        "string",
							"description": "Function type",
							"enum":        []string{"CREATE", "UPDATE", "DELETE"},
						},
						"request_type": map[string]any{
							"type":        "string",
							"description": "Request type",
							"enum":        []string{"after", "before"},
						},
					},
				},
			},
		},
	}
}

func GetOfsCode(req models.Message) (string, error) {

	cfg := config.Load()

	defMessages, err := GetDefaultMsssages()
	if err != nil {
		return "", err
	}

	defMessages = append(defMessages, req)

	requestBody := models.OpenAIRequestV2{
		Model:    "gpt-4o",
		Messages: defMessages,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	reqBody, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	reqBody.Header.Set("Content-Type", "application/json")
	reqBody.Header.Set("Authorization", "Bearer "+cfg.OpenAIApiKey)

	client := &http.Client{}
	resp, err := client.Do(reqBody)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var response models.OpenAIResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", err
	}

	if response.Error.Message != "" {
		return "", fmt.Errorf("not full information given")
	}

	return response.Choices[0].Message.Content, nil
}

func GetDefaultMsssages() ([]models.Message, error) {

	code, err := GetTemplateCode()
	if err != nil {
		return nil, err
	}

	return []models.Message{
		{
			Role:    "system",
			Content: "You are a helpful assistant that writes OpenFaaS functions in Golang.",
		},
		{
			Role:    "system",
			Content: fmt.Sprintf("There is a template for you to use for coding: ```%s```", code),
		},
		{
			Role:    "system",
			Content: `If you want to convert variables, use package like "github.com/spf13/cast"  (cast.ToString, cast.ToInt, etc). DONT USE variable.(string)`,
		},
		{
			Role:    "system",
			Content: `Always use "github.com/google/uuid" to generate guid in create`,
		},
		{
			Role: "system",
			Content: `If the prompt indicates that you need to get data from the request, then:
				For new data, access reqBody.Data.ObjectData.
				For old data, access reqBody.Data.ObjectDataBeforeUpdate.
			`,
		},
		{
			Role:    "system",
			Content: "You should write your code within the Handle() function and you must RETURN ONLY Handle() function without any comments.",
		},
		{
			Role:    "system",
			Content: "Return only function Handle() without package, import, and constants",
		},
	}, nil
}

func GetTemplateCode() (string, error) {

	curDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	if err := os.Chdir(curDir + "/pkg/gpt/"); err != nil {
		return "", err
	}

	data, err := os.ReadFile("ofs.txt")
	if err != nil {
		return "", err
	}

	return string(data), nil
}
