package handlers

import (
	"maps"
	"github.com/Ucode-io/ucode-opensource/services/auth/api/http"
	"github.com/Ucode-io/ucode-opensource/services/auth/api/models"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/company_service"
	nobs "github.com/Ucode-io/ucode-opensource/services/auth/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/auth/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/saidamir98/udevs_pkg/util"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

// @Security ApiKeyAuth
// V2CreateUser godoc
// @ID create_user_v2
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/user [POST]
// @Summary Create User
// @Description Create User
// @Tags V2_User
// @Accept json
// @Produce json
// @Param user body auth_service.CreateUserRequest true "CreateUserRequestBody"
// @Success 201 {object} http.Response{data=auth_service.User} "User data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2CreateUser(c *gin.Context) {
	var (
		user auth_service.CreateUserRequest
		resp *auth_service.User
	)

	if err := c.ShouldBindJSON(&user); err != nil {
		h.handleError(c, http.BadRequest, err)
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleError(c, http.BadRequest, config.ErrEnvironmentIdValid)
		return
	}

	if !util.IsValidUUID(user.ProjectId) {
		h.handleError(c, http.BadRequest, config.ErrProjectIdValid)
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(c.Request.Context(), &pb.GetSingleServiceResourceReq{
		EnvironmentId: environmentId.(string),
		ProjectId:     user.ProjectId,
		ServiceType:   pb.ServiceType_BUILDER_SERVICE,
	})
	if err != nil {
		h.handleResponse(c, http.GRPCError, err)
		return
	}

	user.ResourceEnvironmentId = resource.ResourceEnvironmentId
	user.ResourceType = int32(resource.GetResourceType())
	user.EnvironmentId = resource.EnvironmentId

	resp, err = h.services.UserService().V2CreateUser(
		c.Request.Context(), &user,
	)
	if err != nil {
		if st, ok := grpcstatus.FromError(err); ok && st.Code() == codes.ResourceExhausted {
			h.handleResponse(c, http.PaymentRequired, paymentRequiredFromLimit(st.Message()))
			return
		}
		h.handleError(c, http.InternalServerError, err)
		return
	}

	h.handleResponse(c, http.Created, resp)
}

// V2GetUserList godoc
// @ID get_user_list_v2
// @Param Resource-Id header string true "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/user [GET]
// @Summary Get User List
// @Description  Get User List
// @Tags V2_User
// @Accept json
// @Produce json
// @Param offset query integer false "offset"
// @Param limit query integer false "limit"
// @Param search query string false "search"
// @Param client-platform-id query string false "client-platform-id"
// @Param client-type-id query string false "client-type-id"
// @Param project-id query string true "project-id"
// @Success 200 {object} http.Response{data=auth_service.GetUserListResponse} "GetUserListResponseBody"
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2GetUserList(c *gin.Context) {
	limit, err := h.getLimitParam(c)
	if err != nil {
		h.handleResponse(c, http.InvalidArgument, err.Error())
		return
	}

	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.handleResponse(c, http.InvalidArgument, err.Error())
		return
	}

	projectId := c.DefaultQuery("project-id", "")
	if !util.IsValidUUID(projectId) {
		h.handleResponse(c, http.BadEnvironment, "project-id is required")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, errors.New("cant get environment_id"))
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(c.Request.Context(), &pb.GetSingleServiceResourceReq{
		ProjectId:     projectId,
		EnvironmentId: environmentId.(string),
		ServiceType:   pb.ServiceType_BUILDER_SERVICE,
	})
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	resp, err := h.services.UserService().V2GetUserList(
		c.Request.Context(), &auth_service.GetUserListRequest{
			Limit:                 int32(limit),
			Offset:                int32(offset),
			Search:                c.Query("search"),
			ClientPlatformId:      c.Query("client-platform-id"),
			ClientTypeId:          c.Query("client-type-id"),
			ProjectId:             projectId,
			ResourceEnvironmentId: resource.GetResourceEnvironmentId(),
			ResourceType:          int32(resource.GetResourceType()),
			NodeType:              resource.NodeType,
		},
	)

	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.OK, resp)
}

