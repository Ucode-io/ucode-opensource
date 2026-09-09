package v1

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	_ "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_function_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

// CreateNewFunction godoc
// @Security ApiKeyAuth
// @ID create_new_function
// @Router /v2/function [POST]
// @Summary Create New Function
// @Description Create New Function
// @Tags Function
// @Accept json
// @Produce json
// @Param Function body models.CreateFunctionRequest true "CreateFunctionRequestBody"
// @Success 201 {object} status_http.Response{data=object} "Function data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) CreateNewFunction(c *gin.Context) {
	logText := fmt.Sprintf("I am inside create func %v-%v-%v", h.baseConf.GoFunctionServiceHost, h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
	h.log.Info(logText)
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// GetNewFunctionByID godoc
// @Security ApiKeyAuth
// @ID get_new_function_by_id
// @Router /v2/function/{function_id} [GET]
// @Summary Get Function by id
// @Description Get Function by id
// @Tags Function
// @Accept json
// @Produce json
// @Param function_id path string true "function_id"
// @Success 200 {object} status_http.Response{data=object} "FunctionBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetNewFunctionByID(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// GetAllNewFunctions godoc
// @Security ApiKeyAuth
// @ID get_all_new_functions
// @Router /v2/function [GET]
// @Summary Get all functions
// @Description Get all functions
// @Tags Function
// @Accept json
// @Produce json
// @Param limit query number false "limit"
// @Param offset query number false "offset"
// @Param search query string false "search"
// @Success 200 {object} status_http.Response{data=string} "FunctionBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetAllNewFunctions(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// UpdateNewFunction godoc
// @Security ApiKeyAuth
// @ID update_new_function
// @Router /v2/function [PUT]
// @Summary Update new function
// @Description Update new function
// @Tags Function
// @Accept json
// @Produce json
// @Param Function body models.Function  true "UpdateFunctionRequestBody"
// @Success 200 {object} status_http.Response{data=models.Function} "Function data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateNewFunction(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// DeleteNewFunction godoc
// @Security ApiKeyAuth
// @ID delete_new_function
// @Router /v2/function/{function_id} [DELETE]
// @Summary Delete New Function
// @Description Delete New Function
// @Tags Function
// @Accept json
// @Produce json
// @Param function_id path string true "function_id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) DeleteNewFunction(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// GetFunctionCodebase godoc
// @Security ApiKeyAuth
// @ID get_function_codebase
// @Router /v2/function/{function_id}/codebase [GET]
// @Summary Get function codebase from GitLab
// @Description Returns all files of the function's GitLab repository recursively. Tries the stored branch first, then falls back to master/main.
// @Tags Function
// @Accept json
// @Produce json
// @Param function_id path string true "function_id"
// @Success 200 {object} status_http.Response{data=object} "Codebase files"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetFunctionCodebase(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// InvokeFunctionByPath godoc
// @Security ApiKeyAuth
// @Param function-path path string true "function-path"
// @ID invoke_function_by_path
// @Router /v1/invoke_function/{function-path} [POST]
// @Summary Invoke Function By Path
// @Description Invoke Function By Path
// @Tags Function
// @Accept json
// @Produce json
// @Param InvokeFunctionByPathRequest body models.CommonMessage true "InvokeFunctionByPathRequest"
// @Success 201 {object} status_http.Response{data=models.InvokeFunctionRequest} "Function data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) InvokeFunctionByPath(c *gin.Context) {
	var invokeFunction models.CommonMessage

	if err := c.ShouldBindJSON(&invokeFunction); err != nil {
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
		h.HandleResponse(c, status_http.BadRequest, "error getting environment id | not valid")
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_FUNCTION_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	apiKeys, err := h.authService.ApiKey().GetList(c.Request.Context(), &auth_service.GetListReq{
		EnvironmentId: environmentId.(string),
		ProjectId:     resource.ProjectId,
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	if len(apiKeys.Data) < 1 {
		h.HandleResponse(c, status_http.InvalidArgument, "Api key not found")
		return
	}

	authInfo, _ := h.GetAuthInfo(c)
	invokeFunction.Data["user_id"] = authInfo.GetUserId()
	invokeFunction.Data["project_id"] = authInfo.GetProjectId()
	invokeFunction.Data["environment_id"] = authInfo.GetEnvId()
	invokeFunction.Data["app_id"] = apiKeys.GetData()[0].GetAppId()

	resp, err := util.DoRequest(h.baseConf.OpenFaaSBaseUrl+c.Param("function-path"), http.MethodPost,
		models.NewInvokeFunctionRequest{
			Data: invokeFunction.Data,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	} else if resp.Status == "error" {
		var errStr = resp.Status
		if resp.Data != nil && resp.Data["message"] != nil {
			errStr = resp.Data["message"].(string)
		}
		h.HandleResponse(c, status_http.InvalidArgument, errStr)
		return
	}

	h.HandleResponse(c, status_http.Created, resp)
}

// FunctionRun godoc
// @Security ApiKeyAuth
// @ID function_run
// @Router /v2/functions/{function-id}/run [POST]
// @Summary Function Run
// @Description Function Run
// @Tags Function
// @Accept json
// @Produce json
// @Param InvokeFunctionRequest body models.InvokeFunctionRequest true "InvokeFunctionRequest"
// @Success 200 {object} status_http.Response{data=models.InvokeFunctionResponse} "Function data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) FunctionRun(c *gin.Context) {
	var (
		requestData    models.HttpRequest
		invokeFunction models.InvokeFunctionRequest
	)

	bodyReq, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.log.Error("cant read request body", logger.Any("req", c.Request))
		return
	}

	if err = json.Unmarshal(bodyReq, &invokeFunction); err != nil {
		h.log.Error("cant parse body or an empty body received", logger.Any("error", err.Error()))
	}

	if cast.ToBool(c.GetHeader("/v1/functions/")) {
		var authData = models.AuthData{}
		err = json.Unmarshal([]byte(c.GetHeader("auth")), &authData)
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, "cant get auth info")
			return
		}

		c.Set("auth", authData)
		c.Set("resource_id", c.GetHeader("resource_id"))
		c.Set("environment_id", c.GetHeader("environment_id"))
		c.Set("project_id", c.GetHeader("project_id"))
		c.Set("resource", c.GetHeader("resource"))
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "error getting environment id | not valid")
		return
	}

	var resource *pb.ServiceResourceModel
	resourceBody, ok := c.Get("resource")
	if resourceBody != "" && ok {
		var resourceList *pb.GetResourceByEnvIDResponse

		if err = json.Unmarshal([]byte(resourceBody.(string)), &resourceList); err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		resource = &pb.ServiceResourceModel{
			ServiceType:           resourceList.ResourceEnvironment.ServiceType,
			ProjectId:             resourceList.ResourceEnvironment.ProjectId,
			ResourceId:            resourceList.ResourceEnvironment.ResourceId,
			ResourceEnvironmentId: resourceList.ResourceEnvironment.ResourceEnvironmentId,
			EnvironmentId:         resourceList.ResourceEnvironment.EnvironmentId,
			ResourceType:          resourceList.ResourceEnvironment.ResourceType,
			NodeType:              resourceList.ResourceEnvironment.NodeType,
		}
	} else {
		resource, err = h.companyServices.ServiceResource().GetSingle(
			c.Request.Context(), &pb.GetSingleServiceResourceReq{
				ProjectId:     projectId.(string),
				EnvironmentId: environmentId.(string),
				ServiceType:   pb.ServiceType_FUNCTION_SERVICE,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}

	authInfoAny, ok := c.Get("auth")
	if !ok {
		h.HandleResponse(c, status_http.InvalidArgument, "cant get auth info")
		return
	}

	authInfo := authInfoAny.(models.AuthData)
	requestData.Method = c.Request.Method
	requestData.Headers = c.Request.Header
	requestData.Path = c.Request.URL.Path
	requestData.Params = c.Request.URL.Query()
	requestData.Body = bodyReq

	var functionResponse = &obs.Function{}

	if util.IsValidUUID(c.Param("function-id")) {
		services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		switch resource.ResourceType {
		case pb.ResourceType_MONGODB:
			functionResponse, err = services.GetBuilderServiceByType(resource.NodeType).Function().GetSingle(
				c.Request.Context(), &obs.FunctionPrimaryKey{
					Id:        c.Param("function-id"),
					ProjectId: resource.ResourceEnvironmentId,
				},
			)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
		case pb.ResourceType_POSTGRESQL:
			resp, err := services.GoObjectBuilderService().Function().GetSingle(
				c.Request.Context(), &nb.FunctionPrimaryKey{
					Id:        c.Param("function-id"),
					ProjectId: resource.ResourceEnvironmentId,
				},
			)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}

			if err = helper.MarshalToStruct(resp, &functionResponse); err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
		}
	} else {
		functionResponse.Path = c.Param("function-id")
	}

	var resp = models.InvokeFunctionResponse{}

	switch functionResponse.Type {
	case config.FUNCTION:
		resp, err = util.DoRequest(h.baseConf.OpenFaaSBaseUrl+functionResponse.GetPath(), http.MethodPost,
			models.FunctionRunV2{
				Auth:        models.AuthData{},
				RequestData: requestData,
				Data: map[string]any{
					"object_ids": invokeFunction.ObjectIDs,
					"attributes": invokeFunction.Attributes,
					"app_id":     authInfo.Data["app_id"],
				},
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		} else if resp.Status == "error" {
			var errStr = resp.Status
			if resp.Data != nil && resp.Data["message"] != nil {
				errStr = resp.Data["message"].(string)
			}
			h.HandleResponse(c, status_http.InvalidArgument, errStr)
			return
		}
	case config.KNATIVE:
		resp, err = util.DoRequest(fmt.Sprintf("http://%s.%s", functionResponse.GetPath(), h.baseConf.KnativeBaseUrl), http.MethodPost,
			models.FunctionRunV2{
				Auth:        models.AuthData{},
				RequestData: requestData,
				Data: map[string]any{
					"object_ids": invokeFunction.ObjectIDs,
					"attributes": invokeFunction.Attributes,
					"app_id":     authInfo.Data["app_id"],
				},
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		} else if resp.Status == "error" {
			var errStr = resp.Status
			if resp.Data != nil && resp.Data["message"] != nil {
				errStr = resp.Data["message"].(string)
			}
			h.HandleResponse(c, status_http.InvalidArgument, errStr)
			return
		}
	}

	if isOwnData, ok := resp.Attributes["is_own_data"].(bool); ok {
		if isOwnData {
			if _, ok := resp.Data["code"]; ok {
				h.log.Info("own data with code", logger.Any("data", resp.Data))
				c.JSON(cast.ToInt(resp.Data["code"]), resp.Data)
				return
			}
			h.log.Info("own data", logger.Any("data", resp.Data))
			c.JSON(http.StatusOK, resp.Data)
			return
		}
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// GetAllNewFunctionsForApp godoc
// @Security ApiKeyAuth
// @ID get_all_new_functions_for_app
// @Router /v2/function-for-app [GET]
// @Summary Get all functions
// @Description Get all functions
// @Tags Function
// @Accept json
// @Produce json
// @Param limit query number false "limit"
// @Param offset query number false "offset"
// @Param search query string false "search"
// @Success 200 {object} status_http.Response{data=string} "FunctionBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetAllNewFunctionsForApp(c *gin.Context) {
	includeCustomEvents := c.DefaultQuery("include_custom_events", "false") == "true"

	limit, err := h.getLimitParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "error getting environment id | not valid")
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_FUNCTION_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).Function().GetList(
			c.Request.Context(), &obs.GetAllFunctionsRequest{
				Search:              c.Query("search"),
				Limit:               int32(limit),
				Offset:              int32(offset),
				ProjectId:           resource.ResourceEnvironmentId,
				Type:                []string{config.FUNCTION, config.KNATIVE, config.WORKFLOW},
				IncludeCustomEvents: includeCustomEvents,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Function().GetList(
			c.Request.Context(), &nb.GetAllFunctionsRequest{
				Search:              c.Query("search"),
				Limit:               int32(limit),
				Offset:              int32(offset),
				ProjectId:           resource.ResourceEnvironmentId,
				Type:                []string{config.FUNCTION, config.KNATIVE, config.WORKFLOW},
				IncludeCustomEvents: includeCustomEvents,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	}
}
