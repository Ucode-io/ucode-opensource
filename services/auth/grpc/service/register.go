package service

import (
	"context"
	"errors"
	"runtime"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	"github.com/Ucode-io/ucode-opensource/services/auth/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"
	span "github.com/Ucode-io/ucode-opensource/services/auth/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/security"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"

	"github.com/google/uuid"
	"github.com/saidamir98/udevs_pkg/logger"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	pbc "github.com/Ucode-io/ucode-opensource/services/auth/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/new_object_builder_service"
	pbObject "github.com/Ucode-io/ucode-opensource/services/auth/genproto/object_builder_service"
)

type registerService struct {
	cfg         config.BaseConfig
	log         logger.LoggerI
	strg        storage.StorageI
	services    client.ServiceManagerI
	serviceNode ServiceNodesI
	pb.UnimplementedRegisterServiceServer
}

func NewRegisterService(cfg config.BaseConfig, log logger.LoggerI, strg storage.StorageI, svcs client.ServiceManagerI, projectServiceNodes ServiceNodesI) *registerService {
	return &registerService{
		cfg:         cfg,
		log:         log,
		strg:        strg,
		services:    svcs,
		serviceNode: projectServiceNodes,
	}
}

func addUserProjectForRegistration(ctx context.Context, repo storage.UserRepoI, req *pb.AddUserToProjectReq) (bool, error) {
	_, err := repo.AddUserToProject(ctx, req)
	if err == nil {
		return true, nil
	}
	if strings.Contains(err.Error(), config.UserProjectIdConstraint) {
		return false, nil
	}
	return false, err
}

