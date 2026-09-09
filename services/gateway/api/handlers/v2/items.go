package v2

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	hHelper "github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/spf13/cast"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

// CreateItem godoc
// @Security ApiKeyAuth
// @ID create_item
// @Router /v2/items/{collection} [POST]
// @Summary Create item
// @Description Create item
// @Tags Items
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param object body models.CommonMessage true "CreateItemsRequestBody"
// @Success 201 {object} status_http.Response{data=models.CommonMessage} "Object data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) CreateItem(c *gin.Context) {
	timeStarted := time.Now().Format(time.RFC3339)

	var (
		objectRequest               models.CommonMessage
		resp                        *obs.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["Created"]
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
	}

	objectRequest.Data["company_service_project_id"] = resource.GetProjectId()
	objectRequest.Data["company_service_environment_id"] = resource.GetEnvironmentId()

	var id string
	uid, _ := uuid.NewRandom()
	id = uid.String()

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
			TableSlug: c.Param("collection"),
			RoleId:    "",
			Method:    "CREATE",
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
			IDs:          []string{id},
			TableSlug:    c.Param("collection"),
			ObjectData:   objectRequest.Data,
			Method:       "CREATE",
			Resource:     resource,
			ActionType:   "BEFORE",
		},
			c,
			h,
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
		MethodApi:    c.Request.Method,
		TimeStarted:  timeStarted,
		ActionType:   "CREATE ITEM",
		UserInfo:     cast.ToString(userId),
		Request:      &structData,
		TableSlug:    c.Param("collection"),
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder().Create(
			ctx, &obs.CommonMessage{
				TableSlug:      c.Param("collection"),
				Data:           structData,
				ProjectId:      resource.ResourceEnvironmentId,
				BlockedBuilder: cast.ToBool(c.DefaultQuery("block_builder", "false")),
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
			logReq.StatusCode = statusHttp.Code
			defer func() { go h.versionHistory(logReq) }()
			h.HandleResponse(c, statusHttp, err.Error())
			return
		}
		logReq.Response = resp
		logReq.StatusCode = statusHttp.Code
		defer func() { go h.versionHistory(logReq) }()
	case pb.ResourceType_POSTGRESQL:
		body, err := services.GoObjectBuilderService().Items().Create(
			ctx, &nb.CommonMessage{
				TableSlug:      c.Param("collection"),
				Data:           structData,
				ProjectId:      resource.ResourceEnvironmentId,
				BlockedBuilder: cast.ToBool(c.DefaultQuery("block_builder", "false")),
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
			logReq.StatusCode = statusHttp.Code
			defer func() { go h.versionHistoryGo(c, logReq) }()
			h.handleDynamicError(c, statusHttp, err)
			return
		}

		if err = helper.MarshalToStruct(body, &resp); err != nil {
			return
		}

		logReq.Response = resp
		logReq.StatusCode = statusHttp.Code
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
			Services:     services.GoObjectBuilderService(),
			CustomEvents: afterActions,
			IDs:          []string{id},
			TableSlug:    c.Param("collection"),
			ObjectData:   objectRequest.Data,
			Method:       "CREATE",
			Resource:     resource,
			ActionType:   "AFTER",
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
	h.HandleResponse(c, statusHttp, resp)
}

// CreateItems godoc
// @Security ApiKeyAuth
// @ID create_items
// @Router /v2/items/{collection}/multiple-insert [POST]
// @Summary Create items
// @Description Create items
// @Tags Items
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param object body models.MultipleInsertItems true "CreateItemsRequestBody"
// @Success 201 {object} status_http.Response{data=models.CommonMessage} "Object data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) CreateItems(c *gin.Context) {
	var (
		objectRequest               models.MultipleInsertItems
		resp                        *obs.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["Created"]
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
			TableSlug: c.Param("collection"),
			RoleId:    "",
			Method:    "CREATE_MANY",
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
			TableSlug:    c.Param("collection"),
			ObjectData:   request,
			Method:       "CREATE_MANY",
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

	logReq := &models.CreateVersionHistoryRequest{
		Services:     services,
		NodeType:     resource.NodeType,
		ProjectId:    resource.ResourceEnvironmentId,
		ActionSource: c.Request.URL.String(),
		MethodApi:    c.Request.Method,
		TimeStarted:  time.Now().Format(time.RFC3339),
		ActionType:   "CREATE ITEM",
		UserInfo:     cast.ToString(userId),
		Request:      structData,
		TableSlug:    c.Param("collection"),
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder().Create(
			context.Background(),
			&obs.CommonMessage{
				TableSlug: c.Param("collection"),
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
		logReq.StatusCode = statusHttp.Code
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
		functionName, err := hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: afterActions,
			IDs:          ids,
			TableSlug:    c.Param("collection"),
			ObjectData:   request,
			Method:       "CREATE_MANY",
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

// GetSingleItem godoc
// @Security ApiKeyAuth
// @ID get_item_by_id
// @Router /v2/items/{collection}/{id} [GET]
// @Summary Get item by id
// @Description Get item by id
// @Tags Items
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param id path string true "id"
// @Success 200 {object} status_http.Response{data=models.CommonMessage} "ObjectBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetSingleItem(c *gin.Context) {
	timeStarted := time.Now().Format(time.RFC3339)

	var (
		object     models.CommonMessage
		statusHttp = status_http.GrpcStatusToHTTP["Ok"]

		tableSlug = c.Param("collection")
		objectID  = c.Param("id")

		withRelations = cast.ToBool(c.Query("with_relations"))
	)

	if !util.IsValidUUID(objectID) {
		h.HandleResponse(c, status_http.InvalidArgument, "id is an invalid uuid")
		return
	}

	object.Data = make(map[string]any)
	object.Data["id"] = objectID
	object.Data["with_relations"] = withRelations

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

	redisKey := base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "%s-%s-%s", resource.ResourceEnvironmentId, tableSlug, objectID))

	var logRequest = &models.CreateVersionHistoryRequest{
		Services:     services,
		NodeType:     resource.NodeType,
		ProjectId:    resource.ResourceEnvironmentId,
		ActionSource: "ITEMS",
		ActionType:   "GET_ITEM",
		Request:      structData,
		TableSlug:    tableSlug,
		MethodApi:    "GET",
		TimeStarted:  timeStarted,
	}
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder().GetSingle(
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
			go func() {
				logRequest.Response = err.Error()
				logRequest.StatusCode = statusHttp.Code
				h.versionHistory(logRequest)
			}()
			h.HandleResponse(c, statusHttp, err.Error())
			return
		}

		statusHttp.CustomMessage = resp.GetCustomMessage()
		h.HandleResponse(c, statusHttp, resp)

		go func() {
			logRequest.Response = resp
			logRequest.StatusCode = statusHttp.Code
			h.versionHistory(logRequest)
		}()
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
				return
			}
		}
		resp, err := services.GoObjectBuilderService().Items().GetSingle(
			c.Request.Context(), &nb.CommonMessage{
				TableSlug: tableSlug,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			go func() {
				logRequest.Response = err.Error()
				logRequest.StatusCode = status_http.GRPCError.Code
				h.versionHistoryGo(c, logRequest)
			}()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		if resp.IsCached {
			jsonData, _ := resp.GetData().MarshalJSON()
			err = h.redis.SetX(c.Request.Context(), redisKey, string(jsonData), config.REDIS_KEY_TIMEOUT, projectId.(string), resource.NodeType)
			if err != nil {
				h.log.Error("Error while setting redis", logger.Error(err))
			}
		}

		statusHttp.CustomMessage = resp.GetCustomMessage()
		h.HandleResponse(c, statusHttp, resp)

		go func() {
			logRequest.Response = resp
			logRequest.StatusCode = statusHttp.Code
			h.versionHistoryGo(c, logRequest)
		}()
	}
}

// GetAllItems godoc
// @Security ApiKeyAuth
// @ID get_list_items
// @Router /v2/items/{collection} [GET]
// @Summary Get all items
// @Description Get all items
// @Tags Items
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param language_setting query string false "language_setting"
// @Param data query string false "data"
// @Success 200 {object} status_http.Response{data=models.CommonMessage} "ObjectBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetAllItems(c *gin.Context) {
	timeStarted := time.Now().Format(time.RFC3339)

	var (
		resp          *obs.CommonMessage
		statusHttp    = status_http.GrpcStatusToHTTP["Ok"]
		queryData     string
		objectRequest = make(map[string]any)
		tableSlug     = c.Param("collection")
	)

	queryParams := c.Request.URL.Query()
	if ok := queryParams.Has("data"); ok {
		queryData = queryParams.Get("data")
	} else {
		queryData = "{}"
	}

	decoded, err := url.QueryUnescape(queryData)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	if err := json.Unmarshal([]byte(decoded), &objectRequest); err != nil {
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
			objectRequest["tables"] = tokenInfo.GetTables()
		}
		objectRequest["user_id_from_token"] = tokenInfo.GetUserId()
		objectRequest["role_id_from_token"] = tokenInfo.GetRoleId()
		objectRequest["client_type_id_from_token"] = tokenInfo.GetClientTypeId()
	}

	objectRequest["language_setting"] = c.DefaultQuery("language_setting", "")

	if c.DefaultQuery("with_types", "false") == "true" {
		objectRequest["with_types"] = true
	}

	structData, err := helper.ConvertMapToStruct(objectRequest)
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

	service := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder()

	redisKey := base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "%s-%s-%s", tableSlug, structData.String(), resource.ResourceEnvironmentId))

	var logRequest = &models.CreateVersionHistoryRequest{
		Services:     services,
		NodeType:     resource.NodeType,
		ProjectId:    resource.ResourceEnvironmentId,
		ActionSource: "ITEMS",
		ActionType:   "GET_LIST",
		Request:      structData,
		TableSlug:    tableSlug,
		MethodApi:    "GET",
		TimeStarted:  timeStarted,
	}

	if viewId, ok := objectRequest["builder_service_view_id"].(string); ok {
		if util.IsValidUUID(viewId) {
			switch resource.ResourceType {
			case pb.ResourceType_MONGODB:
				redisResp, err := h.redis.Get(c.Request.Context(), redisKey, resource.ProjectId, resource.NodeType)
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

				resp, err = service.GroupByColumns(
					c.Request.Context(),
					&obs.CommonMessage{
						TableSlug: tableSlug,
						Data:      structData,
						ProjectId: resource.ResourceEnvironmentId,
					},
				)
				if err != nil {
					go func() {
						logRequest.Response = err.Error()
						logRequest.StatusCode = status_http.GRPCError.Code
						h.versionHistory(logRequest)
					}()
					h.HandleResponse(c, status_http.GRPCError, err.Error())
					return
				}

				go func() {
					logRequest.Response = resp
					logRequest.StatusCode = status_http.OK.Code
					h.versionHistory(logRequest)
				}()

				if resp.IsCached {
					jsonData, _ := resp.GetData().MarshalJSON()
					err = h.redis.SetX(context.Background(), redisKey, string(jsonData), 15*time.Second, resource.ProjectId, resource.NodeType)
					if err != nil {
						h.log.Error("Error while setting redis", logger.Error(err))
					}
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
			redisResp, err := h.redis.Get(context.Background(), redisKey, resource.ProjectId, resource.NodeType)
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

			resp, err = services.GetBuilderServiceByType(resource.NodeType).ItemsService().GetList(
				c.Request.Context(),
				&obs.CommonMessage{
					TableSlug: tableSlug,
					Data:      structData,
					ProjectId: resource.ResourceEnvironmentId,
				},
			)
			if err != nil {
				go func() {
					logRequest.Response = err.Error()
					logRequest.StatusCode = status_http.GRPCError.Code
					h.versionHistory(logRequest)
				}()
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}

			go func() {
				logRequest.Response = resp
				logRequest.StatusCode = status_http.OK.Code
				h.versionHistory(logRequest)
			}()

			if resp.IsCached {
				jsonData, _ := resp.GetData().MarshalJSON()
				err = h.redis.SetX(context.Background(), redisKey, string(jsonData), 15*time.Second, resource.ProjectId, resource.NodeType)
				if err != nil {
					h.log.Error("Error while setting redis", logger.Error(err))
				}
			}

		case pb.ResourceType_POSTGRESQL:
			redisResp, err := h.redis.Get(context.Background(), redisKey, resource.ProjectId, resource.NodeType)
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

			resp, err := services.GoObjectBuilderService().ObjectBuilder().GetList2(
				c.Request.Context(), &nb.CommonMessage{
					TableSlug:        tableSlug,
					Data:             structData,
					ProjectId:        resource.ResourceEnvironmentId,
					CompanyProjectId: resource.ProjectId,
				},
			)
			if err != nil {
				go func() {
					logRequest.Response = err.Error()
					logRequest.StatusCode = status_http.GRPCError.Code
					h.versionHistoryGo(c, logRequest)
				}()
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}

			go func() {
				logRequest.Response = resp
				logRequest.StatusCode = status_http.OK.Code
				h.versionHistoryGo(c, logRequest)
			}()

			if resp.IsCached {
				jsonData, _ := resp.GetData().MarshalJSON()
				err = h.redis.SetX(context.Background(), redisKey, string(jsonData), 15*time.Second, resource.ProjectId, resource.NodeType)
				if err != nil {
					h.log.Error("Error while setting redis", logger.Error(err))
				}
			}

			statusHttp.CustomMessage = resp.GetCustomMessage()
			h.HandleResponse(c, statusHttp, resp)

			return
		}
	}

	statusHttp.CustomMessage = resp.GetCustomMessage()
	h.HandleResponse(c, statusHttp, resp)
}

// UpdateItem godoc
// @Security ApiKeyAuth
// @ID update_item
// @Router /v2/items/{collection} [PUT]
// @Summary Update item
// @Description Update item
// @Tags Items
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param item body models.CommonMessage true "UpdateItemRequestBody"
// @Success 200 {object} status_http.Response{data=models.CommonMessage} "Item data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) UpdateItem(c *gin.Context) {
	var (
		objectRequest               models.CommonMessage
		resp, singleObject          *obs.CommonMessage
		body                        *nb.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["Ok"]
		actionErr                   error
		functionName                string
		id                          string
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
				TableSlug: c.Param("collection"),
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
				TableSlug: c.Param("collection"),
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
			TableSlug: c.Param("collection"),
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
	}

	if len(beforeActions) > 0 {
		functionName, err := hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: beforeActions,
			IDs:          []string{id},
			TableSlug:    c.Param("collection"),
			ObjectData:   objectRequest.Data,
			Method:       "UPDATE",
			ActionType:   "BEFORE",
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

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			MethodApi:    c.Request.Method,
			TimeStarted:  time.Now().Format(time.RFC3339),
			ActionType:   "UPDATE ITEM",
			UserInfo:     cast.ToString(userId),
			Request:      &structData,
			TableSlug:    c.Param("collection"),
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
			h.handleDynamicError(c, status_http.GRPCError, err)
		} else if actionErr != nil {
			logReq.Response = actionErr.Error() + " in " + functionName
			h.handleDynamicError(c, status_http.InvalidArgument, actionErr)
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
				TableSlug:        c.Param("collection"),
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
				TableSlug:        c.Param("collection"),
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
			TableSlug:              c.Param("collection"),
			ObjectData:             objectRequest.Data,
			Method:                 "UPDATE",
			ObjectDataBeforeUpdate: singleObject.Data.AsMap(),
			ActionType:             "AFTER",
			Resource:               resource,
		},
			c, // gin context,
			h, // handler
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

// UpdateItemByID godoc
// @Security ApiKeyAuth
// @ID update_item_by_id
// @Router /v2/items/{collection}/{id} [PUT]
// @Summary Update item by id
// @Description Update item by id
// @Tags Items
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param id path string true "id"
// @Param item body models.CommonMessage true "UpdateItemRequestBody"
// @Success 200 {object} status_http.Response{data=models.CommonMessage} "Item data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) UpdateItemByIDSwaggerDoc(c *gin.Context) {}

// PatchItemByID godoc
// @Security ApiKeyAuth
// @ID patch_item_by_id
// @Router /v2/items/{collection}/{id} [PATCH]
// @Summary Patch item by id
// @Description Patch item by id
// @Tags Items
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param id path string true "id"
// @Param item body models.CommonMessage true "UpdateItemRequestBody"
// @Success 200 {object} status_http.Response{data=models.CommonMessage} "Item data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) PatchItemByIDSwaggerDoc(c *gin.Context) {}

// MultipleUpdateItems godoc
// @Security ApiKeyAuth
// @ID multiple_update_items
// @Router /v2/items/{collection} [PATCH]
// @Summary Multiple Update items
// @Description Multiple Update items
// @Tags Items
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param items body models.MultipleUpdateItems true "MultipleItemsRequesUpdatetBody"
// @Success 201 {object} status_http.Response{data=models.CommonMessage} "Items data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) MultipleUpdateItems(c *gin.Context) {
	var (
		objectRequest               models.MultipleUpdateItems
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["Created"]
		actionErr                   error
		functionName                string
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
			TableSlug: c.Param("collection"),
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
		functionName, err = hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: beforeActions,
			IDs:          objectRequest.Ids,
			TableSlug:    c.Param("collection"),
			ObjectData:   objectRequest.Data,
			Method:       "MULTIPLE_UPDATE",
			ActionType:   "BEFORE",
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

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			MethodApi:    c.Request.Method,
			TimeStarted:  time.Now().Format(time.RFC3339),
			ActionType:   "UPDATE ITEM",
			UserInfo:     cast.ToString(userId),
			Request:      &structData,
			TableSlug:    c.Param("collection"),
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
				TableSlug: c.Param("collection"),
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
				TableSlug: c.Param("collection"),
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
			TableSlug:    c.Param("collection"),
			ObjectData:   objectRequest.Data,
			Method:       "MULTIPLE_UPDATE",
			ActionType:   "AFTER",
			Resource:     resource,
		},
			c, // gin context
			h, // handler
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error()+" in "+functionName)
			return
		}
	}
	statusHttp.CustomMessage = resp.GetCustomMessage()
}