// V2GetUserByID godoc
// @ID get_user_by_id_v2
// @Router /v2/user/{user-id} [GET]
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Summary Get User By ID
// @Description Get User By ID
// @Tags V2_User
// @Accept json
// @Produce json
// @Param user-id path string true "user-id"
// @Param project-id query string true "project-id"
// @Param client-type-id query string false "client-type-id"
// @Success 200 {object} http.Response{data=auth_service.User} "UserBody"
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2GetUserByID(c *gin.Context) {
	var err error

	userID := c.Param("user-id")
	if !util.IsValidUUID(userID) {
		h.handleResponse(c, http.InvalidArgument, "user-id is an invalid uuid")
		return
	}

	clientTypeID := c.Query("client-type-id")
	if !util.IsValidUUID(clientTypeID) {
		h.handleResponse(c, http.InvalidArgument, "client type id is an invalid uuid")
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, http.BadRequest, errors.New("cant get project id"))
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, errors.New("cant get environment_id"))
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(c.Request.Context(), &pb.GetSingleServiceResourceReq{
		ProjectId:     projectId.(string),
		EnvironmentId: environmentId.(string),
		ServiceType:   pb.ServiceType_BUILDER_SERVICE,
	})
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	resp, err := h.services.UserService().V2GetUserByID(
		c.Request.Context(), &auth_service.UserPrimaryKey{
			Id:                    userID,
			ResourceEnvironmentId: resource.ResourceEnvironmentId,
			ResourceType:          int32(resource.GetResourceType()),
			ClientTypeId:          clientTypeID,
			ProjectId:             resource.GetProjectId(),
			NodeType:              resource.NodeType,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.OK, resp)
}

// @Security ApiKeyAuth
// V2UpdateUser godoc
// @ID update_user_v2
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/user [PUT]
// @Summary Update User
// @Description Update User
// @Tags V2_User
// @Accept json
// @Produce json
// @Param user body auth_service.UpdateUserRequest true "UpdateUserRequestBody"
// @Success 200 {object} http.Response{data=auth_service.User} "User data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2UpdateUser(c *gin.Context) {
	var (
		user auth_service.UpdateUserRequest
		resp *auth_service.User
	)

	if err := c.ShouldBindJSON(&user); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, http.BadRequest, "cant get project-id in query param")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, "cant get environment_id")
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(c.Request.Context(), &pb.GetSingleServiceResourceReq{
		EnvironmentId: environmentId.(string),
		ProjectId:     projectId.(string),
		ServiceType:   pb.ServiceType_BUILDER_SERVICE,
	})
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	user.ResourceType = int32(resource.GetResourceType())
	user.ResourceEnvironmentId = resource.ResourceEnvironmentId
	user.ProjectId = projectId.(string)
	user.EnvironmentId = environmentId.(string)
	user.NodeType = resource.NodeType

	userId, _ := c.Get("user_id")

	var (
		logReq = &models.CreateVersionHistoryRequest{
			NodeType:     user.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "UPDATE USER",
			UserInfo:     cast.ToString(userId),
			Request:      &user,
			TableSlug:    "USER",
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
		} else {
			logReq.Response = resp
		}
		go func() { _ = h.versionHistory(logReq) }()
	}()

	resp, err = h.services.UserService().V2UpdateUser(
		c.Request.Context(), &user,
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.OK, resp)
}

// @Security ApiKeyAuth
// V2DeleteUser godoc
// @ID delete_user_v2
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/user/{user-id} [DELETE]
// @Summary Delete User
// @Description Get User
// @Tags V2_User
// @Accept json
// @Produce json
// @Param user-id path string true "user-id"
// @Param project-id query string true "project-id"
// @Param client-type-id query string true "client-type-id"
// @Success 204
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2DeleteUser(c *gin.Context) {
	var (
		err    error
		userID = c.Param("user-id")
	)

	if !util.IsValidUUID(userID) {
		h.handleResponse(c, http.InvalidArgument, "user id is an invalid uuid")
		return
	}

	clientTypeID := c.Query("client-type-id")
	if clientTypeID == "" {
		h.handleResponse(c, http.InvalidArgument, "client type id is required")
		return
	}

	if !util.IsValidUUID(clientTypeID) {
		h.handleResponse(c, http.InvalidArgument, "client type id is an invalid uuid")
		return
	}

	projectID, ok := c.Get("project_id")
	if ok && !util.IsValidUUID(projectID.(string)) {
		h.handleResponse(c, http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, "cant get environment_id")
		return
	}

	project, err := h.services.ProjectServiceClient().GetById(c.Request.Context(), &pb.GetProjectByIdRequest{
		ProjectId: projectID.(string),
	})
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(c.Request.Context(), &pb.GetSingleServiceResourceReq{
		EnvironmentId: environmentId.(string),
		ProjectId:     projectID.(string),
		ServiceType:   pb.ServiceType_BUILDER_SERVICE,
	})
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	userId, _ := c.Get("user_id")
	var (
		logReq = &models.CreateVersionHistoryRequest{
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "DELETE USER",
			UsedEnvironments: map[string]bool{
				cast.ToString(resource.EnvironmentId): true,
			},
			UserInfo:  cast.ToString(userId),
			Request:   &userID,
			TableSlug: "USER",
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
		} else {
			logReq.Response = nil
		}
		go func() { _ = h.versionHistory(logReq) }()
	}()

	resp, err := h.services.UserService().V2DeleteUser(
		c.Request.Context(),
		&auth_service.UserPrimaryKey{
			Id:                    userID,
			ProjectId:             projectID.(string),
			ResourceType:          int32(resource.ResourceType),
			ResourceEnvironmentId: resource.ResourceEnvironmentId,
			ClientTypeId:          clientTypeID,
			CompanyId:             project.CompanyId,
		},
	)

	if err != nil {
		h.handleError(c, http.InternalServerError, err)
		return
	}

	h.handleResponse(c, http.NoContent, resp)
}

