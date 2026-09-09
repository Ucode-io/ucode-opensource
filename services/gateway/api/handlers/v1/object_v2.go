package v1

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/security"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"google.golang.org/grpc/status"
)

// GetListV2 godoc
// @Security ApiKeyAuth
// @ID get_list_objects_v2
// @Router /v2/object/get-list/{collection} [POST]
// @Summary Get all objects version 2
// @Description Get all objects version 2
// @Tags Object
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param language_setting query string false "language_setting"
// @Param object body models.CommonMessage true "GetListObjectRequestBody"
// @Success 200 {object} status_http.Response{data=models.CommonMessage} "ObjectBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetListV2(c *gin.Context) {
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

	if objectRequest.Data["view_type"] != "CALENDAR" {
		if _, ok := objectRequest.Data["limit"]; !ok {
			objectRequest.Data["limit"] = h.baseConf.DefaultLimitInt
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
					TableSlug:        tableSlug,
					Data:             structData,
					ProjectId:        resource.ResourceEnvironmentId,
					CompanyProjectId: projectId.(string),
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
			resp, err := services.GoObjectBuilderService().ObjectBuilder().GetList2(
				c.Request.Context(), &nb.CommonMessage{
					TableSlug:        tableSlug,
					Data:             structData,
					ProjectId:        resource.ResourceEnvironmentId,
					CompanyProjectId: projectId.(string),
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

// GetListSlimV2 godoc
// @Security ApiKeyAuth
// @ID get_list_objects_slim_v2
// @Router /v2/object-slim/get-list/{collection} [GET]
// @Summary Get all objects slim v2
// @Description Get all objects slim v2
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
func (h *HandlerV1) GetListSlimV2(c *gin.Context) {
	var (
		objectRequest models.CommonMessage
		queryData     string
		statusHttp    = status_http.GrpcStatusToHTTP["Ok"]
		hashed        bool
	)

	tableSlug := c.Param("collection")

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

	if limit > 40 {
		limit = 40
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
		var slimKey = base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "slim-%s-%s-%s", tableSlug, structData.String(), resource.ResourceEnvironmentId))
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

		resp, err := service.ObjectBuilder().GetListSlimV2(
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

// UpdateWithParams godoc
// @Security ApiKeyAuth
// @ID update_with_params
// @Router /v2/update-with/{collection} [PUT]
// @Summary UpdateWith Params
// @Description UpdateWith Params
// @Tags Object
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param item body models.CommonMessage true "UpdateWithParamsBody"
// @Success 200 {object} status_http.Response{data=models.CommonMessage} "Object data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateWithParams(c *gin.Context) {
	var objectRequest models.CommonMessage

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

	body, err := services.GoObjectBuilderService().ObjectBuilder().UpdateWithParams(
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

	h.HandleResponse(c, status_http.OK, body)
}