// DeleteItem godoc
// @Security ApiKeyAuth
// @ID delete_item
// @Router /v2/items/{collection}/{id} [DELETE]
// @Summary Delete item
// @Description Delete item
// @Tags Items
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param id path string true "id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) DeleteItem(c *gin.Context) {
	var (
		objectRequest               models.CommonMessage
		resp                        *obs.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["NoContent"]
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
			TableSlug: c.Param("collection"),
			RoleId:    "",
			Method:    "DELETE",
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
			TableSlug:    c.Param("collection"),
			ObjectData:   objectRequest.Data,
			Method:       "DELETE",
			Resource:     resource,
			ActionType:   "BEFORE",
		},
			c,
			h,
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
		MethodApi:    c.Request.Method,
		TimeStarted:  time.Now().Format(time.RFC3339),
		ActionType:   "DELETE ITEM",
		UserInfo:     cast.ToString(userId),
		Request:      structData,
		TableSlug:    c.Param("collection"),
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder().Delete(
			c.Request.Context(), &obs.CommonMessage{
				TableSlug: c.Param("collection"),
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
				TableSlug: c.Param("collection"),
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
			TableSlug:    c.Param("collection"),
			ObjectData:   objectRequest.Data,
			Method:       "DELETE",
			Resource:     resource,
			ActionType:   "AFTER",
		},
			c, // gin context,
			h, // handler
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

// DeleteManyObject godoc
// @Security ApiKeyAuth
// @ID delete_items
// @Router /v2/items/{collection} [DELETE]
// @Summary Delete many itmes
// @Description Delete many itmes
// @Tags Items
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param object body models.Ids true "DeleteManyItemRequestBody"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) DeleteItems(c *gin.Context) {
	var (
		objectRequest               models.Ids
		resp                        *obs.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["NoContent"]
		data                        = make(map[string]any)
		actionErr                   error
		functionName                string
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
			TableSlug: c.Param("collection"),
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
		functionName, err = hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			CustomEvents: beforeActions,
			IDs:          objectRequest.Ids,
			TableSlug:    c.Param("collection"),
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

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			MethodApi:    c.Request.Method,
			TimeStarted:  time.Now().Format(time.RFC3339),
			ActionType:   "DELETE ITEM",
			UserInfo:     cast.ToString(userId),
			Request:      &structData,
			TableSlug:    c.Param("collection"),
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
				TableSlug: c.Param("collection"),
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
				TableSlug: c.Param("collection"),
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
			Services:     services.GoObjectBuilderService(),
			CustomEvents: afterActions,
			IDs:          objectRequest.Ids,
			TableSlug:    c.Param("collection"),
			ObjectData:   data,
			Method:       "DELETE_MANY",
			Resource:     resource,
		},
			c, // gin context,
			h, // handler
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error()+" in "+functionName)
			return
		}
	}

	statusHttp.CustomMessage = resp.GetCustomMessage()
}

