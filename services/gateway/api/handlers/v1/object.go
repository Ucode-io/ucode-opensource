package v1

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/billing"
	hHelper "github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pba "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/security"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/spf13/cast"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

// CreateObject godoc
// @Security ApiKeyAuth
// @ID create_object
// @Router /v1/object/{collection}/ [POST]
// @Summary Create object
// @Description Create object
// @Tags Object
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param object body models.CommonMessage true "CreateObjectRequestBody"
// @Success 201 {object} status_http.Response{data=models.CommonMessage} "Object data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) CreateObject(c *gin.Context) {
	var (
		objectRequest               models.CommonMessage
		resp                        *obs.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["Created"]
		err                         error
	)

	tableSlug := c.Param("collection")

	if err = c.ShouldBindJSON(&objectRequest); err != nil {
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
	}

	objectRequest.Data["company_service_project_id"] = resource.GetProjectId()
	objectRequest.Data["company_service_environment_id"] = resource.GetEnvironmentId()

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if resource.ResourceType == pb.ResourceType_POSTGRESQL {
		if err = billing.CheckDatabaseLimit(c.Request.Context(), h.centralRedis, h.companyServices, services, projectId.(string), resource.ResourceEnvironmentId, resource.NodeType); err != nil {
			if errors.Is(err, billing.ErrDatabaseLimitExceeded) {
				h.HandleResponse(c, status_http.PaymentRequired, models.PaymentDatabaseLimit)
			} else {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
			}
			return
		}
	}

	service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()

	id := uuid.New().String()

	guid, ok := objectRequest.Data["guid"]
	if ok {
		if util.IsValidUUID(guid.(string)) {
			id = objectRequest.Data["guid"].(string)
		}
	}

	objectRequest.Data["guid"] = id

	// THIS for loop is written to create child objects (right now it is used in the case of One2One relation)
	for key, value := range objectRequest.Data {
		if key[0] == '$' {
			var (
				interfaceToMap = value.(map[string]any)
				id             = uuid.New()
			)

			interfaceToMap["guid"] = id

			mapToStruct, err := helper.ConvertMapToStruct(interfaceToMap)
			if err != nil {
				h.HandleResponse(c, status_http.InvalidArgument, err.Error())
				return
			}

			_, err = service.Create(c.Request.Context(),
				&obs.CommonMessage{
					TableSlug: key[1:],
					Data:      mapToStruct,
					ProjectId: resource.ResourceEnvironmentId,
				},
			)

			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}

			objectRequest.Data[key[1:]+"_id"] = id
		}
	}

	structData, err := helper.ConvertMapToStruct(objectRequest.Data)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	fromOfs := c.Query("from-ofs")
	if fromOfs != "true" {
		event := models.GetListCustomEventsStruct{
			TableSlug: tableSlug,
			RoleId:    "",
			Method:    "CREATE",
			Resource:  resource,
		}

		beforeActions, afterActions, err = hHelper.GetListCustomEvents(event, c, h)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}
	}

	if len(beforeActions) > 0 {
		invoke := models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: beforeActions,
			IDs:          []string{id},
			TableSlug:    tableSlug,
			ObjectData:   objectRequest.Data,
			Method:       "CREATE",
			Resource:     resource,
		}
		functionName, err := hHelper.DoInvokeFunction(invoke, c, h)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = service.Create(c.Request.Context(),
			&obs.CommonMessage{
				TableSlug:         tableSlug,
				Data:              structData,
				ProjectId:         resource.ResourceEnvironmentId,
				BlockedLoginTable: cast.ToBool(c.DefaultQuery("blocked_login_table", "false")),
				BlockedBuilder:    cast.ToBool(c.DefaultQuery("block_builder", "false"))},
		)
		// this logic for custom error message, object builder service may be return 400, 404, 500
		if err != nil {
			statusHttp = status_http.GrpcStatusToHTTP["Internal"]
			stat, ok := status.FromError(err)
			if ok {
				statusHttp = status_http.GrpcStatusToHTTP[stat.Code().String()]
				statusHttp.CustomMessage = stat.Message()
			}
			h.HandleResponse(c, statusHttp, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		h.HandleResponse(c, status_http.BadRequest, "does not implemented")
		return
	}

	if data, ok := resp.Data.AsMap()["data"].(map[string]any); ok {
		objectRequest.Data = data
		if _, ok = data["guid"].(string); ok {
			id = data["guid"].(string)
		}
	}

	if len(afterActions) > 0 {
		invoke := models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: afterActions,
			IDs:          []string{id},
			TableSlug:    tableSlug,
			ObjectData:   objectRequest.Data,
			Method:       "CREATE",
			Resource:     resource,
		}
		functionName, err := hHelper.DoInvokeFunction(invoke, c, h)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	statusHttp.CustomMessage = resp.GetCustomMessage()
	h.HandleResponse(c, statusHttp, resp)
}

