package v2

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/gpt"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

// SendToGpt godoc
// @Security ApiKeyAuth
// @ID send_to_gpt
// @Router /v2/send-to-gpt [POST]
// @Summary Send To Gpt
// @Description Send To Gpt
// @Tags GPT
// @Accept json
// @Produce json
// @Param object body models.SendToGptRequest true "SendToGptRequestBody"
// @Success 201 {object} status_http.Response{data=string} "Success"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) SendToGpt(c *gin.Context) {
	var (
		reqBody      models.SendToGptRequest
		respMessages = []models.Message{}
		messages     = []string{}
	)

	err := c.ShouldBindJSON(&reqBody)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		err = errors.New("error getting environment id | not valid")
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	userId, _ := c.Get("user_id")

	respMessages = append(respMessages, models.Message{
		Role:    "user",
		Content: reqBody.Promt,
	})

	toolCalls, err := gpt.SendReqToGPT(respMessages)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(
		c.Request.Context(),
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	for _, toolCall := range toolCalls {
		var (
			functionCall = toolCall.Function
			functionName = functionCall.Name
			arguments    map[string]any
			logReq       []*models.CreateVersionHistoryRequest
			msg          string
		)

		err = json.Unmarshal([]byte(functionCall.Arguments), &arguments)
		if err != nil {
			continue
		}

		switch functionName {
		case "create_menu":
			_, err = gpt.CreateMenu(&models.CreateMenuAI{
				Label:    cast.ToString(arguments["name"]),
				UserId:   userId.(string),
				Resource: resource,
				Service:  services,
			})

			if err == nil {
				msg = "Menu created Successfully"
			} else {
				msg = fmt.Sprintf("Error while create menu: %s", err.Error())
			}

		case "delete_menu", "delete_table":
			logReq, err = gpt.DeleteMenu(&models.DeleteMenuAI{
				Label:    cast.ToString(arguments["name"]),
				UserId:   cast.ToString(userId),
				Resource: resource,
				Service:  services,
			})

			if err == nil {
				if functionName == "delete_menu" {
					msg = "Menu deleted Successfully"
				} else {
					msg = "Table deleted Successfully"
				}
			} else {
				if functionName == "delete_menu" {
					msg = fmt.Sprintf("Error while delete menu: %s", err.Error())
				} else {
					msg = fmt.Sprintf("Error while delete table: %s", err.Error())
				}
			}
		case "update_menu":
			logReq, err = gpt.UpdateMenu(&models.UpdateMenuAI{
				OldLabel: cast.ToString(arguments["old_name"]),
				NewLabel: cast.ToString(arguments["new_name"]),
				UserId:   cast.ToString(userId),
				Resource: resource,
				Service:  services,
			})

			if err == nil {
				msg = "Menu updated Successfully"
			} else {
				msg = fmt.Sprintf("Error while update menu: %s", err.Error())
			}
		case "create_table":
			if _, ok := arguments["menu"]; !ok {
				arguments["menu"] = "c57eedc3-a954-4262-a0af-376c65b5a284"
			}

			logReq, err = gpt.CreateTable(&models.CreateTableAI{
				Label:         cast.ToString(arguments["name"]),
				TableSlug:     cast.ToString(arguments["table_slug"]),
				Menu:          cast.ToString(arguments["menu"]),
				EnvironmentId: resource.EnvironmentId,
				UserId:        userId.(string),
				Resource:      resource,
				Service:       services,
			})

			if err == nil {
				msg = "Table created Successfully"
			} else {
				msg = fmt.Sprintf("Error while create table: %s", err.Error())
			}
		case "update_table":
			logReq, err = gpt.UpdateTable(&models.UpdateTableAI{
				OldLabel: cast.ToString(arguments["old_name"]),
				NewLabel: cast.ToString(arguments["new_name"]),
				Resource: resource,
				Service:  services,
			})

			if err == nil {
				msg = "Table updated Successfully"
			} else {
				msg = fmt.Sprintf("Error while update table: %s", err.Error())
			}
		case "create_field":
			if cast.ToString(arguments["table"]) != "" {
				logReq, err = gpt.CreateField(&models.CreateFieldAI{
					Label:    cast.ToString(arguments["label"]),
					Slug:     cast.ToString(arguments["slug"]),
					Type:     cast.ToString(arguments["type"]),
					Table:    cast.ToString(arguments["table"]),
					UserId:   userId.(string),
					Resource: resource,
					Service:  services,
				})
				if err == nil {
					msg = "Field created Successfully"
				} else {
					msg = fmt.Sprintf("Error while create field: %s", err.Error())
				}
			} else {
				msg = "Table not found please give table's label"
			}
		case "update_field":
			logReq, err = gpt.UpdateField(&models.UpdateFieldAI{
				NewLabel: cast.ToString(arguments["new_label"]),
				OldLabel: cast.ToString(arguments["old_label"]),
				NewType:  cast.ToString(arguments["new_type"]),
				Table:    cast.ToString(arguments["table"]),
				UserId:   userId.(string),
				Resource: resource,
				Service:  services,
			})
			if err == nil {
				msg = "Field updated Successfully"
			} else {
				msg = fmt.Sprintf("Error while update field: %s", err.Error())
			}
		case "delete_field":
			logReq, err = gpt.DeleteField(&models.DeleteFieldAI{
				Label:    cast.ToString(arguments["label"]),
				Table:    cast.ToString(arguments["table"]),
				UserId:   userId.(string),
				Resource: resource,
				Service:  services,
			})
			if err == nil {
				msg = "Field deleted Successfully"
			} else {
				msg = fmt.Sprintf("Error while delete field: %s", err.Error())
			}
		case "create_relation":
			logReq, err = gpt.CreateRelation(&models.CreateRelationAI{
				TableFrom:    cast.ToString(arguments["table_from"]),
				TableTo:      cast.ToString(arguments["table_to"]),
				RelationType: cast.ToString(arguments["relation_type"]),
				ViewField:    cast.ToStringSlice(arguments["view_field"]),
				ViewType:     cast.ToString(arguments["view_type"]),
				UserId:       userId.(string),
				Resource:     resource,
				Service:      services,
			})
			if err == nil {
				msg = "Relation created Successfully"
			} else {
				msg = fmt.Sprintf("Error while create relation: %s", err.Error())
			}
		case "delete_relation":
			logReq, err = gpt.DeleteRelation(&models.DeleteRelationAI{
				TableFrom:    cast.ToString(arguments["table_from"]),
				TableTo:      cast.ToString(arguments["table_to"]),
				RelationType: cast.ToString(arguments["relation_type"]),
				UserId:       userId.(string),
				Resource:     resource,
				Service:      services,
			})
			if err == nil {
				msg = "Relation deleted Successfully"
			} else {
				msg = fmt.Sprintf("Error while delete relation: %s", err.Error())
			}
		case "create_row":
			logReq, err = gpt.CreateItems(&models.CreateItemsAI{
				Table:     cast.ToString(arguments["table"]),
				Arguments: cast.ToStringSlice(arguments["arguments"]),
				UserId:    userId.(string),
				Resource:  resource,
				Service:   services,
			})
			if err == nil {
				msg = "Item created Successfully"
			} else {
				msg = fmt.Sprintf("Error while create item: %s", err.Error())
			}
		case "generate_row":
			logReq, err = gpt.GenerateItems(&models.GenerateItemsAI{
				Table:    cast.ToString(arguments["table"]),
				Count:    cast.ToInt(arguments["count"]),
				UserId:   userId.(string),
				Resource: resource,
				Service:  services,
			})
			if err == nil {
				msg = "Items created Successfully"
			} else {
				msg = fmt.Sprintf("Error while create items: %s", err.Error())
			}
		case "update_item":
			logReq, err = gpt.UpdateItems(&models.UpdateItemsAI{
				Table:     cast.ToString(arguments["table"]),
				OldColumn: arguments["old_data"],
				NewColumn: arguments["new_data"],
				UserId:    userId.(string),
				Resource:  resource,
				Service:   services,
			})
			if err == nil {
				msg = "Item updated Successfully"
			} else {
				msg = fmt.Sprintf("Error while update item: %s", err.Error())
			}
		case "delete_item":
			logReq, err = gpt.DeleteItems(&models.UpdateItemsAI{
				Table:     cast.ToString(arguments["table"]),
				OldColumn: arguments["old_data"],
				UserId:    userId.(string),
				Resource:  resource,
				Service:   services,
			})
			if err == nil {
				msg = "Item deleted Successfully"
			} else {
				msg = fmt.Sprintf("Error while delete item: %s", err.Error())
			}
		case "login_table":
			logReq, err = gpt.LoginTable(&models.LoginTableAI{
				Table:    cast.ToString(arguments["table"]),
				Login:    cast.ToString(arguments["login"]),
				Password: cast.ToString(arguments["password"]),
				UserId:   userId.(string),
				Resource: resource,
				Service:  services,
			})
			if err == nil {
				msg = "Table updated to login table Successfully"
			} else {
				msg = fmt.Sprintf("Error while update table to login table: %s", err.Error())
			}
		case "create_ofs":
			token, _ := c.Get("token")

			logReq, err = gpt.CreateFunction(&models.CreateFunctionAI{
				Table:        cast.ToString(arguments["table"]),
				FunctionName: "create-customer-ai-super",
				Prompt:       cast.ToString(arguments["prompt"]),
				Token:        token.(string),
				GitlabToken:  "glpat-QBGchQypKG2uAbx-zjXJ",
				ActionType:   cast.ToString(arguments["request_type"]),
				Method:       cast.ToString(arguments["function_type"]),
				UserId:       userId.(string),
				Resource:     resource,
				Service:      services,
			})
			if err == nil {
				msg = "Function created and added to table Successfully"
			} else {
				msg = fmt.Sprintf("Error while create function: %s", err.Error())
			}
		default:
			h.HandleResponse(c, status_http.BadRequest, "Unknown function: "+functionName)
			return
		}

		for _, log := range logReq {
			switch resource.ResourceType {
			case pb.ResourceType_MONGODB:
				go h.versionHistory(log)
			case pb.ResourceType_POSTGRESQL:
				go h.versionHistoryGo(c, log)
			}
		}

		messages = append(messages, msg)
	}

	msg := strings.Join(messages, ", ")

	h.HandleResponse(c, status_http.OK, msg)
}