// @Security ApiKeyAuth
// V2DeleteFromAuthTable godoc
// @ID delete_user_v2
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/auth-table/{user-id} [DELETE]
// @Summary Delete From Auth service
// @Description Delete from auth table
// @Tags V2_User
// @Accept json
// @Produce json
// @Param user-id path string true "user-id"
// @Param project-id query string true "project-id"
// @Param client-type-id query string true "client-type-id"
// @Success 204
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2DeleteFromAuthTable(c *gin.Context) {
	var (
		userID       = c.Param("user-id")
		clientTypeID = c.Query("client-type-id")
	)

	if !util.IsValidUUID(userID) {
		h.handleResponse(c, http.InvalidArgument, "user id is an invalid uuid")
		return
	}

	if !util.IsValidUUID(clientTypeID) {
		h.handleResponse(c, http.InvalidArgument, "client type id is an invalid uuid")
		return
	}

	projectID, ok := c.Get("project_id")
	if ok && !util.IsValidUUID(projectID.(string)) {
		h.handleResponse(c, http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	_, err := h.services.SyncUserService().DeleteUser(
		c.Request.Context(), &auth_service.DeleteSyncUserRequest{
			UserId:       userID,
			ClientTypeId: clientTypeID,
			ProjectId:    projectID.(string),
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.NoContent, nil)
}

// AddUserToProject godoc
// @ID add user to project
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/add-user-to-project [POST]
// @Summary Create User
// @Description Create User
// @Tags V2_User
// @Accept json
// @Produce json
// @Param user body auth_service.AddUserToProjectReq true "AddUserToProjectReq"
// @Success 201 {object} http.Response{data=auth_service.AddUserToProjectRes} "User data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) AddUserToProject(c *gin.Context) {
	var (
		req           models.AddUserToProjectReq
		userDataToMap = make(map[string]any)
		tableSlug     = "users"
	)

	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Error("!!!AddUserToProject -> cant parse json")
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	if !util.IsValidUUID(req.ProjectId) {
		h.handleResponse(c, http.InvalidArgument, "project-id is an invalid uuid")
		return
	}

	if req.ClientTypeId == "" {
		h.handleResponse(c, http.InvalidArgument, "client_type_id is required")
		return
	}

	project, err := h.services.ProjectServiceClient().GetById(
		c.Request.Context(),
		&pb.GetProjectByIdRequest{ProjectId: req.ProjectId},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			EnvironmentId: req.EnvId,
			ProjectId:     req.ProjectId,
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		})
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	req.CompanyId = project.CompanyId

	res, err := h.services.UserService().AddUserToProject(
		c.Request.Context(), &auth_service.AddUserToProjectReq{
			UserId:       req.UserId,
			ProjectId:    req.ProjectId,
			RoleId:       req.RoleId,
			Active:       req.Active,
			CompanyId:    req.CompanyId,
			ResourceType: req.ResourceType,
			ClientTypeId: req.ClientTypeId,
			TableSlug:    req.TableSlug,
			EnvId:        req.EnvId,
			NodeType:     req.NodeType,
			Status:       req.Status,
		},
	)
	if err != nil {
		h.handleError(c, http.InternalServerError, err)
		return
	}

	userDataToMap["guid"] = uuid.NewString()
	userDataToMap["user_id_auth"] = req.UserId
	userDataToMap["guid"] = req.UserId
	userDataToMap["project_id"] = req.ProjectId
	userDataToMap["role_id"] = req.RoleId
	userDataToMap["client_type_id"] = req.ClientTypeId
	userDataToMap["from_auth_service"] = true

	maps.Copy(userDataToMap, req.AdditionalData)

	structData, err := helper.ConvertMapToStruct(userDataToMap)
	if err != nil {
		h.handleResponse(c, http.InvalidArgument, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(c, resource.ProjectId, resource.NodeType)
	if err != nil {
		h.handleResponse(c, http.InternalServerError, err.Error())
		return
	}

	switch int32(resource.ResourceType.Number()) {
	case 1:
		clientType, _ := services.GetObjectBuilderServiceByType(req.NodeType).GetSingle(
			c.Request.Context(),
			&obs.CommonMessage{
				TableSlug: "client_type",
				Data: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"id": structpb.NewStringValue(req.ClientTypeId),
					},
				},
				ProjectId: resource.ResourceEnvironmentId,
			},
		)

		clientTypeResponse := cast.ToStringMap(clientType.Data.AsMap()["response"])
		if clientTypeTableSlug, ok := clientTypeResponse["table_slug"].(string); ok {
			tableSlug = clientTypeTableSlug
		}
		_, err = services.GetObjectBuilderServiceByType(req.NodeType).Create(
			c.Request.Context(),
			&obs.CommonMessage{
				TableSlug: tableSlug,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, http.InternalServerError, err.Error())
			return
		}
	case 3:
		clientType, _ := services.GoItemService().GetSingle(
			c.Request.Context(),
			&nobs.CommonMessage{
				TableSlug: "client_type",
				Data: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"id": structpb.NewStringValue(req.ClientTypeId),
					},
				},
				ProjectId: resource.ResourceEnvironmentId,
			},
		)

		clientTypeResponse := cast.ToStringMap(clientType.Data.AsMap()["response"])
		if clientTypeTableSlug, ok := clientTypeResponse["table_slug"].(string); ok {
			tableSlug = clientTypeTableSlug
		}

		_, err = services.GoItemService().Create(
			c.Request.Context(),
			&nobs.CommonMessage{
				TableSlug: tableSlug,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, http.InternalServerError, err.Error())
			return
		}
	}

	res.EnvId = req.EnvId
	res.ProjectId = req.ProjectId

	h.handleResponse(c, http.Created, res)
}