// DeleteManyToMany godoc
// @Security ApiKeyAuth
// @ID v2_delete_many2many
// @Router /v2/items/many-to-many [DELETE]
// @Summary Delete Many2Many items
// @Description Delete Many2Many items
// @Tags Items
// @Accept json
// @Produce json
// @Param object body obs.ManyToManyMessage true "DeleteManyToManyBody"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) DeleteManyToMany(c *gin.Context) {
	var (
		m2mMessage                  obs.ManyToManyMessage
		resp                        *obs.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["NoContent"]
		actionErr                   error
		functionName                string
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
			c,
			h,
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
			c,
			h,
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
			MethodApi:    c.Request.Method,
			TimeStarted:  time.Now().Format(time.RFC3339),
			ActionType:   "DELETE ITEM",
			UsedEnvironments: map[string]bool{
				cast.ToString(environmentId): true,
			},
			UserInfo:  cast.ToString(userId),
			Request:   &m2mMessage,
			TableSlug: c.Param("collection"),
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
			Services:     services.GoObjectBuilderService(),
			CustomEvents: afterActions,
			IDs:          []string{m2mMessage.IdFrom},
			TableSlug:    m2mMessage.TableFrom,
			ObjectData:   map[string]any{"id_to": m2mMessage.IdTo, "table_from": m2mMessage.TableTo},
			Method:       "DELETE_MANY2MANY",
			Resource:     resource,
		},
			c, // gin context,
			h, // handler
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error()+" in "+functionName)
			return
		}
	}

	statusHttp.CustomMessage = resp.GetCustomMessage()
}

