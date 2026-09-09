package helper

import (
	"context"
	"log"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	"github.com/Ucode-io/ucode-opensource/services/gateway/function"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	auth "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	gb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"

	"github.com/Ucode-io/ucode-opensource/services/gateway/services"

	"github.com/gin-gonic/gin"
)

type HandlerInterface interface {
	AuthService() services.AuthServiceManagerI
	BaseConf() config.BaseConfig
	HandleResponse(c *gin.Context, status status_http.Status, data any)
	GetAuthInfo(c *gin.Context) (result *auth.V2HasAccessUserRes, err error)
	GetProjectSrvc(c context.Context, projectId string, nodeType string) (services.ServiceManagerI, error)
}

func DoInvokeFunction(request models.DoInvokeFunctionStruct, c *gin.Context, h HandlerInterface) (functionName string, err error) {
	apiKeys, err := h.AuthService().ApiKey().GetList(context.Background(), &auth_service.GetListReq{
		EnvironmentId: request.Resource.EnvironmentId,
		ProjectId:     request.Resource.ProjectId,
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, "error getting api keys by environment id")
		return
	}

	var appId string
	if len(apiKeys.Data) > 0 {
		appId = apiKeys.Data[0].AppId
	} else {
		h.HandleResponse(c, status_http.GRPCError, "error no app id for this environment")
		return
	}

	authInfo, _ := h.GetAuthInfo(c)

	for _, customEvent := range request.CustomEvents {
		var (
			invokeFunction models.NewInvokeFunctionRequest
			path           = customEvent.GetFunctions()[0].GetPath()
			name           = customEvent.GetFunctions()[0].GetName()
			requestType    = customEvent.GetFunctions()[0].GetRequestType()

			funcError error

			status      = "success"
			duration    int64
			sendTime    time.Time
			completedAt time.Time
		)

		if customEvent.Path != "" {
			path = customEvent.Path
		}

		data, err := helper.ConvertStructToResponse(customEvent.Attributes)
		if err != nil {
			return customEvent.GetFunctions()[0].Name, err
		}

		data["object_ids"] = request.IDs
		data["table_slug"] = request.TableSlug
		data["object_data"] = request.ObjectData
		data["object_data_before_update"] = request.ObjectDataBeforeUpdate
		data["method"] = request.Method
		data["action_type"] = request.ActionType
		data["app_id"] = appId
		data["user_id"] = authInfo.GetUserId()
		data["session_id"] = authInfo.GetId()
		data["project_id"] = request.Resource.ProjectId
		data["environment_id"] = request.Resource.EnvironmentId
		invokeFunction.Data = data
		invokeFunction.OpenFaaSURL = h.BaseConf().OpenFaaSBaseUrl
		invokeFunction.KnativeURL = h.BaseConf().KnativeBaseUrl
		//HISOBIM project -> UZ knative cluster
		if request.Resource.ProjectId == "b744d518-5f66-4818-bfd7-9f3f44ce3379" && h.BaseConf().KnativeBaseUrlUz != "" {
			invokeFunction.KnativeURL = h.BaseConf().KnativeBaseUrlUz
		}
		invokeFunction.AutomationURL = h.BaseConf().AutomationURL

		switch requestType {
		case "", "ASYNC":
			sendTime = time.Now()
			functionName, funcError = function.FuncHandlers[customEvent.Functions[0].Type](path, name, invokeFunction)
			if funcError != nil {
				status = "error"
			}

			completedAt = time.Now()
			duration = time.Since(sendTime).Milliseconds()

		case "SYNC":
			go func(customEvent *obs.CustomEvent) {
				_, _ = function.FuncHandlers[customEvent.Functions[0].Type](path, name, invokeFunction)
			}(customEvent)
		}

		go func() {
			if request.Resource.ResourceType == pb.ResourceType_POSTGRESQL {
				_, err = request.Services.VersionHistory().CreateFunctionLog(c, &gb.FunctionLogReq{
					ProjectId:     request.Resource.ResourceEnvironmentId,
					FunctionId:    customEvent.Functions[0].GetId(),
					TableSlug:     request.TableSlug,
					RequestMethod: request.Method,
					ActionType:    request.ActionType,
					SendAt:        sendTime.Format(time.DateTime),
					CompletedAt:   completedAt.Format(time.DateTime),
					Duration:      duration,
					Status:        status,
					ReturnSize:    0,
				})
				if err != nil {
					log.Println("error creating function log:", err)
					return
				}
			}
		}()

		if funcError != nil {
			return functionName, funcError
		}

	}
	return
}

func GetListCustomEvents(request models.GetListCustomEventsStruct, c *gin.Context, h HandlerInterface) (beforeEvents, afterEvents []*obs.CustomEvent, err error) {
	var (
		res   *obs.GetCustomEventsListResponse
		goRes *nb.GetCustomEventsListResponse
	)

	services2, err := h.GetProjectSrvc(c.Request.Context(), request.Resource.ProjectId, request.Resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch request.Resource.ResourceType {
	case pb.ResourceType_MONGODB:
		res, err = services2.GetBuilderServiceByType(request.Resource.NodeType).CustomEvent().GetList(
			c.Request.Context(), &obs.GetCustomEventsListRequest{
				TableSlug: request.TableSlug,
				Method:    request.Method,
				RoleId:    request.RoleId,
				ProjectId: request.Resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		goRes, err = services2.GoObjectBuilderService().CustomEvent().GetList(
			c.Request.Context(), &nb.GetCustomEventsListRequest{
				TableSlug: request.TableSlug,
				Method:    request.Method,
				RoleId:    request.RoleId,
				ProjectId: request.Resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		if err = helper.MarshalToStruct(goRes, &res); err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}
	}

	if res != nil {
		for _, customEvent := range res.CustomEvents {

			switch customEvent.ActionType {
			case config.BEFORE:
				beforeEvents = append(beforeEvents, customEvent)
			case config.AFTER:
				afterEvents = append(afterEvents, customEvent)
			}
		}
	}

	return
}