// GetSingle godoc
// @Security ApiKeyAuth
// @ID get_object_by_id
// @Router /v1/object/{collection}/{object_id} [GET]
// @Summary Get object by id
// @Description Get object by id
// @Tags Object
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param object_id path string true "object_id"
// @Success 200 {object} status_http.Response{data=models.CommonMessage} "ObjectBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetSingle(c *gin.Context) {
	var (
		object     models.CommonMessage
		resp       *obs.CommonMessage
		statusHttp = status_http.GrpcStatusToHTTP["Ok"]
		tableSlug  = c.Param("collection")
	)

	object.Data = make(map[string]any)

	objectID := c.Param("object_id")
	if !util.IsValidUUID(objectID) {
		h.HandleResponse(c, status_http.InvalidArgument, "object_id is an invalid uuid")
		return
	}

	tokenInfo, err := h.GetAuthInfo(c)
	if err != nil {
		h.HandleResponse(c, status_http.Forbidden, err.Error())
		return
	}
	if tokenInfo != nil {
		object.Data["user_id_from_token"] = tokenInfo.GetUserId()
		object.Data["role_id_from_token"] = tokenInfo.GetRoleId()
		object.Data["client_type_id_from_token"] = tokenInfo.GetClientTypeId()
	}

	object.Data["id"] = objectID

	structData, err := helper.ConvertMapToStruct(object.Data)
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
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
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

	service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = service.GetSingle(c.Request.Context(),
			&obs.CommonMessage{
				TableSlug: tableSlug,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			statusHttp = status_http.GrpcStatusToHTTP["Internal"]
			stat, ok := status.FromError(err)
			if ok {
				statusHttp = status_http.GrpcStatusToHTTP[stat.Code().String()]
				statusHttp.CustomMessage = stat.Message()
			}
			h.HandleResponse(c, statusHttp, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		h.HandleResponse(c, status_http.BadEnvironment, "does not implemented")
		return
	}

	statusHttp.CustomMessage = resp.GetCustomMessage()
	h.HandleResponse(c, statusHttp, resp)
}

// GetSingleSlim godoc
// @Security ApiKeyAuth
// @ID get_object_by_id_slim
// @Router /v1/object-slim/{collection}/{object_id} [GET]
// @Summary Get object by id slim
// @Description Get object by id slim
// @Tags Object
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param object_id path string true "object_id"
// @Success 200 {object} status_http.Response{data=models.CommonMessage} "ObjectBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetSingleSlim(c *gin.Context) {
	var (
		object     models.CommonMessage
		statusHttp = status_http.GrpcStatusToHTTP["Ok"]
		hashed     bool
		tableSlug  = c.Param("collection")
	)

	object.Data = make(map[string]any)

	objectID := c.Param("object_id")
	if !util.IsValidUUID(objectID) {
		hashData, err := security.Decrypt(c.Param("object_id"), h.baseConf.SecretKey)
		if err == nil {
			objectID = hashData
			hashed = true
		} else {
			h.HandleResponse(c, status_http.InvalidArgument, "object_id is an invalid uuid")
			return
		}
	}

	object.Data["id"] = objectID
	object.Data["with_relations"] = c.DefaultQuery("with_relations", "false")

	structData, err := helper.ConvertMapToStruct(object.Data)
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

	userId, _ := c.Get("user_id")
	apiKey := c.GetHeader("X-API-KEY")

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

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "GET",
			UserInfo:     cast.ToString(userId),
			Request:      &structData,
			ApiKey:       apiKey,
			Type:         "API_KEY",
			TableSlug:    tableSlug,
		}
	)

	redisKey := base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "%s-%s-%s", tableSlug, structData.String(), resource.ResourceEnvironmentId))

	service := services.GetBuilderServiceByType(resource.NodeType)

	accessType, ok := c.Get("with_access")
	if ok && !cast.ToBool(accessType) {
		switch resource.ResourceType {
		case pb.ResourceType_MONGODB:
			permission, err := service.Permission().GetTablePermission(
				c.Request.Context(),
				&obs.GetTablePermissionRequest{
					TableSlug:             tableSlug,
					ResourceEnvironmentId: resource.ResourceEnvironmentId,
					Method:                "read",
				},
			)
			if err != nil {
				h.HandleResponse(c, status_http.Forbidden, "table is not public")
				return
			}

			if !permission.IsHavePermission {
				h.HandleResponse(c, status_http.Forbidden, "table is not public")
				return
			}
		case pb.ResourceType_POSTGRESQL:
			permission, err := services.GoObjectBuilderService().Permission().GetTablePermission(
				c.Request.Context(),
				&nb.GetTablePermissionRequest{
					TableSlug:             tableSlug,
					ResourceEnvironmentId: resource.ResourceEnvironmentId,
					Method:                "read",
				},
			)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, "table is not public")
				return
			}

			if !permission.IsHavePermission {
				h.HandleResponse(c, status_http.Forbidden, "table is not public")
				return
			}
		}
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()
		redisResp, err := h.redis.Get(c.Request.Context(), redisKey, projectId.(string), resource.NodeType)
		if err == nil {
			var (
				resp = make(map[string]any)
				m    = make(map[string]any)
			)

			if err = json.Unmarshal([]byte(redisResp), &m); err != nil {
				h.log.Error("Error while unmarshal redis", logger.Error(err))
			} else {
				resp["data"] = m
				h.HandleResponse(c, status_http.OK, resp)
				logReq.Response = m
				go h.versionHistory(logReq)
				return
			}
		} else {
			h.log.Error("Error while getting redis", logger.Error(err))
		}

		resp, err := service.GetSingleSlim(
			c.Request.Context(),
			&obs.CommonMessage{
				TableSlug: tableSlug,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			statusHttp = status_http.GrpcStatusToHTTP["Internal"]
			stat, ok := status.FromError(err)
			if ok {
				statusHttp = status_http.GrpcStatusToHTTP[stat.Code().String()]
				statusHttp.CustomMessage = stat.Message()
			}
			logReq.Response = err.Error()
			go h.versionHistory(logReq)
			h.HandleResponse(c, statusHttp, err.Error())
			return
		}

		logReq.Response = resp
		go h.versionHistory(logReq)

		if resp.IsCached {
			jsonData, _ := resp.GetData().MarshalJSON()
			err = h.redis.SetX(c.Request.Context(), redisKey, string(jsonData), 15*time.Second, projectId.(string), resource.NodeType)
			if err != nil {
				h.log.Error("Error while setting redis", logger.Error(err))
			}
		}

		statusHttp.CustomMessage = resp.GetCustomMessage()
		if hashed {
			response, err := security.Encrypt(resp, h.baseConf.SecretKey)
			if err != nil {
				h.HandleResponse(c, status_http.InternalServerError, err.Error())
				return
			}
			h.HandleResponse(c, statusHttp, response)
			return
		}
		h.HandleResponse(c, statusHttp, resp)
	case pb.ResourceType_POSTGRESQL:
		redisResp, err := h.redis.Get(c.Request.Context(), redisKey, projectId.(string), resource.NodeType)
		if err == nil {
			var (
				resp = make(map[string]any)
				m    = make(map[string]any)
			)

			if err = json.Unmarshal([]byte(redisResp), &m); err != nil {
				h.log.Error("Error while unmarshal redis", logger.Error(err))
			} else {
				resp["data"] = m
				h.HandleResponse(c, status_http.OK, resp)
				logReq.Response = m
				go h.versionHistory(logReq)
				return
			}
		} else {
			h.log.Error("Error while getting redis", logger.Error(err))
		}

		resp, err := services.GoObjectBuilderService().ObjectBuilder().GetSingleSlim(
			c.Request.Context(), &nb.CommonMessage{
				TableSlug: tableSlug,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			statusHttp = status_http.GrpcStatusToHTTP["Internal"]
			stat, ok := status.FromError(err)
			if ok {
				statusHttp = status_http.GrpcStatusToHTTP[stat.Code().String()]
				statusHttp.CustomMessage = stat.Message()
			}
			logReq.Response = err.Error()
			go h.versionHistoryGo(c, logReq)
			h.HandleResponse(c, statusHttp, err.Error())
			return
		}

		logReq.Response = resp
		go h.versionHistoryGo(c, logReq)

		if resp.IsCached {
			jsonData, _ := resp.GetData().MarshalJSON()
			err = h.redis.SetX(c.Request.Context(), redisKey, string(jsonData), 15*time.Second, projectId.(string), resource.NodeType)
			if err != nil {
				h.log.Error("Error while setting redis", logger.Error(err))
			}
		}

		statusHttp.CustomMessage = resp.GetCustomMessage()
		h.HandleResponse(c, statusHttp, resp)
	}

}