// AppendManyToMany godoc
// @Security ApiKeyAuth
// @ID v2_append_many2many
// @Router /v2/items/many-to-many [PUT]
// @Summary Append many-to-many items
// @Description Append many-to-many items
// @Tags Items
// @Accept json
// @Produce json
// @Param object body obs.ManyToManyMessage true "UpdateMany2ManyRequestBody"
// @Success 200 {object} status_http.Response{data=string} "Object data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) AppendManyToMany(c *gin.Context) {
	var (
		m2mMessage                  obs.ManyToManyMessage
		resp                        *obs.CommonMessage
		beforeActions, afterActions []*obs.CustomEvent
		statusHttp                  = status_http.GrpcStatusToHTTP["Ok"]
		actionErr                   error
		functionName                string
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
			CustomEvents: beforeActions,
			IDs:          []string{m2mMessage.IdFrom},
			TableSlug:    m2mMessage.TableFrom,
			ObjectData:   map[string]any{"id_to": m2mMessage.IdTo, "table_to": m2mMessage.TableTo},
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

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			MethodApi:    c.Request.Method,
			TimeStarted:  time.Now().Format(time.RFC3339),
			ActionType:   "UPDATE ITEM",
			UsedEnvironments: map[string]bool{
				cast.ToString(environmentId): true,
			},
			UserInfo:  cast.ToString(userId),
			Request:   &m2mMessage,
			TableSlug: c.Param("collection"),
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
		// Does Not Implemented
		h.HandleResponse(c, status_http.BadRequest, "does not implemented")
		return
	}

	if len(afterActions) > 0 {
		functionName, actionErr = hHelper.DoInvokeFunction(models.DoInvokeFunctionStruct{
			Services:     services.GoObjectBuilderService(),
			CustomEvents: afterActions,
			IDs:          []string{m2mMessage.IdFrom},
			TableSlug:    m2mMessage.TableFrom,
			ObjectData:   map[string]any{"id_to": m2mMessage.IdTo, "table_to": m2mMessage.TableTo},
			Method:       "APPEND_MANY2MANY",
			Resource:     resource,
		},
			c, // gin context,
			h, // handler
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error()+" in "+functionName)
			return
		}
	}
	statusHttp.CustomMessage = resp.GetCustomMessage()
}