func (rs *registerService) RegisterUser(ctx context.Context, data *pb.RegisterUserRequest) (*pb.V2LoginResponse, error) {
	rs.log.Info("--RegisterUser invoked--", logger.Any("data", data))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_register.RegisterUser", data)
	defer dbSpan.Finish()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		rs.log.Info("Memory used by the RegisterUser", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			rs.log.Info("Memory used over 300 mb", logger.Any("RegisterUser", memoryUsed))
		}
	}()

	var (
		body = data.Data.AsMap()

		foundUser                   *pb.User
		err                         error
		userId, objectBuilderUserId string
		userData                    *pbObject.LoginDataRes

		login    = helper.GetStringFromMap(body, "login")
		email    = helper.GetStringFromMap(body, "email")
		password = helper.GetStringFromMap(body, "password")
		phone    = helper.GetStringFromMap(body, "phone")
	)

	switch strings.ToLower(data.Type) {
	case config.WithEmail:
		foundUser, err = rs.strg.User().GetByUsername(ctx, email)
		if err != nil {
			rs.log.Error("!!!RegisterUserError-->EmailGetByUsername", logger.Error(err))
			return nil, err
		}
	case config.WithPhone:
		foundUser, err = rs.strg.User().GetByUsername(ctx, phone)
		if err != nil {
			rs.log.Error("!RegisterUserError--->PhoneGetByUsernames", logger.Error(err))
			return nil, err
		}
	case config.WithLogin:
		if len(login) < 6 {
			err := config.ErrInvalidUsername
			rs.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		foundUser, err = rs.strg.User().GetByUsername(ctx, login)
		if err != nil {
			rs.log.Error("!RegisterUserError--->LoginGetByUsername", logger.Error(err))
			return nil, err
		}
	}

	userId = foundUser.GetId()

	if len(foundUser.GetId()) == 0 {
		if !helper.EmailValidation(email) && len(email) > 0 {
			err = config.ErrInvalidEmail
			rs.log.Error("!!!CreateUser--->EmailValidation", logger.Error(err))
			return nil, err
		}

		if len(password) > 0 {
			hashedPassword, err := security.HashPasswordBcrypt(password)
			if err != nil {
				rs.log.Error("!!!CreateUser--->HashPassword", logger.Error(err))
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			password = hashedPassword
		}

		pKey, err := rs.strg.User().Create(ctx, &pb.CreateUserRequest{
			Login:     login,
			Password:  password,
			Email:     email,
			Phone:     phone,
			CompanyId: data.GetCompanyId(),
		})
		if err != nil {
			rs.log.Error("!!!CreateUser--->CreateRequest", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		userId = pKey.GetId()
	}

	body["guid"] = uuid.NewString()
	body["from_auth_service"] = true
	body["user_id_auth"] = userId
	structData, err := helper.ConvertMapToStruct(body)
	if err != nil {
		rs.log.Error("!!!CreateUser--->ConvertMapToStruct", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	services, err := rs.serviceNode.GetByNodeType(data.ProjectId, data.NodeType)
	if err != nil {
		rs.log.Error("!!!CreateUser-->GetByNodeType", logger.Error(err))
		return nil, err
	}

	var (
		resourceType = body["resource_type"].(float64)
		tableSlug    = "user"
	)

	projForLimit, err := rs.services.ProjectServiceClient().GetById(ctx, &pbc.GetProjectByIdRequest{ProjectId: data.ProjectId})
	if err != nil {
		rs.log.Error("!!!RegisterUser--->GetProjectById", logger.Error(err))
		return nil, status.Error(codes.Internal, "error getting project info")
	}

	if limitErr := checkUserProjectLimit(ctx, rs.services, rs.strg, projForLimit.GetFareId(), projForLimit.GetCompanyId()); limitErr != nil {
		rs.log.Error("!!!RegisterUser--->checkUserProjectLimit", logger.Error(limitErr))
		return nil, limitErr
	}

	userProjectCreated, err := addUserProjectForRegistration(ctx, rs.strg.User(), &pb.AddUserToProjectReq{
		UserId:       userId,
		RoleId:       data.RoleId,
		CompanyId:    data.CompanyId,
		ProjectId:    data.ProjectId,
		ClientTypeId: data.ClientTypeId,
		EnvId:        data.EnvironmentId,
	})
	if err != nil {
		rs.log.Error("!RegisterUserError--->AddUserToProject", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !userProjectCreated {
		// The auth membership can outlive a failed builder record creation (for
		// example, when a downstream request fails after AddUserToProject). A
		// retry must continue and create the missing project user instead of
		// getting stuck behind the user_project unique constraint forever.
		rs.log.Info("RegisterUser: user project membership already exists; continuing registration")
	}

	rollbackUserProject := func() {
		if !userProjectCreated {
			return
		}

		_, rollbackErr := rs.strg.User().DeleteUserFromProject(context.WithoutCancel(ctx), &pb.DeleteSyncUserRequest{
			UserId:        userId,
			RoleId:        data.RoleId,
			CompanyId:     data.CompanyId,
			ProjectId:     data.ProjectId,
			ClientTypeId:  data.ClientTypeId,
			EnvironmentId: data.EnvironmentId,
		})
		if rollbackErr != nil {
			rs.log.Error("!!!RegisterUser--->RollbackUserProject", logger.Error(rollbackErr))
			return
		}

		userProjectCreated = false
	}

	switch resourceType {
	case 1:
		response, err := services.GetObjectBuilderServiceByType(data.NodeType).GetSingle(ctx, &pbObject.CommonMessage{
			TableSlug: "client_type",
			ProjectId: data.ResourceEnvironmentId,
			Data: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"id": structpb.NewStringValue(data.ClientTypeId),
				},
			},
		})
		if err != nil {
			rollbackUserProject()
			rs.log.Error("!!!CreateUser--->Node GetSingle", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		clientType, ok := response.Data.AsMap()["response"]
		if ok && clientType != nil {
			if clientTypeTableSlug, ok := clientType.(map[string]any)["table_slug"]; ok {
				tableSlug = cast.ToString(clientTypeTableSlug)
			}
		}

		getResp, err := services.GetObjectBuilderServiceByType(data.NodeType).Create(ctx, &pbObject.CommonMessage{
			TableSlug: tableSlug,
			Data:      structData,
			ProjectId: data.ResourceEnvironmentId,
		})
		if err != nil {
			rollbackUserProject()
			rs.log.Error("!!!CreateUser--->NodeType Create", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		objectBuilderUserId = cast.ToString(cast.ToStringMap(getResp.Data.AsMap()["data"])["guid"])
	case 3:
		response, err := services.GoItemService().GetSingle(ctx, &new_object_builder_service.CommonMessage{
			TableSlug: "client_type",
			ProjectId: data.ResourceEnvironmentId,
			Data: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"id": structpb.NewStringValue(data.ClientTypeId),
				},
			},
		})
		if err != nil {
			rollbackUserProject()
			rs.log.Error("!!!CreateUser--->GetSingle", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		if clientType, ok := response.Data.AsMap()["response"]; ok {
			if clientTypeTableSlug, ok := clientType.(map[string]any)["table_slug"]; ok {
				tableSlug = cast.ToString(clientTypeTableSlug)
			}
		}

		getResp, err := services.GoItemService().Create(ctx, &new_object_builder_service.CommonMessage{
			TableSlug: tableSlug,
			Data:      structData,
			ProjectId: data.ResourceEnvironmentId,
		})
		if err != nil {
			rollbackUserProject()
			rs.log.Error("!!!PostgresObjectBuilderService.CreateUser--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		objectBuilderUserId = cast.ToString(getResp.Data.AsMap()["guid"])
	}

	reqLoginData := &pbObject.LoginDataReq{
		UserId:                userId,
		ProjectId:             data.ProjectId,
		ClientType:            data.ClientTypeId,
		ResourceEnvironmentId: data.ResourceEnvironmentId,
	}

	switch resourceType {
	case 1:
		userData, err = services.GetLoginServiceByType(data.NodeType).LoginData(ctx, reqLoginData)
		if err != nil {
			errGetUserProjectData := errors.New("invalid user project data")
			rs.log.Error("!!!Login--->LoginData", logger.Error(err))
			return nil, status.Error(codes.Internal, errGetUserProjectData.Error())
		}
	case 3:
		pgLoginData := &new_object_builder_service.LoginDataReq{}

		err := helper.MarshalToStruct(reqLoginData, &pgLoginData)
		if err != nil {
			rs.log.Error("!!!PostgresBuilder.Login--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		pgUserData, err := services.GoLoginService().LoginData(
			ctx,
			pgLoginData,
		)
		if err != nil {
			errGetUserProjectData := errors.New("invalid user project data")
			rs.log.Error("!!!PostgresBuilder.Login--->", logger.Error(err))
			return nil, status.Error(codes.Internal, errGetUserProjectData.Error())
		}

		err = helper.MarshalToStruct(&pgUserData, &userData)
		if err != nil {
			rs.log.Error("!!!Login--->", logger.Error(err))
			return nil, status.Error(codes.Internal, "invalid user project data")
		}
	}

	if !userData.UserFound {
		rs.log.Error("!!!Login--->", logger.Error(config.ErrUserNotFound))
		return nil, status.Error(codes.NotFound, config.ErrUserNotFound.Error())
	}

	res := helper.ConvertPbToAnotherPb(&pbObject.V2LoginResponse{
		Role:           userData.GetRole(),
		UserId:         userData.GetUserId(),
		UserFound:      userData.GetUserFound(),
		ClientType:     userData.GetClientType(),
		UserIdAuth:     userData.GetUserIdAuth(),
		Permissions:    userData.GetPermissions(),
		ClientPlatform: userData.GetClientPlatform(),
		LoginTableSlug: userData.GetLoginTableSlug(),
	})

	res, err = rs.services.SessionService().SessionAndTokenGenerator(ctx, &pb.SessionAndTokenRequest{
		LoginData:     res,
		ProjectId:     data.ProjectId,
		Tables:        []*pb.Object{},
		EnvironmentId: data.EnvironmentId,
		ClientId:      data.ClientId,
		ClientIp:      data.ClientIp,
		UserAgent:     data.UserAgent,
	})
	if res == nil {
		rs.log.Error("!!!Login--->SessionAndTokenGenerator", logger.Error(config.ErrUserNotFound))
		return nil, status.Error(codes.NotFound, config.ErrUserNotFound.Error())
	}
	if err != nil {
		rs.log.Error("!!!Login--->SessionAndTokenGenerator", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	res.GlobalPermission = nil
	res.UserData = nil
	res.UserFound = true
	res.UserId = objectBuilderUserId

	return res, nil
}
