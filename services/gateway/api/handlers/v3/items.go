package v3

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/billing"
	hHelper "github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/helper"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/spf13/cast"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

func (h *HandlerV3) CreateItem(c *gin.Context) {
	var (
		objectRequest               models.CommonMessage
		resp                        *obs.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["Created"]
		collection                  = c.Param("collection")
	)

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second*15)
	defer cancel()

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

	objectRequest.Data["company_service_project_id"] = resource.GetProjectId()
	objectRequest.Data["company_service_environment_id"] = resource.GetEnvironmentId()

	var id string = uuid.NewString()

	guid, ok := objectRequest.Data["guid"]
	if ok {
		if util.IsValidUUID(guid.(string)) {
			id = objectRequest.Data["guid"].(string)
		}
	}

	objectRequest.Data["guid"] = id

	structData, err := helper.ConvertMapToStruct(objectRequest.Data)

	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	fromOfs := c.Query("from-ofs")
	if fromOfs != "true" {
		beforeActions, afterActions, err = hHelper.GetListCustomEvents(models.GetListCustomEventsStruct{
			TableSlug: collection,
			RoleId:    "",
			Method:    "CREATE",
			Resource:  resource,
		},
			c, h,
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
			IDs:          []string{id},
			TableSlug:    collection,
			ObjectData:   objectRequest.Data,
			Method:       "CREATE",
			Resource:     resource,
		},
			c, h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	logReq := &models.CreateVersionHistoryRequest{
		Services:     services,
		NodeType:     resource.NodeType,
		ProjectId:    resource.ResourceEnvironmentId,
		ActionSource: c.Request.URL.String(),
		ActionType:   "CREATE ITEM",
		UserInfo:     cast.ToString(userId),
		Request:      &structData,
		TableSlug:    collection,
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder().Create(
			ctx, &obs.CommonMessage{
				TableSlug: collection,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)

		// this logic for custom error message, object builder service may be return 400, 404, 500
		if err != nil {
			statusHttp = status_http.GrpcStatusToHTTP["Internal"]
			stat, ok := status.FromError(err)
			if ok {
				statusHttp = status_http.GrpcStatusToHTTP[stat.Code().String()]
				statusHttp.CustomMessage = stat.Message()
			}
			logReq.Response = err.Error()
			defer func() { go h.versionHistory(logReq) }()
			h.HandleResponse(c, statusHttp, err.Error())
			return
		}
		logReq.Response = resp
		defer func() { go h.versionHistory(logReq) }()
	case pb.ResourceType_POSTGRESQL:
		body, err := services.GoObjectBuilderService().Items().Create(
			ctx, &nb.CommonMessage{
				TableSlug: collection,
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
			defer func() { go h.versionHistoryGo(c, logReq) }()
			h.HandleResponse(c, statusHttp, err.Error())
			return
		}

		if err = helper.MarshalToStruct(body, &resp); err != nil {
			return
		}

		logReq.Response = resp
		defer func() { go h.versionHistoryGo(c, logReq) }()
	}

	if data, ok := resp.Data.AsMap()["data"].(map[string]any); ok {
		objectRequest.Data = data
		if _, ok = data["guid"].(string); ok {
			id = data["guid"].(string)
		}
	}

	if len(afterActions) > 0 {
		functionName, err := hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			CustomEvents: afterActions,
			IDs:          []string{id},
			TableSlug:    collection,
			ObjectData:   objectRequest.Data,
			Method:       "CREATE",
			Resource:     resource,
		},
			c, h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	statusHttp.CustomMessage = resp.GetCustomMessage()
	h.HandleResponse(c, statusHttp, resp)
}

func (h *HandlerV3) CreateItems(c *gin.Context) {
	var (
		objectRequest               models.MultipleInsertItems
		resp                        *obs.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["Created"]
		collection                  = c.Param("collection")
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

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
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

	request := make(map[string]any)
	request["company_service_project_id"] = resource.GetProjectId()
	request["company_service_environment_id"] = resource.GetEnvironmentId()
	request["items"] = objectRequest

	structData, err := helper.ConvertMapToStruct(request)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	fromOfs := c.Query("from-ofs")
	if fromOfs != "true" {
		beforeActions, afterActions, err = hHelper.GetListCustomEvents(models.GetListCustomEventsStruct{
			TableSlug: collection,
			RoleId:    "",
			Method:    "CREATE_MANY",
			Resource:  resource,
		},
			c, h,
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
			TableSlug:    collection,
			ObjectData:   request,
			Method:       "CREATE_MANY",
			Resource:     resource,
		},
			c, h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	logReq := &models.CreateVersionHistoryRequest{
		Services:     services,
		NodeType:     resource.NodeType,
		ProjectId:    resource.ResourceEnvironmentId,
		ActionSource: c.Request.URL.String(),
		ActionType:   "CREATE ITEM",
		UserInfo:     cast.ToString(userId),
		Request:      structData,
		TableSlug:    collection,
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder().Create(
			c.Request.Context(),
			&obs.CommonMessage{
				TableSlug: collection,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		// this logic for custom error message, object builder service may be return 400, 404, 500
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
	case pb.ResourceType_POSTGRESQL:
		// Does Not Implemented
		h.HandleResponse(c, status_http.BadRequest, "does not implemented")
		return
	}

	var items []any
	if itemsFromResp, ok := resp.Data.AsMap()["items"].([]any); ok {
		items = itemsFromResp
	}
	var ids = make([]string, 0, len(items))
	for _, item := range items {
		if itemMap, ok := item.(map[string]any); ok {
			if id, ok := itemMap["guid"].(string); ok {
				ids = append(ids, id)
			}
		}
	}
	if len(afterActions) > 0 {
		invoke := models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: afterActions,
			IDs:          ids,
			TableSlug:    collection,
			ObjectData:   request,
			Method:       "CREATE_MANY",
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

func (h *HandlerV3) GetSingleItem(c *gin.Context) {
	var (
		object     models.CommonMessage
		statusHttp = status_http.GrpcStatusToHTTP["Ok"]
		collection = c.Param("collection")
	)

	object.Data = make(map[string]any)

	objectID := c.Param("id")
	if !util.IsValidUUID(objectID) {
		h.HandleResponse(c, status_http.InvalidArgument, "id is an invalid uuid")
		return
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

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder().GetSingle(
			c.Request.Context(),
			&obs.CommonMessage{
				TableSlug: collection,
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

		statusHttp.CustomMessage = resp.GetCustomMessage()
		h.HandleResponse(c, statusHttp, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Items().GetSingle(
			c.Request.Context(), &nb.CommonMessage{
				TableSlug: collection,
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
	}
}

func (h *HandlerV3) GetListV2(c *gin.Context) {
	var (
		objectRequest models.CommonMessage
		resp          *obs.CommonMessage
		statusHttp    = status_http.GrpcStatusToHTTP["Ok"]
	)

	tableSlug := c.Param("collection")

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

	var projectIDs = map[string]bool{
		"0f111e78-3a93-4bec-945a-2a77e0e0a82d": true, // wayll
		"25d16930-b1a9-4ae5-ab01-b79cc993f06e": true, // dasyor
		"da7ced2e-ed43-4bbe-8b5a-3b545c8e7ef0": true, // taskmanager
	}

	if objectRequest.Data["view_type"] != "CALENDAR" {
		if _, ok := objectRequest.Data["limit"]; ok {
			if cast.ToInt(objectRequest.Data["limit"]) > 40 {
				objectRequest.Data["limit"] = 40
			}
		} else {
			if !projectIDs[projectId.(string)] {
				objectRequest.Data["limit"] = 10
			}

		}
	}

	structData, err := helper.ConvertMapToStruct(objectRequest.Data)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
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

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()
	redisKey := base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "%s-%s-%s", tableSlug, structData.String(), resource.ResourceEnvironmentId))

	if viewId, ok := objectRequest.Data["builder_service_view_id"].(string); ok {
		if util.IsValidUUID(viewId) {
			switch resource.ResourceType {
			case pb.ResourceType_MONGODB:
				if resource.ResourceEnvironmentId == "49ae6c46-5397-4975-b238-320617f0190c" { // starex
					h.HandleResponse(c, statusHttp, pb.Empty{})
					return
				}

				redisResp, err := h.redis.Get(c.Request.Context(), redisKey, projectId.(string), resource.NodeType)
				if err == nil {
					resp := make(map[string]any)
					m := make(map[string]any)
					err = json.Unmarshal([]byte(redisResp), &m)
					if err != nil {
						h.log.Error("Error while unmarshal redis", logger.Error(err))
					} else {
						resp["data"] = m
						h.HandleResponse(c, status_http.OK, resp)
						return
					}
				}

				resp, err = service.GroupByColumns(c.Request.Context(),
					&obs.CommonMessage{
						TableSlug: tableSlug,
						Data:      structData,
						ProjectId: resource.ResourceEnvironmentId,
					},
				)

				if err == nil {
					if resp.IsCached {
						jsonData, _ := resp.GetData().MarshalJSON()
						err = h.redis.SetX(c.Request.Context(), redisKey, string(jsonData), 15*time.Second, projectId.(string), resource.NodeType)
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
				resp, err := services.GoObjectBuilderService().ObjectBuilder().GetGroupByField(c.Request.Context(),
					&nb.CommonMessage{
						TableSlug: tableSlug,
						Data:      structData,
						ProjectId: resource.ResourceEnvironmentId,
					},
				)

				if err != nil {
					h.handleError(c, status_http.GRPCError, err)
					return
				}

				statusHttp.CustomMessage = resp.GetCustomMessage()
				h.HandleResponse(c, statusHttp, resp)
				return
			}
		}
	} else {
		switch resource.ResourceType {
		case pb.ResourceType_MONGODB:
			redisResp, err := h.redis.Get(c.Request.Context(), redisKey, projectId.(string), resource.NodeType)
			if err == nil {
				resp := make(map[string]any)
				m := make(map[string]any)
				err = json.Unmarshal([]byte(redisResp), &m)
				if err != nil {
					h.log.Error("Error while unmarshal redis", logger.Error(err))
				} else {
					resp["data"] = m
					h.HandleResponse(c, status_http.OK, resp)
					return
				}
			}

			resp, err = service.GetList2(c.Request.Context(),
				&obs.CommonMessage{
					TableSlug: tableSlug,
					Data:      structData,
					ProjectId: resource.ResourceEnvironmentId,
				},
			)

			if err == nil {
				if resp.IsCached {
					jsonData, _ := resp.GetData().MarshalJSON()
					err = h.redis.SetX(context.Background(), redisKey, string(jsonData), 15*time.Second, projectId.(string), resource.NodeType)
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
			resp, err := services.GoObjectBuilderService().ObjectBuilder().GetList2(
				c.Request.Context(), &nb.CommonMessage{
					TableSlug: tableSlug,
					Data:      structData,
					ProjectId: resource.ResourceEnvironmentId,
				},
			)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}

			resp.ProjectId = cast.ToString(projectId)
			statusHttp.CustomMessage = resp.GetCustomMessage()
			h.HandleResponse(c, statusHttp, resp)
			return
		}
	}

	statusHttp.CustomMessage = resp.GetCustomMessage()
	h.HandleResponse(c, statusHttp, resp)
}

func (h *HandlerV3) UpdateItem(c *gin.Context) {
	var (
		objectRequest               models.CommonMessage
		resp, singleObject          *obs.CommonMessage
		body                        *nb.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["Ok"]
		actionErr                   error
		functionName                string
		id                          string
		collection                  = c.Param("collection")
	)

	if err := c.ShouldBindJSON(&objectRequest); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	if objectRequest.Data["guid"] != nil {
		id = objectRequest.Data["guid"].(string)
	} else {
		objectRequest.Data["guid"] = c.Param("id")
		id = c.Param("id")

		if id == "" {
			h.HandleResponse(c, status_http.BadRequest, "guid is required")
			return
		}
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

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		singleObject, err = services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder().GetSingleSlim(
			c.Request.Context(),
			&obs.CommonMessage{
				TableSlug: collection,
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
		single, err := services.GoObjectBuilderService().Items().GetSingle(
			c.Request.Context(), &nb.CommonMessage{
				TableSlug: collection,
				Data:      &structpb.Struct{Fields: map[string]*structpb.Value{"id": structpb.NewStringValue(id)}},
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		if err = helper.MarshalToStruct(single, &singleObject); err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}

	fromOfs := c.Query("from-ofs")
	if fromOfs != "true" {
		beforeActions, afterActions, err = hHelper.GetListCustomEvents(models.GetListCustomEventsStruct{
			TableSlug: collection,
			RoleId:    "",
			Method:    "UPDATE",
			Resource:  resource,
		},
			c, h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}
	}

	if len(beforeActions) > 0 {
		functionName, err = hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: beforeActions,
			IDs:          []string{id},
			TableSlug:    collection,
			ObjectData:   objectRequest.Data,
			Method:       "UPDATE",
			ActionType:   "BEFORE",
			Resource:     resource,
		},
			c, h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "UPDATE ITEM",
			UserInfo:     cast.ToString(userId),
			Request:      &structData,
			TableSlug:    collection,
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else if actionErr != nil {
			logReq.Response = actionErr.Error() + " in " + functionName
			h.HandleResponse(c, status_http.InvalidArgument, actionErr.Error()+" in "+functionName)
		} else {
			logReq.Response = resp
			h.HandleResponse(c, status_http.OK, resp)
		}

		switch resource.ResourceType {
		case pb.ResourceType_MONGODB:
			go h.versionHistory(logReq)
		case pb.ResourceType_POSTGRESQL:
			go h.versionHistoryGo(c, logReq)
		}
	}()

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder().Update(
			c.Request.Context(), &obs.CommonMessage{
				TableSlug:        collection,
				Data:             structData,
				ProjectId:        resource.ResourceEnvironmentId,
				EnvId:            resource.EnvironmentId,
				CompanyProjectId: resource.ProjectId,
				BlockedBuilder:   cast.ToBool(c.DefaultQuery("block_builder", "false")),
			},
		)
		if err != nil {
			statusHttp = status_http.GrpcStatusToHTTP["Internal"]
			stat, ok := status.FromError(err)
			if ok {
				statusHttp = status_http.GrpcStatusToHTTP[stat.Code().String()]
				statusHttp.CustomMessage = stat.Message()
			}
			return
		}
	case pb.ResourceType_POSTGRESQL:
		body, err = services.GoObjectBuilderService().Items().Update(
			c.Request.Context(), &nb.CommonMessage{
				TableSlug:        collection,
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
			return
		}

		if err = helper.MarshalToStruct(body, &resp); err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}

	if len(afterActions) > 0 {
		functionName, actionErr = hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:               services.GoObjectBuilderService(),
			CustomEvents:           afterActions,
			IDs:                    []string{id},
			TableSlug:              collection,
			ObjectData:             objectRequest.Data,
			Method:                 "UPDATE",
			ObjectDataBeforeUpdate: singleObject.Data.AsMap(),
			ActionType:             "AFTER",
			Resource:               resource,
		},
			c, h, // gin context,
		)
		if actionErr != nil {
			return
		}
	}
	beforeData := singleObject.GetData().AsMap()
	if nested, ok := beforeData["data"].(map[string]any); ok {
		beforeData = nested
	}
	if nested, ok := beforeData["response"].(map[string]any); ok {
		beforeData = nested
	}
	syncData := make(map[string]any, len(beforeData)+len(objectRequest.Data)+2)
	for key, value := range beforeData {
		syncData[key] = value
	}
	for key, value := range objectRequest.Data {
		syncData[key] = value
	}
	if _, ok := syncData["guid"]; !ok {
		syncData["guid"] = id
	}
	statusHttp.CustomMessage = resp.GetCustomMessage()
}

func (h *HandlerV3) MultipleUpdateItems(c *gin.Context) {
	var (
		objectRequest               models.MultipleUpdateItems
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["Created"]
		actionErr                   error
		functionName                string
		collection                  = c.Param("collection")
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

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	structData, err := helper.ConvertMapToStruct(objectRequest.Data)

	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	fromOfs := c.Query("from-ofs")
	if fromOfs != "true" {
		beforeActions, afterActions, err = hHelper.GetListCustomEvents(models.GetListCustomEventsStruct{
			TableSlug: collection,
			RoleId:    "",
			Method:    "MULTIPLE_UPDATE",
			Resource:  resource,
		},
			c, h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}
	}
	if len(beforeActions) > 0 {
		functionName, err = hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: beforeActions,
			IDs:          objectRequest.Ids,
			TableSlug:    collection,
			ObjectData:   objectRequest.Data,
			Method:       "MULTIPLE_UPDATE",
			ActionType:   "BEFORE",
			Resource:     resource,
		},
			c, h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "UPDATE ITEM",
			UserInfo:     cast.ToString(userId),
			Request:      &structData,
			TableSlug:    collection,
		}
	)

	var resp *obs.CommonMessage

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else if actionErr != nil {
			logReq.Response = actionErr.Error() + " in " + functionName
			h.HandleResponse(c, status_http.InvalidArgument, actionErr.Error()+" in "+functionName)
		} else {
			logReq.Response = resp
			h.HandleResponse(c, status_http.NoContent, resp)
		}
		switch resource.ResourceType {
		case pb.ResourceType_MONGODB:
			go h.versionHistory(logReq)
		case pb.ResourceType_POSTGRESQL:
			go h.versionHistoryGo(c, logReq)
		}
	}()

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder().MultipleUpdate(
			c.Request.Context(), &obs.CommonMessage{
				TableSlug: collection,
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
			return
		}
	case pb.ResourceType_POSTGRESQL:
		body, err := services.GoObjectBuilderService().Items().MultipleUpdate(
			c.Request.Context(), &nb.CommonMessage{
				TableSlug: collection,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		if err = helper.MarshalToStruct(body, &resp); err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}

	if len(afterActions) > 0 {
		functionName, actionErr = hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: afterActions,
			IDs:          objectRequest.Ids,
			TableSlug:    collection,
			ObjectData:   objectRequest.Data,
			Method:       "MULTIPLE_UPDATE",
			ActionType:   "AFTER",
			Resource:     resource,
		},
			c, h, // gin context
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error()+" in "+functionName)
			return
		}
	}
	statusHttp.CustomMessage = resp.GetCustomMessage()
}

func (h *HandlerV3) DeleteItem(c *gin.Context) {
	var (
		objectRequest               models.CommonMessage
		resp                        *obs.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["NoContent"]
		collection                  = c.Param("collection")
	)

	if err := c.ShouldBindJSON(&objectRequest); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	objectID := c.Param("id")
	if !util.IsValidUUID(objectID) {
		h.HandleResponse(c, status_http.InvalidArgument, "item id is an invalid uuid")
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

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	objectRequest.Data["id"] = objectID
	objectRequest.Data["company_service_project_id"] = projectId.(string)
	objectRequest.Data["company_service_environment_id"] = environmentId.(string)

	structData, err := helper.ConvertMapToStruct(objectRequest.Data)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	fromOfs := c.Query("from-ofs")
	if fromOfs != "true" {
		beforeActions, afterActions, err = hHelper.GetListCustomEvents(models.GetListCustomEventsStruct{
			TableSlug: collection,
			RoleId:    "",
			Method:    "DELETE",
			Resource:  resource,
		},
			c, h,
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
			TableSlug:    collection,
			ObjectData:   objectRequest.Data,
			Method:       "DELETE",
			Resource:     resource,
		},
			c, h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	logReq := &models.CreateVersionHistoryRequest{
		Services:     services,
		NodeType:     resource.NodeType,
		ProjectId:    resource.ResourceEnvironmentId,
		ActionSource: c.Request.URL.String(),
		ActionType:   "DELETE ITEM",
		UsedEnvironments: map[string]bool{
			cast.ToString(environmentId): true,
		},
		UserInfo:  cast.ToString(userId),
		Request:   structData,
		TableSlug: collection,
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder().Delete(
			c.Request.Context(), &obs.CommonMessage{
				TableSlug: collection,
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
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		logReq.Response = resp
		go h.versionHistory(logReq)
	case pb.ResourceType_POSTGRESQL:
		new, err := services.GoObjectBuilderService().Items().Delete(
			c.Request.Context(), &nb.CommonMessage{
				TableSlug: collection,
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
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		logReq.Response = resp
		go h.versionHistoryGo(c, logReq)

		err = helper.MarshalToStruct(new, &resp)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

	}

	if len(afterActions) > 0 {
		functionName, err := hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: afterActions,
			IDs:          []string{objectID},
			TableSlug:    collection,
			ObjectData:   objectRequest.Data,
			Method:       "DELETE",
			Resource:     resource,
		},
			c, h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	syncData := objectRequest.Data
	if resp != nil && resp.Data != nil {
		if responseData := resp.Data.AsMap(); len(responseData) > 0 {
			syncData = responseData
		}
	}
	if _, ok := syncData["guid"]; !ok {
		syncData["guid"] = objectID
	}

	statusHttp.CustomMessage = resp.GetCustomMessage()
	h.HandleResponse(c, statusHttp, resp)
}

func (h *HandlerV3) DeleteItems(c *gin.Context) {
	var (
		objectRequest               models.Ids
		resp                        *obs.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["NoContent"]
		data                        = make(map[string]any)
		actionErr                   error
		functionName                string
		collection                  = c.Param("collection")
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

	data["company_service_project_id"] = projectId.(string)
	data["company_service_environment_id"] = environmentId.(string)
	data["ids"] = objectRequest.Ids
	data["query"] = objectRequest.Query

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()
	if err != nil {
		h.log.Info("Error while getting "+resource.NodeType+" object builder service", logger.Error(err))
		h.HandleResponse(c, status_http.InternalServerError, err)
		return
	}

	structData, err := helper.ConvertMapToStruct(data)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	fromOfs := c.Query("from-ofs")
	if fromOfs != "true" {
		beforeActions, afterActions, err = hHelper.GetListCustomEvents(models.GetListCustomEventsStruct{
			TableSlug: collection,
			RoleId:    "",
			Method:    "DELETE_MANY",
			Resource:  resource,
		},
			c, h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}
	}

	if len(beforeActions) > 0 {
		functionName, err = hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			CustomEvents: beforeActions,
			IDs:          objectRequest.Ids,
			TableSlug:    collection,
			ObjectData:   data,
			Method:       "DELETE_MANY",
			Resource:     resource,
		},
			c, h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "DELETE ITEM",
			UsedEnvironments: map[string]bool{
				cast.ToString(environmentId): true,
			},
			UserInfo:  cast.ToString(userId),
			Request:   &structData,
			TableSlug: collection,
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else if actionErr != nil {
			logReq.Response = actionErr.Error() + " in " + functionName
			h.HandleResponse(c, status_http.InvalidArgument, actionErr.Error()+" in "+functionName)
		} else {
			logReq.Response = resp
			h.HandleResponse(c, status_http.NoContent, resp)
		}
		switch resource.ResourceType {
		case pb.ResourceType_MONGODB:
			go h.versionHistory(logReq)
		case pb.ResourceType_POSTGRESQL:
			go h.versionHistoryGo(c, logReq)
		}
	}()

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = service.DeleteMany(
			c.Request.Context(), &obs.CommonMessage{
				TableSlug: collection,
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
		_, err = services.GoObjectBuilderService().Items().DeleteMany(
			c.Request.Context(), &nb.CommonMessage{
				TableSlug: collection,
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
	}

	if len(afterActions) > 0 {
		functionName, actionErr = hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			CustomEvents: afterActions,
			IDs:          objectRequest.Ids,
			TableSlug:    collection,
			ObjectData:   data,
			Method:       "DELETE_MANY",
			Resource:     resource,
		},
			c, h, // gin context,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error()+" in "+functionName)
			return
		}
	}

	statusHttp.CustomMessage = resp.GetCustomMessage()
}

func (h *HandlerV3) DeleteManyToMany(c *gin.Context) {
	var (
		m2mMessage                  obs.ManyToManyMessage
		resp                        *obs.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["NoContent"]
		actionErr                   error
		functionName                string
		collection                  = c.Param("collection")
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

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()
	if err != nil {
		h.log.Info("Error while getting "+resource.NodeType+" object builder service", logger.Error(err))
		h.HandleResponse(c, status_http.InternalServerError, err)
		return
	}

	m2mMessage.ProjectId = resource.ResourceEnvironmentId
	fromOfs := c.Query("from-ofs")
	if fromOfs != "true" {
		beforeActions, afterActions, err = hHelper.GetListCustomEvents(models.GetListCustomEventsStruct{
			TableSlug: m2mMessage.TableFrom,
			RoleId:    "",
			Method:    "DELETE_MANY2MANY",
			Resource:  resource,
		},
			c, h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}
	}

	if len(beforeActions) > 0 {
		functionName, err = hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			CustomEvents: beforeActions,
			IDs:          []string{m2mMessage.IdFrom},
			TableSlug:    m2mMessage.TableFrom,
			ObjectData:   map[string]any{"id_to": m2mMessage.IdTo, "table_to": m2mMessage.TableTo},
			Method:       "DELETE_MANY2MANY",
			Resource:     resource,
		},
			c, h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "DELETE ITEM",
			UsedEnvironments: map[string]bool{
				cast.ToString(environmentId): true,
			},
			UserInfo:  cast.ToString(userId),
			Request:   &m2mMessage,
			TableSlug: collection,
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else if actionErr != nil {
			logReq.Response = actionErr.Error() + " in " + functionName
			h.HandleResponse(c, status_http.InvalidArgument, actionErr.Error()+" in "+functionName)
		} else {
			logReq.Response = resp
			h.HandleResponse(c, status_http.NoContent, resp)
		}
		switch resource.ResourceType {
		case pb.ResourceType_MONGODB:
			go h.versionHistory(logReq)
		case pb.ResourceType_POSTGRESQL:
			go h.versionHistoryGo(c, logReq)
		}

	}()

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
			return
		}
	case pb.ResourceType_POSTGRESQL:
		// Does Not Implemented
		h.HandleResponse(c, status_http.BadRequest, "does not implemented")
		return
	}

	if len(afterActions) > 0 {
		functionName, actionErr = hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			CustomEvents: afterActions,
			IDs:          []string{m2mMessage.IdFrom},
			TableSlug:    m2mMessage.TableFrom,
			ObjectData:   map[string]any{"id_to": m2mMessage.IdTo, "table_from": m2mMessage.TableTo},
			Method:       "DELETE_MANY2MANY",
			Resource:     resource,
		},
			c, h, // gin context,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error()+" in "+functionName)
			return
		}
	}

	statusHttp.CustomMessage = resp.GetCustomMessage()
}

func (h *HandlerV3) AppendManyToMany(c *gin.Context) {
	var (
		m2mMessage                  obs.ManyToManyMessage
		resp                        *obs.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["Ok"]
		actionErr                   error
		functionName                string
		collection                  = c.Param("collection")
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

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()
	if err != nil {
		h.log.Info("Error while getting "+resource.NodeType+" object builder service", logger.Error(err))
		h.HandleResponse(c, status_http.InternalServerError, err)
		return
	}

	m2mMessage.ProjectId = resource.ResourceEnvironmentId
	fromOfs := c.Query("from-ofs")
	if fromOfs != "true" {
		beforeActions, afterActions, err = hHelper.GetListCustomEvents(models.GetListCustomEventsStruct{
			TableSlug: m2mMessage.TableFrom,
			RoleId:    "",
			Method:    "APPEND_MANY2MANY",
			Resource:  resource,
		},
			c, h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}
	}

	if len(beforeActions) > 0 {
		functionName, err = hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			CustomEvents: beforeActions,
			IDs:          []string{m2mMessage.IdFrom},
			TableSlug:    m2mMessage.TableFrom,
			ObjectData:   map[string]any{"id_to": m2mMessage.IdTo, "table_to": m2mMessage.TableTo},
			Method:       "APPEND_MANY2MANY",
			Resource:     resource,
		},
			c, h,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error()+" in "+functionName)
			return
		}
	}

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "UPDATE ITEM",
			UserInfo:     cast.ToString(userId),
			Request:      &m2mMessage,
			TableSlug:    collection,
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else if actionErr != nil {
			logReq.Response = actionErr.Error() + " in " + functionName
			h.HandleResponse(c, status_http.InvalidArgument, actionErr.Error()+" in "+functionName)
		} else {
			logReq.Response = resp
			h.HandleResponse(c, status_http.NoContent, resp)
		}
		go h.versionHistory(logReq)
	}()

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
			return
		}
	case pb.ResourceType_POSTGRESQL:
		h.HandleResponse(c, status_http.BadRequest, "does not implemented")
		return
	}

	if len(afterActions) > 0 {
		functionName, actionErr = hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			CustomEvents: afterActions,
			IDs:          []string{m2mMessage.IdFrom},
			TableSlug:    m2mMessage.TableFrom,
			ObjectData:   map[string]any{"id_to": m2mMessage.IdTo, "table_to": m2mMessage.TableTo},
			Method:       "APPEND_MANY2MANY",
			Resource:     resource,
		},
			c, h, // gin context,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error()+" in "+functionName)
			return
		}
	}
	statusHttp.CustomMessage = resp.GetCustomMessage()
}

func (h *HandlerV3) GetListAggregation(c *gin.Context) {
	var (
		reqBody    models.CommonMessage
		collection = c.Param("collection")
		resp       = &obs.CommonMessage{}
	)

	if err := c.ShouldBindJSON(&reqBody); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
	}

	key, err := json.Marshal(reqBody.Data)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	structData, err := helper.ConvertMapToStruct(reqBody.Data)
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

	if reqBody.IsCached {
		redisResp, err := h.redis.Get(
			c.Request.Context(),
			base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s-%s-%s", collection, string(key), resource.ResourceEnvironmentId))),
			projectId.(string),
			resource.NodeType,
		)
		if err == nil {
			var (
				resp = make(map[string]any)
				m    = make(map[string]any)
			)

			if err = json.Unmarshal([]byte(redisResp), &m); err != nil {
				h.log.Error("Error while unmarshal redis in items aggregation", logger.Error(err))
			} else {
				resp["data"] = m
				h.HandleResponse(c, status_http.OK, resp)
				return
			}
		}
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder().GetListAggregation(
			c.Request.Context(), &obs.CommonMessage{
				TableSlug: collection,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		pgResp, err := services.GoObjectBuilderService().ObjectBuilder().GetListAggregation(
			c.Request.Context(), &nb.CommonMessage{
				TableSlug: collection,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		err = helper.MarshalToStruct(pgResp, &resp)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}

	if reqBody.IsCached {
		jsonData, _ := resp.GetData().MarshalJSON()
		err = h.redis.SetX(
			c.Request.Context(),
			base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s-%s-%s", collection, string(key), resource.ResourceEnvironmentId))),
			string(jsonData),
			15*time.Second,
			projectId.(string),
			resource.NodeType,
		)
		if err != nil {
			h.log.Error("Error while setting redis in items aggregation", logger.Error(err))
		}
	}

	h.HandleResponse(c, status_http.OK, resp)
}

func (h *HandlerV3) UpdateRowOrder(c *gin.Context) {
	var (
		objectRequest models.CommonMessage
		collection    = c.Param("collection")
	)

	if err := c.ShouldBindJSON(&objectRequest); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	var (
		objects = cast.ToSlice(objectRequest.Data["objects"])
		limit   = cast.ToInt(objectRequest.Data["limit"])
		offset  = cast.ToInt(objectRequest.Data["offset"])
		num     = limit * offset
	)

	delete(objectRequest.Data, "limit")
	delete(objectRequest.Data, "offset")

	for i, o := range objects {
		obj := cast.ToStringMap(o)

		obj["row_order"] = i + num
	}

	objectRequest.Data["objects"] = objects

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

	structData, err := helper.ConvertMapToStruct(objectRequest.Data)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		_, err = service.MultipleUpdate(
			c.Request.Context(), &obs.CommonMessage{
				TableSlug: collection,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}
	}
}

func (h *HandlerV3) UpsertMany(c *gin.Context) {
	var (
		objectRequest models.CommonMessage
		actionErr     error
		functionName  string
		collection    = c.Param("collection")
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
		err := errors.New("error getting environment id | not valid")
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	userId, _ := c.Get("user_id")

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

	services, err := h.GetProjectSrvc(
		c.Request.Context(),
		resource.GetProjectId(),
		resource.NodeType,
	)
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

	structData, err := helper.ConvertMapToStruct(objectRequest.Data)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "UPSERT MANY ITEM",
			UserInfo:     cast.ToString(userId),
			Request:      &structData,
			TableSlug:    collection,
		}
	)

	var resp *obs.CommonMessage

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else if actionErr != nil {
			logReq.Response = actionErr.Error() + " in " + functionName
			h.HandleResponse(c, status_http.InvalidArgument, actionErr.Error()+" in "+functionName)
		} else {
			logReq.Response = resp
			h.HandleResponse(c, status_http.OK, resp)
		}
		switch resource.ResourceType {
		case pb.ResourceType_MONGODB:
			go h.versionHistory(logReq)
		case pb.ResourceType_POSTGRESQL:
			go h.versionHistoryGo(c, logReq)
		}
	}()

	service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = service.UpsertMany(c.Request.Context(), &obs.CommonMessage{
			TableSlug: collection,
			Data:      structData,
			ProjectId: resource.ResourceEnvironmentId,
		})
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		_, err = services.GoObjectBuilderService().Items().UpsertMany(c.Request.Context(),
			&nb.CommonMessage{
				TableSlug: collection,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}
	}
}

func (h *HandlerV3) AgTree(c *gin.Context) {
	var (
		objectRequest models.CommonMessage
		collection    = c.Param("collection")
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

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder().AgGridTree(
			c.Request.Context(),
			&obs.CommonMessage{
				TableSlug: collection,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().ObjectBuilder().AgGridTree(
			c.Request.Context(),
			&nb.CommonMessage{
				TableSlug: collection,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
	}
}