// GetListAggregation godoc
// @Security ApiKeyAuth
// @ID get_list_aggregation
// @Router /v2/items/{collection}/aggregation [POST]
// @Summary Get List Aggregation
// @Description Get List Aggregation
// @Tags Items
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param object body models.CommonMessage true "GetListAggregation"
// @Success 200 {object} status_http.Response{data=models.CommonMessage} "ObjectBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetListAggregation(c *gin.Context) {
	var (
		reqBody   models.CommonMessage
		tableSlug = c.Param("collection")
		resp      = &obs.CommonMessage{}
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
			base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s-%s-%s", tableSlug, string(key), resource.ResourceEnvironmentId))),
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
				TableSlug: c.Param("collection"),
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
				TableSlug: c.Param("collection"),
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
			base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s-%s-%s", tableSlug, string(key), resource.ResourceEnvironmentId))),
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

// UpdateRowOrder godoc
// @Security ApiKeyAuth
// @ID update_row_order
// @Router /v2/update-row/{collection} [PUT]
// @Summary Update Row Order
// @Description Update Row Order
// @Tags Items
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param object body models.CommonMessage true "GetListAggregation"
// @Success 200 {object} status_http.Response{data=models.CommonMessage} "ObjectBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) UpdateRowOrder(c *gin.Context) {
	var objectRequest models.CommonMessage

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
				TableSlug: c.Param("collection"),
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

// UpsertManyItems godoc
// @Security ApiKeyAuth
// @ID upsert_many_items
// @Router /v2/items/{collection}/upsert-many [POST]
// @Summary Upsert Many items
// @Description Upsert Many items
// @Tags Items
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param object body models.CommonMessage true "UpsertManyItemsRequestBody"
// @Success 201 {object} status_http.Response{data=models.CommonMessage} "Object data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) UpsertMany(c *gin.Context) {
	var (
		objectRequest models.CommonMessage
		actionErr     error
		functionName  string
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
			MethodApi:    c.Request.Method,
			TimeStarted:  time.Now().Format(time.RFC3339),
			ActionType:   "UPSERT MANY ITEM",
			UserInfo:     cast.ToString(userId),
			Request:      &structData,
			TableSlug:    c.Param("collection"),
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
			TableSlug: c.Param("collection"),
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
				TableSlug: c.Param("collection"),
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

func (h *HandlerV2) AgTree(c *gin.Context) {
	var objectRequest models.CommonMessage

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

	offset := objectRequest.Data["offset"]
	if offset == nil {
		objectRequest.Data["offset"] = 0
	}
	limit := cast.ToInt(objectRequest.Data["limit"])
	if limit == 0 || limit > 100 {
		objectRequest.Data["limit"] = 100
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
				TableSlug: c.Param("collection"),
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
				TableSlug: c.Param("collection"),
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

func (h *HandlerV2) GetBoardStructure(c *gin.Context) {
	var objectRequest models.CommonMessage

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
		resp, err := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder().GetBoardStructure(
			c.Request.Context(),
			&obs.CommonMessage{
				TableSlug: c.Param("collection"),
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
		resp, err := services.GoObjectBuilderService().ObjectBuilder().GetBoardStructure(
			c.Request.Context(),
			&nb.CommonMessage{
				TableSlug: c.Param("collection"),
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleError(c, status_http.GRPCError, err)
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
	}
}

func (h *HandlerV2) GetBoardData(c *gin.Context) {
	var objectRequest models.CommonMessage

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
		resp, err := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder().GetBoardData(
			c.Request.Context(),
			&obs.CommonMessage{
				TableSlug: c.Param("collection"),
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleError(c, status_http.GRPCError, err)
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().ObjectBuilder().GetBoardData(
			c.Request.Context(),
			&nb.CommonMessage{
				TableSlug: c.Param("collection"),
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleError(c, status_http.GRPCError, err)
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
	}
}

// GetTableSchema godoc
// @Security ApiKeyAuth
// @ID get_table_schema
// @Router /v2/items/{collection}/schema [GET]
// @Summary Get table schema
// @Description Returns table schema (columns, indexes, constraints) equivalent to PostgreSQL \d command
// @Tags Items
// @Produce json
// @Param collection path string true "collection"
// @Param project-id query string true "project-id"
// @Success 200 {object} status_http.Response{data=object} "Table schema"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetTableSchema(c *gin.Context) {
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

	resp, err := services.GoObjectBuilderService().ObjectBuilder().GetTableSchema(
		c.Request.Context(),
		&nb.CommonMessage{
			TableSlug: c.Param("collection"),
			ProjectId: resource.ResourceEnvironmentId,
		},
	)
	if err != nil {
		h.handleError(c, status_http.GRPCError, err)
		return
	}
	h.HandleResponse(c, status_http.OK, resp)
}

// pgTypeToFieldType maps postgres type names to our internal field types.
// Mirrors TRACKED_TABLES_FIELD_TYPES in object builder.
var pgTypeToFieldType = map[string]string{
	"character varying": "SINGLE_LINE",
	"varchar":           "SINGLE_LINE",
	"text":              "MULTI_LINE",
	"enum":              "SINGLE_LINE",
	"bytea":             "SINGLE_LINE",
	"citext":            "SINGLE_LINE",

	"jsonb": "JSON",
	"json":  "JSON",

	"smallint":         "NUMBER",
	"integer":          "NUMBER",
	"bigint":           "NUMBER",
	"numeric":          "FLOAT",
	"decimal":          "FLOAT",
	"real":             "FLOAT",
	"double precision": "FLOAT",
	"smallserial":      "NUMBER",
	"serial":           "NUMBER",
	"bigserial":        "NUMBER",
	"money":            "FLOAT",
	"int2":             "NUMBER",
	"int4":             "NUMBER",
	"int8":             "NUMBER",

	"timestamp without time zone": "DATE_TIME_WITHOUT_TIME_ZONE",
	"timestamp with time zone":    "DATE_TIME",
	"timestamp":                   "DATE_TIME",
	"timestamptz":                 "DATE_TIME",
	"date":                        "DATE",

	"boolean": "CHECKBOX",

	"uuid": "UUID",

	"text[]": "MULTISELECT",
	"uuid[]": "LOOKUPS",
}

// CreateTableSchemaField godoc
// @Security ApiKeyAuth
// @ID create_table_schema_field
// @Router /v2/items/{collection}/schema [POST]
// @Summary Create a new field/column for a collection
// @Description Accepts the same body as POST /v2/fields/:collection but type is a postgres type which gets mapped to a ucode field type
// @Tags Items
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param project-id query string true "project-id"
// @Param object body models.CreateFieldRequest true "CreateSchemaFieldBody"
// @Success 201 {object} status_http.Response "Field data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) CreateTableSchemaField(c *gin.Context) {
	var fieldRequest models.CreateFieldRequest

	if err := c.ShouldBindJSON(&fieldRequest); err != nil {
		h.handleError(c, status_http.BadRequest, err)
		return
	}

	// Map postgres type to ucode field type
	ucodeType, ok := pgTypeToFieldType[fieldRequest.Type]
	if !ok {
		h.HandleResponse(c, status_http.BadRequest, "unsupported pg_type: "+fieldRequest.Type)
		return
	}
	fieldRequest.Type = ucodeType

	if fieldRequest.ID == "" {
		fieldRequest.ID = uuid.New().String()
	}

	attributes, err := helper.ConvertMapToStruct(fieldRequest.Attributes)
	if err != nil {
		h.handleError(c, status_http.InvalidArgument, err)
		return
	}

	projectId, ok2 := c.Get("project_id")
	if !ok2 || !util.IsValidUUID(projectId.(string)) {
		h.handleError(c, status_http.InvalidArgument, errors.New("project id is not valid"))
		return
	}

	environmentId, ok3 := c.Get("environment_id")
	if !ok3 || !util.IsValidUUID(environmentId.(string)) {
		h.handleError(c, status_http.BadRequest, errors.New("environment id is not valid"))
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
		h.handleError(c, status_http.GRPCError, err)
		return
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.handleError(c, status_http.InternalServerError, err)
		return
	}

	field := SetTitlePrefix(fieldRequest, "", resource.ResourceEnvironmentId, attributes, false, false)

	var newReq nb.CreateFieldRequest
	if err = helper.MarshalToStruct(&field, &newReq); err != nil {
		h.handleError(c, status_http.InternalServerError, err)
		return
	}

	resp, err := services.GoObjectBuilderService().Field().Create(c.Request.Context(), &newReq)
	if err != nil {
		h.handleError(c, status_http.InternalServerError, err)
		return
	}
	h.HandleResponse(c, status_http.Created, resp)
}

// UpdateTableSchemaField godoc
// @Security ApiKeyAuth
// @ID update_table_schema_field
// @Router /v2/items/{collection}/schema [PUT]
// @Summary Update a field/column for a collection
// @Description Accepts the same body as PUT /v2/fields/:collection but type is a postgres type which gets mapped to a ucode field type
// @Tags Items
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param project-id query string true "project-id"
// @Param object body models.Field true "UpdateSchemaFieldBody"
// @Success 200 {object} status_http.Response "Field data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) UpdateTableSchemaField(c *gin.Context) {
	var fieldRequest models.Field

	if err := c.ShouldBindJSON(&fieldRequest); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	// Map postgres type to ucode field type
	ucodeType, ok := pgTypeToFieldType[fieldRequest.Type]
	if !ok {
		h.HandleResponse(c, status_http.BadRequest, "unsupported pg_type: "+fieldRequest.Type)
		return
	}
	fieldRequest.Type = ucodeType

	attributes, err := helper.ConvertMapToStruct(fieldRequest.Attributes)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	projectId, ok2 := c.Get("project_id")
	if !ok2 || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok3 := c.Get("environment_id")
	if !ok3 || !util.IsValidUUID(environmentId.(string)) {
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

	field := obs.Field{
		Id:                  fieldRequest.ID,
		Default:             fieldRequest.Default,
		Type:                fieldRequest.Type,
		Index:               fieldRequest.Index,
		Label:               fieldRequest.Label,
		Slug:                fieldRequest.Slug,
		TableId:             fieldRequest.TableID,
		Required:            fieldRequest.Required,
		Attributes:          attributes,
		IsVisible:           fieldRequest.IsVisible,
		AutofillField:       fieldRequest.AutoFillField,
		AutofillTable:       fieldRequest.AutoFillTable,
		RelationId:          fieldRequest.RelationId,
		Automatic:           fieldRequest.Automatic,
		Unique:              fieldRequest.Unique,
		RelationField:       fieldRequest.RelationField,
		ShowLabel:           fieldRequest.ShowLabel,
		EnableMultilanguage: fieldRequest.EnableMultilanguage,
		ProjectId:           resource.ResourceEnvironmentId,
		EnvId:               resource.EnvironmentId,
	}

	newReq := nb.Field{}
	if err = helper.MarshalToStruct(&field, &newReq); err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	resp, err := services.GoObjectBuilderService().Field().Update(c.Request.Context(), &newReq)
	if err != nil {
		h.handleDynamicError(c, status_http.GRPCError, err)
		return
	}
	h.HandleResponse(c, status_http.OK, resp)
}