// UpdateObject godoc
// @Security ApiKeyAuth
// @ID update_object
// @Router /v1/object/{collection} [PUT]
// @Summary Update object
// @Description Update object
// @Tags Object
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param object body models.CommonMessage true "UpdateObjectRequestBody"
// @Success 200 {object} status_http.Response{data=models.CommonMessage} "Object data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateObject(c *gin.Context) {
	var (
		objectRequest               models.CommonMessage
		resp, singleObject          *obs.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["Ok"]
		tableSlug                   = c.Param("collection")
	)

	if err := c.ShouldBindJSON(&objectRequest); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	structData, err := helper.ConvertMapToStruct(objectRequest.Data)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	var id string
	if objectRequest.Data["guid"] != nil {
		id = objectRequest.Data["guid"].(string)
	} else {
		h.HandleResponse(c, status_http.BadRequest, "guid is required")
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

	var resource *pb.ServiceResourceModel
	resourceBody, ok := c.Get("resource")
	if ok {
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
				ServiceType:   pb.ServiceType_BUILDER_SERVICE,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()

	fromOfs := c.Query("from-ofs")
	if fromOfs != "true" {
		beforeActions, afterActions, err = hHelper.GetListCustomEvents(models.GetListCustomEventsStruct{
			TableSlug: tableSlug,
			RoleId:    "",
			Method:    "UPDATE",
			Resource:  resource,
		},
			c,
			h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}

		switch resource.ResourceType {
		case pb.ResourceType_MONGODB:
			singleObject, err = service.GetSingle(
				c.Request.Context(),
				&obs.CommonMessage{
					TableSlug: tableSlug,
					Data:      &structpb.Struct{Fields: map[string]*structpb.Value{"id": structpb.NewStringValue(id)}},
					ProjectId: resource.ResourceEnvironmentId,
				},
			)
			if err != nil {
				statusHttp = status_http.GrpcStatusToHTTP["Internal"]
				stat, ok := status.FromError(err)
				if ok {
					statusHttp = status_http.GrpcStatusToHTTP[stat.Code().String()]
					statusHttp.CustomMessage = stat.Message()
				}
				h.HandleResponse(c, statusHttp, err.Error())
				return
			}
		case pb.ResourceType_POSTGRESQL:
			h.HandleResponse(c, status_http.BadEnvironment, "does not implemented")
			return
		}
	}

	if len(beforeActions) > 0 {
		functionName, err := hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: beforeActions,
			IDs:          []string{id},
			TableSlug:    tableSlug,
			ObjectData:   objectRequest.Data,
			Method:       "UPDATE",
			Resource:     resource,
		},
			c,
			h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = service.Update(
			c.Request.Context(), &obs.CommonMessage{
				TableSlug:        tableSlug,
				Data:             structData,
				ProjectId:        resource.ResourceEnvironmentId,
				BlockedBuilder:   cast.ToBool(c.DefaultQuery("block_builder", "false")),
				EnvId:            resource.EnvironmentId,
				CompanyProjectId: resource.ProjectId,
			},
		)
		if err != nil {
			statusHttp = status_http.GrpcStatusToHTTP["Internal"]
			stat, ok := status.FromError(err)
			if ok {
				statusHttp = status_http.GrpcStatusToHTTP[stat.Code().String()]
				statusHttp.CustomMessage = stat.Message()
			}
			h.HandleResponse(c, statusHttp, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		// Does Not Implemented
		h.HandleResponse(c, status_http.BadRequest, "does not implemented")
		return
	}

	if tableSlug == "record_permission" {
		if objectRequest.Data["role_id"] == nil {
			h.HandleResponse(c, status_http.BadRequest, "role id must be have in update permission")
			return
		}

		service, conn, err := h.authService.Session(c)
		if err != nil {
			h.HandleResponse(c, status_http.BadEnvironment, err.Error())
			return
		}
		defer conn.Close()

		_, err = service.UpdateSessionsByRoleId(
			c.Request.Context(),
			&pba.UpdateSessionByRoleIdRequest{
				RoleId:    objectRequest.Data["role_id"].(string),
				IsChanged: true,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}

	if len(afterActions) > 0 {
		functionName, err := hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:               services.GoObjectBuilderService(),
			CustomEvents:           afterActions,
			IDs:                    []string{id},
			TableSlug:              tableSlug,
			ObjectData:             objectRequest.Data,
			Method:                 "UPDATE",
			ObjectDataBeforeUpdate: singleObject.Data.AsMap(),
			Resource:               resource,
		},
			c, // gin context,
			h, // handler
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	statusHttp.CustomMessage = resp.GetCustomMessage()
	h.HandleResponse(c, status_http.OK, resp)
}

// DeleteObject godoc
// @Security ApiKeyAuth
// @ID delete_object
// @Router /v1/object/{collection}/{object_id} [DELETE]
// @Summary Delete object
// @Description Delete object
// @Tags Object
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param object body models.CommonMessage true "DeleteObjectRequestBody"
// @Param object_id path string true "object_id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) DeleteObject(c *gin.Context) {
	var (
		objectRequest               models.CommonMessage
		resp                        *obs.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["NoContent"]
		tableSlug                   = c.Param("collection")
	)

	if err := c.ShouldBindJSON(&objectRequest); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	objectID := c.Param("object_id")
	if !util.IsValidUUID(objectID) {
		h.HandleResponse(c, status_http.InvalidArgument, "object id is an invalid uuid")
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

	resource, _ := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)

	objectRequest.Data["id"] = objectID
	objectRequest.Data["company_service_project_id"] = projectId.(string)
	objectRequest.Data["company_service_environment_id"] = environmentId.(string)

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()

	structData, err := helper.ConvertMapToStruct(objectRequest.Data)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	fromOfs := c.Query("from-ofs")
	if fromOfs != "true" {
		beforeActions, afterActions, err = hHelper.GetListCustomEvents(models.GetListCustomEventsStruct{
			TableSlug: tableSlug,
			RoleId:    "",
			Method:    http.MethodDelete,
			Resource:  resource,
		},
			c,
			h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}
	}

	if len(beforeActions) > 0 {
		functionName, err := hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: beforeActions,
			IDs:          []string{objectID},
			TableSlug:    tableSlug,
			ObjectData:   objectRequest.Data,
			Method:       "DELETE",
			Resource:     resource,
		},
			c,
			h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = service.Delete(
			c.Request.Context(),
			&obs.CommonMessage{
				TableSlug: tableSlug,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			statusHttp = status_http.GrpcStatusToHTTP["Internal"]
			stat, ok := status.FromError(err)
			if ok {
				statusHttp = status_http.GrpcStatusToHTTP[stat.Code().String()]
				statusHttp.CustomMessage = stat.Message()
			}
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		// Does Not Implemented
		h.HandleResponse(c, status_http.BadRequest, "does not implemented")
		return
	}

	if len(afterActions) > 0 {
		functionName, err := hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: afterActions,
			IDs:          []string{objectID},
			TableSlug:    tableSlug,
			ObjectData:   objectRequest.Data,
			Method:       "DELETE",
			Resource:     resource,
		},
			c, // gin context,
			h, // handler
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	statusHttp.CustomMessage = resp.GetCustomMessage()
	h.HandleResponse(c, statusHttp, resp)
}

// GetList godoc
// @Security ApiKeyAuth
// @ID get_list_objects
// @Router /v1/object/get-list/{collection} [POST]
// @Summary Get all objects
// @Description Get all objects
// @Tags Object
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param language_setting query string false "language_setting"
// @Param object body models.CommonMessage true "GetListObjectRequestBody"
// @Success 200 {object} status_http.Response{data=models.CommonMessage} "ObjectBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetList(c *gin.Context) {
	var (
		objectRequest models.CommonMessage
		resp          *obs.CommonMessage
		statusHttp    = status_http.GrpcStatusToHTTP["Ok"]
		tableSlug     = c.Param("collection")
	)

	if err := c.ShouldBindJSON(&objectRequest); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	tokenInfo, err := h.GetAuthInfo(c)
	if err != nil {
		h.HandleResponse(c, status_http.Forbidden, err.Error())
		return
	}

	if tokenInfo != nil {
		if tokenInfo.Tables != nil {
			objectRequest.Data["tables"] = tokenInfo.GetTables()
		}
		objectRequest.Data["user_id_from_token"] = tokenInfo.GetUserId()
		objectRequest.Data["role_id_from_token"] = tokenInfo.GetRoleId()
		objectRequest.Data["client_type_id_from_token"] = tokenInfo.GetClientTypeId()
	}

	objectRequest.Data["language_setting"] = c.DefaultQuery("language_setting", "")
	objectRequest.Data["with_relations"] = true

	if objectRequest.Data["view_type"] != "CALENDAR" {
		if _, ok := objectRequest.Data["limit"]; ok {
			if cast.ToInt(objectRequest.Data["limit"]) > 40 {
				objectRequest.Data["limit"] = 40
			}
		} else {
			objectRequest.Data["limit"] = 10
		}
	} else {
		objectRequest.Data["limit"] = 50
	}

	structData, err := helper.ConvertMapToStruct(objectRequest.Data)
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
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
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

	redisKey := fmt.Appendf(nil, "%s-%s-%s", tableSlug, structData.String(), resource.ResourceEnvironmentId)

	if viewId, ok := objectRequest.Data["builder_service_view_id"].(string); ok {
		if util.IsValidUUID(viewId) {
			switch resource.ResourceType {
			case pb.ResourceType_MONGODB:
				service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()
				resp, err = service.GroupByColumns(
					c.Request.Context(),
					&obs.CommonMessage{
						TableSlug: tableSlug,
						Data:      structData,
						ProjectId: resource.ResourceEnvironmentId,
					},
				)
				if err != nil {
					h.HandleResponse(c, status_http.GRPCError, err.Error())
					return
				}
			case pb.ResourceType_POSTGRESQL:
				// Does Not Implemented
				h.HandleResponse(c, status_http.BadRequest, "does not implemented")
				return
			}
		}
	} else {
		switch resource.ResourceType {
		case pb.ResourceType_MONGODB:
			redisResp, err := h.redis.Get(c.Request.Context(), string(redisKey), projectId.(string), resource.NodeType)
			if err == nil {
				var (
					resp = make(map[string]any)
					m    = make(map[string]any)
				)

				if err = json.Unmarshal([]byte(redisResp), &m); err != nil {
					h.log.Error("Error while unmarshal redis", logger.Error(err))
				} else {
					resp["data"] = m
					h.HandleResponse(c, status_http.OK, resp)
					return
				}
			}
			service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()
			resp, err = service.GetList(
				c.Request.Context(),
				&obs.CommonMessage{
					TableSlug: tableSlug,
					Data:      structData,
					ProjectId: resource.ResourceEnvironmentId,
				},
			)
			if err == nil {
				if resp.IsCached {
					jsonData, _ := resp.GetData().MarshalJSON()
					err = h.redis.SetX(c.Request.Context(), string(redisKey), string(jsonData), 15*time.Second, projectId.(string), resource.NodeType)
					if err != nil {
						h.log.Error("Error while setting redis", logger.Error(err))
					}
				}
			}
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
		case pb.ResourceType_POSTGRESQL:
			resp, err := services.GoObjectBuilderService().ObjectBuilder().GetAll(
				c.Request.Context(),
				&nb.CommonMessage{
					TableSlug: tableSlug,
					Data:      structData,
					ProjectId: resource.ResourceEnvironmentId,
				},
			)

			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
			statusHttp.CustomMessage = resp.GetCustomMessage()
			h.HandleResponse(c, statusHttp, resp)
			return
		}
	}

	statusHttp.CustomMessage = resp.GetCustomMessage()
	h.HandleResponse(c, statusHttp, resp)
}

// GetListSlim godoc
// @Security ApiKeyAuth
// @ID get_list_objects_slim
// @Router /v1/object-slim/get-list/{collection} [GET]
// @Summary Get all objects slim
// @Description Get all objects slim
// @Tags Object
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param limit query number false "limit"
// @Param offset query number false "offset"
// @Param data query string false "data"
// @Success 200 {object} status_http.Response{data=models.CommonMessage} "ObjectBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetListSlim(c *gin.Context) {
	var (
		objectRequest models.CommonMessage
		queryData     string
		statusHttp    = status_http.GrpcStatusToHTTP["Ok"]
		hashed        bool
		tableSlug     = c.Param("collection")
	)

	queryParams := c.Request.URL.Query()
	if ok := queryParams.Has("data"); ok {
		queryData = queryParams.Get("data")
	}

	if ok := queryParams.Has("data"); ok {
		hashData, err := security.Decrypt(queryParams.Get("data"), h.baseConf.SecretKey)
		if err == nil {
			queryData = strings.TrimSpace(hashData)
			hashed = true
		}
	}

	queryMap := make(map[string]any)
	if err := json.Unmarshal([]byte(queryData), &queryMap); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	limit, err := h.getLimitParam(c)
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

	if projectId == "42ab0799-deff-4f8c-bf3f-64bf9665d304" { // rizoTaxi
		limit = 100
	} else {
		if limit > 40 {
			limit = 40
		}
	}

	queryMap["limit"] = limit
	queryMap["offset"] = offset

	objectRequest.Data = queryMap
	tokenInfo, err := h.GetAuthInfo(c)
	if err != nil {
		h.HandleResponse(c, status_http.Forbidden, err.Error())
		return
	}

	objectRequest.Data["tables"] = tokenInfo.GetTables()
	objectRequest.Data["user_id_from_token"] = tokenInfo.GetUserId()
	objectRequest.Data["role_id_from_token"] = tokenInfo.GetRoleId()
	objectRequest.Data["client_type_id_from_token"] = tokenInfo.GetClientTypeId()
	structData, err := helper.ConvertMapToStruct(objectRequest.Data)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	userId, _ := c.Get("user_id")
	apiKey := c.GetHeader("X-API-KEY")

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
				ServiceType:   pb.ServiceType_BUILDER_SERVICE,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}
	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   http.MethodGet,
			UserInfo:     cast.ToString(userId),
			Request:      &structData,
			ApiKey:       apiKey,
			Type:         "API_KEY",
			TableSlug:    tableSlug,
		}
	)

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()
		var slimKey = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("slim-%s-%s-%s", tableSlug, structData.String(), resource.ResourceEnvironmentId)))
		if !cast.ToBool(c.Query("block_cached")) {
			if cast.ToBool(c.Query("is_wait_cached")) {
				var slimWaitKey = config.CACHE_WAIT + "-slim"
				_, slimOK := h.cache.Get(slimWaitKey)
				if !slimOK {
					h.cache.Add(slimWaitKey, []byte(slimWaitKey), 15*time.Second)
				}

				if slimOK {
					ctx, cancel := context.WithTimeout(c.Request.Context(), config.REDIS_WAIT_TIMEOUT)
					defer cancel()

					for {
						slimBody, ok := h.cache.Get(slimKey)
						if ok {
							m := make(map[string]any)
							err = json.Unmarshal(slimBody, &m)
							if err != nil {
								h.HandleResponse(c, status_http.GRPCError, err.Error())
								return
							}

							h.HandleResponse(c, status_http.OK, map[string]any{"data": m})
							return
						}

						if ctx.Err() == context.DeadlineExceeded {
							break
						}

						time.Sleep(config.REDIS_SLEEP)
					}
				}
			} else {
				redisResp, err := h.redis.Get(c.Request.Context(), slimKey, projectId.(string), resource.NodeType)
				if err == nil {
					resp := make(map[string]any)
					m := make(map[string]any)
					err = json.Unmarshal([]byte(redisResp), &m)
					if err != nil {
						h.log.Error("Error while unmarshal redis", logger.Error(err))
					} else {
						resp["data"] = m
						h.HandleResponse(c, status_http.OK, resp)
						logReq.Response = m
						go h.versionHistory(logReq)
						return
					}
				} else {
					h.log.Error("Error while getting redis while get list ", logger.Error(err))
				}
			}
		}

		resp, err := service.GetListSlimV2(
			c.Request.Context(),
			&obs.CommonMessage{
				TableSlug: tableSlug,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			statusHttp = status_http.GrpcStatusToHTTP["Internal"]
			stat, ok := status.FromError(err)
			if ok {
				statusHttp = status_http.GrpcStatusToHTTP[stat.Code().String()]
				statusHttp.CustomMessage = stat.Message()
			}
			logReq.Response = err.Error()
			go h.versionHistory(logReq)
			h.HandleResponse(c, statusHttp, err.Error())
			return
		}

		logReq.Response = resp
		go h.versionHistory(logReq)

		if !cast.ToBool(c.Query("block_cached")) {
			jsonData, _ := resp.GetData().MarshalJSON()
			if cast.ToBool(c.Query("is_wait_cached")) {
				h.cache.Add(slimKey, jsonData, 15*time.Second)
			} else if resp.IsCached {
				err = h.redis.SetX(c.Request.Context(), slimKey, string(jsonData), 15*time.Second, projectId.(string), resource.NodeType)
				if err != nil {
					h.log.Error("Error while setting redis", logger.Error(err))
				}
			}
		}

		statusHttp.CustomMessage = resp.GetCustomMessage()

		if hashed {
			hash, err := security.Encrypt(resp, h.baseConf.SecretKey)
			if err != nil {
				h.HandleResponse(c, status_http.InternalServerError, err.Error())
				return
			}

			h.HandleResponse(c, statusHttp, hash)
			return
		}
		h.HandleResponse(c, statusHttp, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().ObjectBuilder().GetListSlim(
			c.Request.Context(), &nb.CommonMessage{
				TableSlug: tableSlug,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			statusHttp = status_http.GrpcStatusToHTTP["Internal"]
			stat, ok := status.FromError(err)
			if ok {
				statusHttp = status_http.GrpcStatusToHTTP[stat.Code().String()]
				statusHttp.CustomMessage = stat.Message()
			}
			logReq.Response = err.Error()
			go h.versionHistoryGo(c, logReq)
			h.HandleResponse(c, statusHttp, err.Error())
			return
		}

		logReq.Response = resp
		go h.versionHistoryGo(c, logReq)

		statusHttp.CustomMessage = resp.GetCustomMessage()
		h.HandleResponse(c, statusHttp, resp)
	}
}

// GetListInExcel godoc
// @Security ApiKeyAuth
// @ID get_list_objects_in_excel
// @Router /v1/object/excel/{collection} [POST]
// @Summary Get all objects in excel
// @Description Get all objects in excel
// @Tags Object
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param object body models.CommonMessage true "GetListObjectRequestBody"
// @Success 200 {object} status_http.Response{data=models.CommonMessage} "ObjectBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetListInExcel(c *gin.Context) {
	var (
		objectRequest models.CommonMessage
		statusHttp    = status_http.GrpcStatusToHTTP["Ok"]
		tableSlug     = c.Param("collection")
	)

	if err := c.ShouldBindJSON(&objectRequest); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	tokenInfo, err := h.GetAuthInfo(c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	if tokenInfo != nil {
		if tokenInfo.Tables != nil {
			objectRequest.Data["tables"] = tokenInfo.GetTables()
		}
		objectRequest.Data["user_id_from_token"] = tokenInfo.GetUserId()
		objectRequest.Data["role_id_from_token"] = tokenInfo.GetRoleId()
		objectRequest.Data["client_type_id_from_token"] = tokenInfo.GetClientTypeId()
	}

	if _, hasOffset := objectRequest.Data["utc_offset"]; !hasOffset {
		objectRequest.Data["utc_offset"] = float64(5)
	}

	structData, err := helper.ConvertMapToStruct(objectRequest.Data)
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
		err = errors.New("error getting environment id | not valid")
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	userId, _ := c.Get("user_id")

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
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

	var logReq = &models.CreateVersionHistoryRequest{
		Services:     services,
		NodeType:     resource.NodeType,
		ProjectId:    resource.ResourceEnvironmentId,
		ActionSource: "EXCEL",
		ActionType:   "EXPORT EXCEL",
		UserInfo:     cast.ToString(userId),
		Request:      &objectRequest,
		TableSlug:    tableSlug,
	}

	defer func() {
		go h.versionHistoryGo(c, logReq)
	}()

	service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := service.GetListInExcel(
			c.Request.Context(),
			&obs.CommonMessage{
				TableSlug: tableSlug,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			logReq.Response = err.Error()

			statusHttp = status_http.GrpcStatusToHTTP["Internal"]
			stat, ok := status.FromError(err)
			if ok {
				statusHttp = status_http.GrpcStatusToHTTP[stat.Code().String()]
				statusHttp.CustomMessage = stat.Message()
			}
			h.HandleResponse(c, statusHttp, err.Error())
			return
		}

		logReq.Response = resp
		statusHttp.CustomMessage = resp.GetCustomMessage()
		h.HandleResponse(c, statusHttp, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().ObjectBuilder().GetListInExcel(
			c.Request.Context(),
			&nb.CommonMessage{
				TableSlug: tableSlug,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		logReq.Response = resp
		statusHttp.CustomMessage = resp.GetCustomMessage()
		h.HandleResponse(c, statusHttp, resp)
	}

}

// DeleteManyToMany godoc
// @Security ApiKeyAuth
// @ID delete_many2many
// @Router /v1/many-to-many [DELETE]
// @Summary Delete Many2Many
// @Description Delete Many2Many
// @Tags Object
// @Accept json
// @Produce json
// @Param object body obs.ManyToManyMessage true "DeleteManyToManyBody"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) DeleteManyToMany(c *gin.Context) {
	var (
		m2mMessage                  obs.ManyToManyMessage
		resp                        *obs.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["NoContent"]
		tableSlug                   = c.Param("collection")
	)

	if err := c.ShouldBindJSON(&m2mMessage); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
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
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
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

	service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()

	m2mMessage.ProjectId = resource.ResourceEnvironmentId
	fromOfs := c.Query("from-ofs")
	if fromOfs != "true" {
		beforeActions, afterActions, err = hHelper.GetListCustomEvents(models.GetListCustomEventsStruct{
			TableSlug: tableSlug,
			RoleId:    "",
			Method:    "DELETE_MANY2MANY",
			Resource:  resource,
		},
			c,
			h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}
	}

	if len(beforeActions) > 0 {
		functionName, err := hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: beforeActions,
			IDs:          []string{m2mMessage.IdFrom},
			TableSlug:    m2mMessage.TableTo,
			ObjectData:   map[string]any{"id_to": m2mMessage.IdTo, "table_to": m2mMessage.TableFrom},
			Method:       "DELETE_MANY2MANY",
			Resource:     resource,
		},
			c,
			h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = service.ManyToManyDelete(
			c.Request.Context(), &m2mMessage,
		)
		if err != nil {
			statusHttp = status_http.GrpcStatusToHTTP["Internal"]
			stat, ok := status.FromError(err)
			if ok {
				statusHttp = status_http.GrpcStatusToHTTP[stat.Code().String()]
				statusHttp.CustomMessage = stat.Message()
			}
			h.HandleResponse(c, statusHttp, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		// Does Not Implemented
		h.HandleResponse(c, status_http.BadRequest, "does not implemented")
		return
	}

	if len(afterActions) > 0 {
		functionName, err := hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: afterActions,
			IDs:          []string{m2mMessage.IdFrom},
			TableSlug:    tableSlug,
			ObjectData:   map[string]any{"id_to": m2mMessage.IdTo, "table_from": m2mMessage.TableTo},
			Method:       "DELETE_MANY2MANY",
			Resource:     resource,
		},
			c, // gin context,
			h, // handler
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}
	statusHttp.CustomMessage = resp.GetCustomMessage()
	h.HandleResponse(c, status_http.NoContent, resp)
}

// AppendManyToMany godoc
// @Security ApiKeyAuth
// @ID append_many2many
// @Router /v1/many-to-many [PUT]
// @Summary Update many-to-many
// @Description Update many-to-many
// @Tags Object
// @Accept json
// @Produce json
// @Param object body obs.ManyToManyMessage true "UpdateMany2ManyRequestBody"
// @Success 200 {object} status_http.Response{data=string} "Object data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) AppendManyToMany(c *gin.Context) {
	var (
		m2mMessage                  obs.ManyToManyMessage
		resp                        *obs.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["Ok"]
		tableSlug                   = c.Param("collection")
	)

	if err := c.ShouldBindJSON(&m2mMessage); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
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
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
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

	service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()

	m2mMessage.ProjectId = resource.ResourceEnvironmentId
	fromOfs := c.Query("from-ofs")
	if fromOfs != "true" {
		beforeActions, afterActions, err = hHelper.GetListCustomEvents(models.GetListCustomEventsStruct{
			TableSlug: tableSlug,
			RoleId:    "",
			Method:    "APPEND_MANY2MANY",
			Resource:  resource,
		},
			c,
			h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}
	}

	if len(beforeActions) > 0 {
		functionName, err := hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: beforeActions,
			IDs:          []string{m2mMessage.IdFrom},
			TableSlug:    m2mMessage.TableTo,
			ObjectData:   map[string]any{"id_to": m2mMessage.IdTo, "table_from": m2mMessage.TableFrom},
			Method:       "APPEND_MANY2MANY",
			Resource:     resource,
		},
			c,
			h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = service.ManyToManyAppend(
			c.Request.Context(), &m2mMessage,
		)
		if err != nil {
			statusHttp = status_http.GrpcStatusToHTTP["Internal"]
			stat, ok := status.FromError(err)
			if ok {
				statusHttp = status_http.GrpcStatusToHTTP[stat.Code().String()]
				statusHttp.CustomMessage = stat.Message()
			}
			h.HandleResponse(c, statusHttp, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		// Does Not Implemented
		h.HandleResponse(c, status_http.BadRequest, "does not implemented")
		return
	}

	if len(afterActions) > 0 {
		functionName, err := hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: afterActions,
			IDs:          []string{m2mMessage.IdFrom},
			TableSlug:    m2mMessage.TableTo,
			ObjectData:   map[string]any{"id_to": m2mMessage.IdTo, "table_from": m2mMessage.TableFrom},
			Method:       "APPEND_MANY2MANY",
			Resource:     resource,
		},
			c, // gin context,
			h, // handler
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	statusHttp.CustomMessage = resp.GetCustomMessage()
	h.HandleResponse(c, statusHttp, resp)
}

// UpsertObject godoc
// @Security ApiKeyAuth
// @ID upsert_object
// @Router /v1/object-upsert/{collection} [POST]
// @Summary Upsert object
// @Description Upsert object
// @Tags Object
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param object body models.UpsertCommonMessage true "CreateObjectRequestBody"
// @Success 201 {object} status_http.Response{data=models.CommonMessage} "Object data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpsertObject(c *gin.Context) {
	var (
		objectRequest               models.UpsertCommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["Created"]
		tableSlug                   = c.Param("collection")
	)

	if err := c.ShouldBindJSON(&objectRequest); err != nil {
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
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
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

	service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()

	objects := objectRequest.Data["objects"].([]any)
	editedObjects := make([]any, 0, len(objects))
	var objectIds = make([]string, 0, len(objects))
	for _, object := range objects {
		newObject := object.(map[string]any)
		if newObject["guid"] == nil || newObject["guid"] == "" {
			guid, _ := uuid.NewRandom()
			newObject["guid"] = guid.String()
			newObject["is_new"] = true
		}
		objectIds = append(objectIds, newObject["guid"].(string))
		editedObjects = append(editedObjects, newObject)
	}

	objectRequest.Data["objects"] = editedObjects
	fromOfs := c.Query("from-ofs")
	if fromOfs != "true" {
		beforeActions, afterActions, err = hHelper.GetListCustomEvents(models.GetListCustomEventsStruct{
			TableSlug: tableSlug,
			RoleId:    "",
			Method:    "MULTIPLE_UPDATE",
			Resource:  resource,
		},
			c,
			h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}
	}
	if len(beforeActions) > 0 {
		functionName, err := hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: beforeActions,
			IDs:          []string{},
			TableSlug:    tableSlug,
			ObjectData:   objectRequest.Data,
			Method:       "MULTIPLE_UPDATE",
			Resource:     resource,
		},
			c,
			h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	// THIS for loop is written to create child objects (right now it is used in the case of One2One relation)
	for key, value := range objectRequest.Data {
		if key[0] == '$' {

			interfaceToMap := value.(map[string]any)

			id, _ := uuid.NewRandom()
			interfaceToMap["guid"] = id

			_, err := helper.ConvertMapToStruct(interfaceToMap)
			if err != nil {
				h.HandleResponse(c, status_http.InvalidArgument, err.Error())
				return
			}

			objectRequest.Data[key[1:]+"_id"] = id
		}
	}

	structData, err := helper.ConvertMapToStruct(objectRequest.Data)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}
	var resp *obs.CommonMessage
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = service.Batch(
			c.Request.Context(), &obs.BatchRequest{
				TableSlug:     tableSlug,
				Data:          structData,
				UpdatedFields: objectRequest.UpdatedFields,
				ProjectId:     resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			statusHttp = status_http.GrpcStatusToHTTP["Internal"]
			stat, ok := status.FromError(err)
			if ok {
				statusHttp = status_http.GrpcStatusToHTTP[stat.Code().String()]
				statusHttp.CustomMessage = stat.Message()
			}
			h.HandleResponse(c, statusHttp, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		// Does Not Implemented
		h.HandleResponse(c, status_http.BadRequest, "does not implemented")
		return
	}

	if tableSlug == "record_permission" {
		roleId := objectRequest.Data["objects"].([]any)[0].(map[string]any)["role_id"]
		if roleId == nil {
			err := errors.New("role id must be have in upsert permission")
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}

		service, conn, err := h.authService.Session(c)
		if err != nil {
			h.HandleResponse(c, status_http.BadEnvironment, err.Error())
			return
		}
		defer conn.Close()

		_, err = service.UpdateSessionsByRoleId(
			c.Request.Context(),
			&pba.UpdateSessionByRoleIdRequest{
				RoleId:    objectRequest.Data["role_id"].(string),
				IsChanged: true,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}

	if len(afterActions) > 0 {
		functionName, err := hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: afterActions,
			IDs:          objectIds,
			TableSlug:    tableSlug,
			ObjectData:   objectRequest.Data,
			Method:       "MULTIPLE_UPDATE",
			Resource:     resource,
		},
			c, // gin context
			h, // handler
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	statusHttp.CustomMessage = resp.GetCustomMessage()
	h.HandleResponse(c, status_http.Created, resp)
}

// MultipleUpdateObject godoc
// @Security ApiKeyAuth
// @ID multiple_update_object
// @Router /v1/object/multiple-update/{collection} [PUT]
// @Summary Multiple Update object
// @Description Multiple Update object
// @Tags Object
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param object body models.CommonMessage true "MultipleUpdateObjectRequestBody"
// @Success 201 {object} status_http.Response{data=models.CommonMessage} "Object data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) MultipleUpdateObject(c *gin.Context) {
	var (
		objectRequest               models.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["Created"]
		resource                    *pb.ServiceResourceModel
		resourceList                *pb.GetResourceByEnvIDResponse
		resp                        *obs.CommonMessage
		err                         error
		tableSlug                   = c.Param("collection")
	)

	if err = c.ShouldBindJSON(&objectRequest); err != nil {
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

	if resourceBody, ok := c.Get("resource"); ok {
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
		resource, err = h.companyServices.ServiceResource().GetSingle(c.Request.Context(),
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
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()

	objects, ok := objectRequest.Data["objects"].([]any)
	if !ok {
		err = errors.New("objects is not an array")
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	editedObjects := make([]map[string]any, 0, len(objects))
	objectIds := make([]string, 0, len(objects))

	for _, object := range objects {
		newObjects := object.(map[string]any)

		if _, ok := newObjects["guid"].(string); !ok {
			newObjects["guid"] = uuid.NewString()
			newObjects["is_new"] = true
		}

		newObjects["company_service_project_id"] = resource.GetProjectId()
		newObjects["company_service_environment_id"] = resource.GetEnvironmentId()

		objectIds = append(objectIds, newObjects["guid"].(string))
		editedObjects = append(editedObjects, newObjects)
	}

	structData, err := helper.ConvertMapToStruct(objectRequest.Data)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}
	objectRequest.Data["objects"] = editedObjects

	fromOfs := c.Query("from-ofs")
	if fromOfs != "true" {
		beforeActions, afterActions, err = hHelper.GetListCustomEvents(models.GetListCustomEventsStruct{
			TableSlug: tableSlug,
			RoleId:    "",
			Method:    "MULTIPLE_UPDATE",
			Resource:  resource,
		},
			c,
			h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}
	}

	if len(beforeActions) > 0 {
		functionName, err := hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: beforeActions,
			IDs:          objectIds,
			TableSlug:    tableSlug,
			ObjectData:   objectRequest.Data,
			Method:       "MULTIPLE_UPDATE",
			Resource:     resource,
		}, c, h)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = service.MultipleUpdate(
			c.Request.Context(), &obs.CommonMessage{
				TableSlug:        tableSlug,
				Data:             structData,
				ProjectId:        resource.ResourceEnvironmentId,
				BlockedBuilder:   cast.ToBool(c.DefaultQuery("block_builder", "false")),
				EnvId:            resource.EnvironmentId,
				CompanyProjectId: resource.ProjectId,
			},
		)
		if err != nil {
			statusHttp = status_http.GrpcStatusToHTTP["Internal"]
			if stat, ok := status.FromError(err); ok {
				statusHttp = status_http.GrpcStatusToHTTP[stat.Code().String()]
				statusHttp.CustomMessage = stat.Message()
			}
			h.HandleResponse(c, statusHttp, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		goResp, err := services.GoObjectBuilderService().Items().MultipleUpdate(
			c.Request.Context(), &nb.CommonMessage{
				TableSlug:        tableSlug,
				Data:             structData,
				ProjectId:        resource.ResourceEnvironmentId,
				EnvId:            resource.EnvironmentId,
				CompanyProjectId: resource.ProjectId,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		if err = helper.MarshalToStruct(goResp, &resp); err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}
	}

	if len(afterActions) > 0 {
		functionName, err := hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: afterActions,
			IDs:          objectIds,
			TableSlug:    tableSlug,
			ObjectData:   objectRequest.Data,
			Method:       "MULTIPLE_UPDATE",
			Resource:     resource,
		},
			c,
			h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	statusHttp.CustomMessage = resp.GetCustomMessage()
	h.HandleResponse(c, status_http.Created, resp)
}

// DeleteManyObject godoc
// @Security ApiKeyAuth
// @ID delete_many_object
// @Router /v1/object/{collection} [DELETE]
// @Summary Delete many objects
// @Description Delete many objects
// @Tags Object
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param object body models.Ids true "DeleteManyObjectRequestBody"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) DeleteManyObject(c *gin.Context) {
	var (
		objectRequest               models.Ids
		resp                        *obs.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["NoContent"]
		data                        = make(map[string]any)
		tableSlug                   = c.Param("collection")
	)

	if err := c.ShouldBindJSON(&objectRequest); err != nil {
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

	resource, _ := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)

	data["company_service_project_id"] = projectId.(string)
	data["company_service_environment_id"] = environmentId.(string)
	data["ids"] = objectRequest.Ids

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()

	structData, err := helper.ConvertMapToStruct(data)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	fromOfs := c.Query("from-ofs")
	if fromOfs != "true" {
		beforeActions, afterActions, err = hHelper.GetListCustomEvents(models.GetListCustomEventsStruct{
			TableSlug: tableSlug,
			RoleId:    "",
			Method:    "DELETE_MANY",
			Resource:  resource,
		},
			c,
			h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}
	}

	if len(beforeActions) > 0 {
		functionName, err := hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: beforeActions,
			IDs:          objectRequest.Ids,
			TableSlug:    tableSlug,
			ObjectData:   data,
			Method:       "DELETE_MANY",
			Resource:     resource,
		},
			c,
			h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = service.DeleteMany(
			c.Request.Context(),
			&obs.CommonMessage{
				TableSlug: tableSlug,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			statusHttp = status_http.GrpcStatusToHTTP["Internal"]
			stat, ok := status.FromError(err)
			if ok {
				statusHttp = status_http.GrpcStatusToHTTP[stat.Code().String()]
				statusHttp.CustomMessage = stat.Message()
			}
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		goResp, err := services.GoObjectBuilderService().Items().DeleteMany(
			c.Request.Context(),
			&nb.CommonMessage{
				TableSlug: tableSlug,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		body, err := json.Marshal(goResp)
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}

		if err := json.Unmarshal(body, &resp); err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}
	}

	if len(afterActions) > 0 {
		functionName, err := hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: afterActions,
			IDs:          objectRequest.Ids,
			TableSlug:    tableSlug,
			ObjectData:   data,
			Method:       "DELETE_MANY",
			Resource:     resource,
		},
			c, // gin context,
			h, // handler
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	statusHttp.CustomMessage = resp.GetCustomMessage()
	h.HandleResponse(c, statusHttp, resp)
}

// GetListAggregate godoc
// @Security ApiKeyAuth
// @ID get_list_aggregate
// @Router /v1/object/get-list-aggregate/{collection} [POST]
// @Summary Get List Aggregate
// @Description Get List Aggregate
// @Tags Object
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param limit query string false "limit"
// @Param offset query string false "offset"
// @Param sort_type query string false "sort_type"
// @Param object body models.CommonMessage true "GetGroupByFieldObjectRequestBody"
// @Success 200 {object} status_http.Response{data=models.CommonMessage} "ObjectBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetListAggregate(c *gin.Context) {
	var (
		objectRequest models.CommonMessage
		resp          *obs.CommonMessage
		err           error
		tableSlug     = c.Param("collection")
	)

	if err = c.ShouldBindJSON(&objectRequest); err != nil {
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

	var resource *pb.ServiceResourceModel
	resourceBody, ok := c.Get("resource")
	if resourceBody != "" && ok {
		var resourceList *pb.GetResourceByEnvIDResponse
		err = json.Unmarshal([]byte(resourceBody.(string)), &resourceList)
		if err != nil {
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
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()

	var (
		object    models.CommonMessage
		fieldResp = &obs.GetAllFieldsResponse{}
	)

	if len(cast.ToSlice(objectRequest.Data["group_selects"])) <= 0 || len(cast.ToSlice(objectRequest.Data["projects"])) <= 0 {
		var (
			fieldKey     = fmt.Sprintf("%s-%s-%s", tableSlug, projectId.(string), environmentId.(string))
			fieldKeyWait = config.CACHE_WAIT + "-field"
		)

		_, fieldOK := h.cache.Get(fieldKeyWait)
		if !fieldOK {
			h.cache.Add(fieldKeyWait, []byte(fieldKeyWait), config.REDIS_KEY_TIMEOUT)
		}

		if fieldOK {
			ctx, cancel := context.WithTimeout(c.Request.Context(), config.REDIS_WAIT_TIMEOUT)
			defer cancel()
			for {
				fieldBody, ok := h.cache.Get(fieldKey)
				if ok {
					err = json.Unmarshal(fieldBody, &fieldResp)
					if err != nil {
						h.HandleResponse(c, status_http.BadRequest, "cant get auth info")
						c.Abort()
						return
					}
				}

				if len(fieldResp.Fields) >= 0 {
					break
				}

				if ctx.Err() == context.DeadlineExceeded {
					break
				}

				time.Sleep(config.REDIS_SLEEP)
			}
		}

		if len(fieldResp.Fields) <= 0 {
			fieldResp, err = services.GetBuilderServiceByType(resource.NodeType).Field().GetAll(c.Request.Context(), &obs.GetAllFieldsRequest{
				TableSlug: tableSlug,
				ProjectId: resource.ResourceEnvironmentId,
			})
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}

			go func() {
				body, err := json.Marshal(fieldResp)
				if err != nil {
					h.HandleResponse(c, status_http.GRPCError, err.Error())
					return
				}
				h.cache.Add(fieldKey, body, config.REDIS_TIMEOUT)
			}()
		}
	}

	object.Data = map[string]any{
		"match": map[string]any{"$match": map[string]any{}},
		"sort":  map[string]any{"$sort": map[string]any{"_id": 1}},
	}

	if c.Query("limit") != "" {
		object.Data["limit"] = cast.ToInt(c.Query("limit"))
	}

	if c.Query("offset") != "" {
		object.Data["offset"] = cast.ToInt(c.Query("offset"))
	}

	if _, ok := objectRequest.Data["search"]; ok {
		var (
			searchQuery = map[string]any{}
			search      = cast.ToStringMap(objectRequest.Data["search"])
		)

		for key, value := range search {
			if cast.ToString(value) == "" {
				searchQuery[key] = value
			} else {
				searchQuery[key] = map[string]any{"$regex": value, "$options": "i"}
			}
		}
		object.Data["second_match"] = searchQuery
	}

	if _, ok := objectRequest.Data["match"]; ok {
		var (
			matchQuery = map[string]any{}
			match      = cast.ToStringMap(objectRequest.Data["match"])
		)
		for key, value := range match {
			matchQuery[key] = value
		}
		object.Data["match"] = map[string]any{"$match": matchQuery}
	}

	var groupQuery = map[string]any{"_id": "$guid"}
	if _, ok := objectRequest.Data["groups"]; ok {
		var (
			groupSlice = cast.ToStringSlice(objectRequest.Data["groups"])
			groupLen   = len(groupSlice)
		)
		if groupLen > 0 {
			if groupLen > 1 {
				var manyGroup = map[string]any{}
				for _, value := range groupSlice {
					manyGroup[value] = "$" + value
				}
				groupQuery = map[string]any{"_id": manyGroup}
			} else {
				groupQuery = map[string]any{"_id": "$" + groupSlice[0]}
			}
		}
	}
	groupQuery["guid"] = map[string]any{"$first": "$guid"}

	if _, ok := objectRequest.Data["group_selects"]; ok {
		var groupSelects = cast.ToStringSlice(objectRequest.Data["group_selects"])
		for _, value := range groupSelects {
			groupQuery[value] = map[string]any{"$first": "$" + value}
		}
	} else {
		for _, field := range fieldResp.Fields {
			groupQuery[field.Slug] = map[string]any{"$first": "$" + field.Slug}
		}
	}

	if _, ok := objectRequest.Data["group_query"]; ok {
		var groupQueryRequest = cast.ToStringMap(objectRequest.Data["group_query"])
		for key, value := range groupQueryRequest {
			groupQuery[key] = value
		}
	}
	object.Data["query"] = map[string]any{"$group": groupQuery}

	if _, ok := objectRequest.Data["lookups"]; ok {
		var (
			lookups      = cast.ToSlice(objectRequest.Data["lookups"])
			lookupsQuery = []any{}
		)
		for _, objectLookup := range lookups {
			var (
				lookupMap     = cast.ToStringMap(objectLookup)
				fromSlug      = cast.ToString(lookupMap["from"])
				lastCharacter string
			)

			if len(fromSlug) > 0 {
				lastCharacter = string(fromSlug[len(fromSlug)-1])
			}

			if lastCharacter != "s" {
				if lastCharacter == "y" {
					fromSlug = fromSlug[:len(fromSlug)-1]
					fromSlug += "ies"
				} else {
					fromSlug += "s"
				}
			}

			lookupQuery := map[string]any{
				"from":         fromSlug,
				"foreignField": lookupMap["from_field"],
				"localField":   lookupMap["to_field"],
				"as":           lookupMap["as"],
			}

			if len(cast.ToSlice(lookupMap["pipeline"])) > 0 {
				lookupQuery["pipeline"] = cast.ToSlice(lookupMap["pipeline"])
			}

			lookupsQuery = append(lookupsQuery, map[string]any{"$lookup": lookupQuery})
		}
		object.Data["lookups"] = lookupsQuery
	}

	var projectQuery = map[string]any{"_id": 0, "guid": 1}
	if _, ok := objectRequest.Data["projects"]; ok {
		var projects = cast.ToStringSlice(objectRequest.Data["projects"])
		for _, value := range projects {
			if strings.Contains(value, ".") {
				var key = strings.ReplaceAll(value, ".", "_")
				projectQuery[key] = map[string]any{"$first": "$" + value}
			} else {
				projectQuery[value] = 1
			}
		}
	} else {
		for _, field := range fieldResp.Fields {
			projectQuery[field.Slug] = 1
		}
	}

	if _, ok := objectRequest.Data["project_query"]; ok {
		var projectQueries = cast.ToStringMap(objectRequest.Data["project_query"])
		for key, value := range projectQueries {
			projectQuery[key] = value
		}
	}
	object.Data["project"] = map[string]any{"$project": projectQuery}

	if _, ok := objectRequest.Data["sorts"]; ok {
		var (
			sorts      = cast.ToStringSlice(objectRequest.Data["sorts"])
			sortQuery  = map[string]any{}
			sortedType = 1
		)

		if len(c.Query("sort_type")) > 0 {
			switch c.Query("sort_type") {
			case "desc":
				sortedType = -1
			case "asc":
				sortedType = 1
			}
		}

		for _, value := range sorts {
			sortQuery[value] = sortedType
		}
		object.Data["sort"] = map[string]any{"$sort": sortQuery}
	}

	structData, err := helper.ConvertMapToStruct(object.Data)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	var (
		key              = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("aggregate-%s-%s-%s", tableSlug, structData.String(), resource.ResourceEnvironmentId)))
		aggregateWaitKey = config.CACHE_WAIT + "-aggregate"
	)

	if !cast.ToBool(c.Query("block_cached")) {
		_, aggregateOk := h.cache.Get(aggregateWaitKey)
		if !aggregateOk {
			h.cache.Add(aggregateWaitKey, []byte(aggregateWaitKey), 20*time.Second)
		}

		if aggregateOk {
			ctx, cancel := context.WithTimeout(c.Request.Context(), config.REDIS_WAIT_TIMEOUT)
			defer cancel()

			for {
				aggregateBody, ok := h.cache.Get(key)
				if ok {
					m := make(map[string]any)
					err = json.Unmarshal(aggregateBody, &m)
					if err != nil {
						h.HandleResponse(c, status_http.GRPCError, err.Error())
						return
					}
					resp := map[string]any{"data": m}
					h.HandleResponse(c, status_http.OK, resp)
					return
				}

				if ctx.Err() == context.DeadlineExceeded {
					break
				}

				time.Sleep(config.REDIS_SLEEP)
			}
		}
	}

	resp, err = service.GetGroupByField(
		c.Request.Context(),
		&obs.CommonMessage{
			TableSlug: tableSlug,
			Data:      structData,
			ProjectId: resource.ResourceEnvironmentId,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if !cast.ToBool(c.Query("block_cached")) {
		go func() {
			jsonData, _ := json.Marshal(resp.Data)
			h.cache.Add(key, []byte(jsonData), 20*time.Second)
		}()
	}

	h.HandleResponse(c, status_http.OK, resp)
}