// V2GetUserByLoginType godoc
// @ID get_user_by_login_type
// @Router /v2/user/check [POST]
// @Summary Get User By Login type
// @Description Get User By Login type
// @Tags V2_User
// @Accept json
// @Produce json
// @Param user-check body auth_service.GetUserByLoginTypesRequest true "user-check"
// @Success 200 {object} http.Response{data=auth_service.GetUserByLoginTypesResponse} "UserBody"
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2GetUserByLoginType(c *gin.Context) {
	var request auth_service.GetUserByLoginTypesRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	if request.Email != "" {
		var isValid = util.IsValidEmail(request.Email)
		if !isValid {
			err := errors.New("email is not valid")
			h.handleResponse(c, http.InvalidArgument, err.Error())
			return
		}
	}

	resp, err := h.services.UserService().V2GetUserByLoginTypes(c.Request.Context(), &request)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.OK, resp)
}

// V2UserResetPassword godoc
// @ID v2_user_reset_password
// @Router /v2/user/reset-password [PUT]
// @Summary Reset User password
// @Description Reset User Password
// @Tags V2_User
// @Accept json
// @Produce json
// @Param reset_password body auth_service.V2UserResetPasswordRequest true "ResetPasswordRequestBody"
// @Success 200 {object} http.Response{data=auth_service.User} "User data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2UserResetPassword(c *gin.Context) {
	var userPassword = &auth_service.V2UserResetPasswordRequest{}

	if err := c.ShouldBindJSON(&userPassword); err != nil {
		h.handleResponse(c, http.BadRequest, err)
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, errors.New("cant get environment_id"))
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	userPassword.EnvironmentId = environmentId.(string)
	userPassword.ProjectId = projectId.(string)
	userPassword.NodeType = resource.NodeType
	userPassword.ResourceEnvironmentId = resource.ResourceEnvironmentId
	userPassword.ResourceType = int32(resource.GetResourceType())

	user, err := h.services.UserService().V2ResetPassword(c.Request.Context(), userPassword)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.OK, user)
}

func (h *Handler) SyncUser(c *gin.Context) {
	var (
		user auth_service.CreateSyncUserRequest
		resp *auth_service.SyncUserResponse
	)

	if err := c.ShouldBindJSON(&user); err != nil {
		h.handleError(c, http.BadRequest, err)
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleError(c, http.BadRequest, config.ErrEnvironmentIdValid)
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleError(c, http.BadRequest, config.ErrProjectIdValid)
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			EnvironmentId: environmentId.(string),
			ProjectId:     projectId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		})
	if err != nil {
		h.handleResponse(c, http.GRPCError, err)
		return
	}

	user.ResourceEnvironmentId = resource.ResourceEnvironmentId
	user.EnvironmentId = resource.EnvironmentId
	user.ProjectId = resource.ProjectId

	resp, err = h.services.SyncUserService().CreateUser(
		c.Request.Context(), &user,
	)
	if err != nil {
		h.handleError(c, http.InternalServerError, err)
		return
	}

	h.handleResponse(c, http.Created, resp)
}
