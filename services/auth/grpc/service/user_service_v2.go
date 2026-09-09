package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/smtp"
	"regexp"
	"runtime"
	"github.com/Ucode-io/ucode-opensource/services/auth/api/models"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"

	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	pbc "github.com/Ucode-io/ucode-opensource/services/auth/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/new_object_builder_service"
	pbObject "github.com/Ucode-io/ucode-opensource/services/auth/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"
	span "github.com/Ucode-io/ucode-opensource/services/auth/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/security"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/util"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/saidamir98/udevs_pkg/logger"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

func (s *userService) RegisterWithGoogle(ctx context.Context, req *pb.RegisterWithGoogleRequest) (resp *pb.User, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_userv2.RegisterWithGoogle", req)
	defer dbSpan.Finish()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the RegisterWithGoogle", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("RegisterWithGoogle", memoryUsed))
		}
	}()

	emailRegex := regexp.MustCompile(config.EMAIL_REGEX)
	email := emailRegex.MatchString(req.Email)
	if !email {
		err = config.ErrInvalidEmail
		s.log.Error("!!!CreateUser--->", logger.Error(err))
		return nil, err
	}

	foundUser, err := s.strg.User().GetByUsername(ctx, req.Email)
	if err != nil {
		s.log.Error("!!!Get User by name--->", logger.Error(err))
		return nil, err
	}

	services, err := s.serviceNode.GetByNodeType(
		req.ProjectId,
		req.NodeType,
	)
	if err != nil {
		return nil, err
	}

	if foundUser.Id == "" {
		projForLimit, err := s.services.ProjectServiceClient().GetById(ctx, &pbc.GetProjectByIdRequest{ProjectId: req.GetProjectId()})
		if err != nil {
			s.log.Error("!!!RegisterWithGoogle--->GetProjectById", logger.Error(err))
			return nil, status.Error(codes.Internal, "error getting project info")
		}

		if limitErr := checkUserProjectLimit(ctx, s.services, s.strg, projForLimit.GetFareId(), projForLimit.GetCompanyId()); limitErr != nil {
			s.log.Error("!!!RegisterWithGoogle--->checkUserProjectLimit", logger.Error(limitErr))
			return nil, limitErr
		}

		pKey, err := s.strg.User().Create(ctx, &pb.CreateUserRequest{
			Login:                 "",
			Password:              "",
			Email:                 req.Email,
			Phone:                 "",
			Name:                  req.GetName(),
			CompanyId:             req.GetCompanyId(),
			ProjectId:             req.GetProjectId(),
			ResourceEnvironmentId: req.GetResourceEnvironmentId(),
			RoleId:                "",
			ClientTypeId:          req.GetClientTypeId(),
			ClientPlatformId:      "",
			Active:                -1,
		})
		if err != nil {
			s.log.Error("!!!CreateUser--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		structData, err := helper.ConvertRequestToSturct(map[string]any{
			"guid":           pKey.GetId(),
			"project_id":     req.GetProjectId(),
			"role_id":        "",
			"client_type_id": req.GetClientTypeId(),
			"active":         "",
			"expires_at":     "",
			"email":          req.GetEmail(),
			"phone":          "",
			"name":           req.GetName(),
			"login":          "",
		})
		if err != nil {
			s.log.Error("!!!CreateUser--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		switch req.ResourceType {
		case 1:
			_, err = services.GetObjectBuilderServiceByType(req.NodeType).Create(ctx, &pbObject.CommonMessage{
				TableSlug: "user",
				Data:      structData,
				ProjectId: req.GetResourceEnvironmentId(),
			})
			if err != nil {
				s.log.Error("!!!ObjectBuilderService.CreateUser--->", logger.Error(err))
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
		}

		_, err = s.strg.User().AddUserToProject(ctx, &pb.AddUserToProjectReq{
			UserId:    pKey.Id,
			ProjectId: req.GetProjectId(),
			CompanyId: req.GetCompanyId(),
		})
		if err != nil {
			s.log.Error("!!!CreateUser--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		resp, err = s.strg.User().GetByPK(ctx, &pb.UserPrimaryKey{
			Id: pKey.Id,
		})
		if err != nil {
			s.log.Error("!!!CreateUser--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		return resp, err
	} else {
		var objUser *pbObject.V2LoginResponse

		if req.Email != "" {
			switch req.ResourceType {
			case 1:
				objUser, err = services.GetLoginServiceByType(req.NodeType).LoginWithEmailOtp(ctx, &pbObject.EmailOtpRequest{
					Email:      req.Email,
					ClientType: "WEB_USER",
					ProjectId:  req.GetResourceEnvironmentId(),
					TableSlug:  "user",
				})
				if err != nil {
					s.log.Error("!!!Found user from obj--->", logger.Error(err))
					return nil, status.Error(codes.InvalidArgument, err.Error())
				}
			}

		}

		if objUser.UserFound {
			s.log.Error("!!!Found user from obj--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, "User already exists")
		} else {
			structData, err := helper.ConvertRequestToSturct(map[string]any{
				"guid":           foundUser.Id,
				"project_id":     req.GetProjectId(),
				"role_id":        "",
				"client_type_id": req.GetClientTypeId(),
				"active":         "",
				"expires_at":     "",
				"email":          req.GetEmail(),
				"phone":          "",
				"name":           req.GetName(),
				"login":          "",
			})
			if err != nil {
				s.log.Error("!!!CreateUser--->", logger.Error(err))
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}

			switch req.ResourceType {
			case 1:
				_, err = services.GetObjectBuilderServiceByType(req.NodeType).Create(ctx, &pbObject.CommonMessage{
					TableSlug: "user",
					Data:      structData,
					ProjectId: req.GetResourceEnvironmentId(),
				})
				if err != nil {
					s.log.Error("!!!CreateUser--->", logger.Error(err))
					return nil, status.Error(codes.InvalidArgument, err.Error())
				}
			}

			resp, err = s.strg.User().GetByPK(ctx, &pb.UserPrimaryKey{
				Id: foundUser.Id,
			})
			if err != nil {
				s.log.Error("!!!CreateUser--->", logger.Error(err))
				return nil, status.Error(codes.Internal, err.Error())
			}

			return resp, err
		}
	}
}

func (s *userService) RegisterUserViaEmail(ctx context.Context, req *pb.CreateUserRequest) (resp *pb.User, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_userv2.RegisterUserViaEmail", req)
	defer dbSpan.Finish()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the RegisterUserViaEmail", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("RegisterUserViaEmail", memoryUsed))
		}
	}()

	hashedPassword, err := security.HashPasswordBcrypt(req.Password)
	if err != nil {
		s.log.Error("!!!CreateUser--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	req.Password = hashedPassword

	emailRegex := regexp.MustCompile(config.EMAIL_REGEX)
	email := emailRegex.MatchString(req.Email)
	if !email {
		err = config.ErrInvalidEmail
		s.log.Error("!!!CreateUser--->EmailRegex", logger.Error(err))
		return nil, err
	}

	foundUser, _ := s.strg.User().GetByUsername(ctx, req.Email)
	if foundUser.Id == "" {
		foundUser, _ = s.strg.User().GetByUsername(ctx, req.Phone)
	}

	services, err := s.serviceNode.GetByNodeType(req.ProjectId, req.NodeType)
	if err != nil {
		s.log.Error("!!!CreateUser--->GetByNodeType", logger.Error(err))
		return nil, err
	}

	if foundUser.Id == "" {
		projForLimit, err := s.services.ProjectServiceClient().GetById(ctx, &pbc.GetProjectByIdRequest{ProjectId: req.GetProjectId()})
		if err != nil {
			s.log.Error("!!!RegisterUserViaEmail--->GetProjectById", logger.Error(err))
			return nil, status.Error(codes.Internal, "error getting project info")
		}
		if limitErr := checkUserProjectLimit(ctx, s.services, s.strg, projForLimit.GetFareId(), projForLimit.GetCompanyId()); limitErr != nil {
			s.log.Error("!!!RegisterUserViaEmail--->checkUserProjectLimit", logger.Error(limitErr))
			return nil, limitErr
		}

		pKey, err := s.strg.User().Create(ctx, &pb.CreateUserRequest{
			Login:                 req.GetLogin(),
			Password:              req.GetPassword(),
			Phone:                 req.GetPhone(),
			Name:                  req.GetName(),
			CompanyId:             req.GetCompanyId(),
			ProjectId:             req.GetProjectId(),
			ResourceEnvironmentId: req.GetResourceEnvironmentId(),
			RoleId:                req.GetRoleId(),
			ClientTypeId:          req.GetClientTypeId(),
			ClientPlatformId:      req.GetClientPlatformId(),
			Active:                req.GetActive(),
		})
		if err != nil {
			s.log.Error("!!!CreateUser--->UserCreate", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		structData, err := helper.ConvertRequestToSturct(map[string]any{
			"guid":           pKey.GetId(),
			"project_id":     req.GetProjectId(),
			"role_id":        req.GetRoleId(),
			"client_type_id": req.GetClientTypeId(),
			"active":         req.GetActive(),
			"expires_at":     req.GetExpiresAt(),
			"email":          req.GetEmail(),
			"phone":          req.GetPhone(),
			"name":           req.GetName(),
			"login":          req.GetLogin(),
		})
		if err != nil {
			s.log.Error("!!!CreateUser--->ConvertReq", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		switch req.ResourceType {
		case 1:
			_, err = services.GetObjectBuilderServiceByType(req.NodeType).Create(ctx, &pbObject.CommonMessage{
				TableSlug: "user",
				Data:      structData,
				ProjectId: req.GetResourceEnvironmentId(),
			})
			if err != nil {
				s.log.Error("!!!CreateUser--->", logger.Error(err))
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
		}

		_, err = s.strg.User().AddUserToProject(ctx, &pb.AddUserToProjectReq{
			UserId:    pKey.Id,
			ProjectId: req.GetProjectId(),
			CompanyId: req.GetCompanyId(),
		})
		if err != nil {
			s.log.Error("!!!CreateUser--->AddUserToProject", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		resp, err = s.strg.User().GetByPK(ctx, &pb.UserPrimaryKey{
			Id: pKey.Id,
		})
		if err != nil {
			s.log.Error("!!!CreateUser-->UserGetByPK", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		return resp, err
	} else {
		var objUser *pbObject.V2LoginResponse

		if req.Email != "" {
			switch req.ResourceType {
			case 1:
				objUser, err = services.GetLoginServiceByType(req.NodeType).LoginWithEmailOtp(ctx, &pbObject.EmailOtpRequest{
					Email:      req.Email,
					ClientType: "WEB_USER",
					ProjectId:  req.GetResourceEnvironmentId(),
					TableSlug:  "user",
				})
				if err != nil {
					s.log.Error("!!!Found user from obj--->", logger.Error(err))
					return nil, status.Error(codes.InvalidArgument, err.Error())
				}
			}

		}
		if objUser != nil && req.Phone != "" && !objUser.UserFound {
			switch req.ResourceType {
			case 1:
				objUser, err = services.GetLoginServiceByType(req.NodeType).LoginWithOtp(ctx, &pbObject.PhoneOtpRequst{
					PhoneNumber: req.Phone,
					ClientType:  "WEB_USER",
					ProjectId:   req.GetResourceEnvironmentId(),
				})
				if err != nil {
					s.log.Error("!!!Found user from obj--->", logger.Error(err))
					return nil, status.Error(codes.InvalidArgument, err.Error())
				}
			}

		}

		if objUser != nil && objUser.UserFound {
			s.log.Error("!!!Found user from obj--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, "User already exists")
		} else {
			structData, err := helper.ConvertRequestToSturct(map[string]any{
				"guid":           foundUser.Id,
				"project_id":     req.GetProjectId(),
				"role_id":        req.GetRoleId(),
				"client_type_id": req.GetClientTypeId(),
				"active":         req.GetActive(),
				"expires_at":     req.GetExpiresAt(),
				"email":          req.GetEmail(),
				"phone":          req.GetPhone(),
				"name":           req.GetName(),
				"login":          req.GetLogin(),
			})
			if err != nil {
				s.log.Error("!!!CreateUser--->", logger.Error(err))
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}

			switch req.ResourceType {
			case 1:
				_, err = services.GetObjectBuilderServiceByType(req.NodeType).Create(ctx, &pbObject.CommonMessage{
					TableSlug: "user",
					Data:      structData,
					ProjectId: req.GetResourceEnvironmentId(),
				})
				if err != nil {
					s.log.Error("!!!CreateUser--->", logger.Error(err))
					return nil, status.Error(codes.InvalidArgument, err.Error())
				}
			}

			resp, err = s.strg.User().GetByPK(ctx, &pb.UserPrimaryKey{
				Id: foundUser.Id,
			})
			if err != nil {
				s.log.Error("!!!CreateUser--->", logger.Error(err))
				return nil, status.Error(codes.Internal, err.Error())
			}

			return resp, err
		}
	}
}

func ConvertStructToMap(s *structpb.Struct) (map[string]any, error) {
	newMap := make(map[string]any)

	body, err := json.Marshal(s)
	if err != nil {
		return map[string]any{}, err
	}
	if err := json.Unmarshal(body, &newMap); err != nil {
		return map[string]any{}, err
	}

	return newMap, nil
}

func (s *userService) V2CreateUser(ctx context.Context, req *pb.CreateUserRequest) (resp *pb.User, err error) {
	s.log.Info("!!!V2CreateUser--->", logger.Any("req", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_userv2.V2CreateUser", req)
	defer dbSpan.Finish()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2CreateUser", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2CreateUser", memoryUsed))
		}
	}()

	originalPassword := req.GetPassword()
	hashedPassword, err := security.HashPasswordBcrypt(req.Password)
	if err != nil {
		s.log.Error("!!!V2CreateUser--->HashPassword", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, config.ErrPasswordHash)
	}

	if len(req.GetClientTypeId()) == 0 || len(req.GetRoleId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, config.ErrClientTypeRoleIDRequired)
	}

	var (
		emailRegex                 = regexp.MustCompile(config.EMAIL_REGEX)
		email                      = emailRegex.MatchString(req.Email)
		password                   = req.Password
		userId                     string
		tableSlug                  = "user"
		userCreated, alreadyHashed bool
		user                       *pb.User
	)

	req.Password = hashedPassword

	if !email && req.Email != "" {
		s.log.Error("!!!V2CreateUser--->EmailRegex", logger.Any("error", config.ErrInvalidUserEmail))
		return nil, status.Error(codes.InvalidArgument, config.ErrInvalidUserEmail)
	}

	if len(req.GetLogin()) != 0 {
		user, err = s.strg.User().GetByUsername(ctx, req.GetLogin())
		if err != nil {
			return nil, err
		}

		userId = user.GetId()
	}

	if len(req.GetPhone()) != 0 && len(req.GetLogin()) == 0 {
		user, err = s.strg.User().GetByUsername(ctx, req.GetPhone())
		if err != nil {
			return nil, err
		}

		userId = user.GetId()
	}

	if len(req.GetEmail()) != 0 && len(req.GetLogin()) == 0 {
		user, err = s.strg.User().GetByUsername(ctx, req.GetEmail())
		if err != nil {
			return nil, err
		}

		userId = user.GetId()
	}

	project, err := s.services.ProjectServiceClient().GetById(ctx, &pbc.GetProjectByIdRequest{ProjectId: req.GetProjectId()})
	if err != nil {
		s.log.Error("!!!CreateUser-->ProjectGetById", logger.Error(err))
		return nil, err
	}

	var (
		seatCharge    *userSeatCharge
		seatCommitted bool
	)

	if project.GetPerUserPrice() > 0 {
		if userId != "" {
			exists, existErr := s.strg.User().GetUserProjectByAllFields(ctx, models.GetUserProjectByAllFieldsReq{
				UserId:       userId,
				RoleId:       req.GetRoleId(),
				ProjectId:    req.GetProjectId(),
				ClientTypeId: req.GetClientTypeId(),
				CompanyId:    project.GetCompanyId(),
				EnvId:        req.GetEnvironmentId(),
			})
			if existErr != nil {
				s.log.Error("!!!V2CreateUser--->GetUserProjectByAllFields", logger.Error(existErr))
				return nil, existErr
			}
			if exists {
				return nil, errors.New(config.ErrUserExists)
			}
		}

		seatCharge, err = s.reserveUserSeat(ctx, project, "")
		if err != nil {
			return nil, err
		}

		defer func() {
			if err != nil && !seatCommitted {
				s.releaseUserSeat(seatCharge)
			}
		}()
	} else {
		if limitErr := checkUserProjectLimit(ctx, s.services, s.strg, project.GetFareId(), project.GetCompanyId()); limitErr != nil {
			s.log.Error("!!!V2CreateUser--->checkUserProjectLimit", logger.Error(limitErr))
			return nil, limitErr
		}

		if project.GetIsUgen() {
			if limitErr := checkUgenBuildersLimit(ctx, s.services, s.strg, project.GetFareId(), project.GetProjectId()); limitErr != nil {
				s.log.Error("!!!V2CreateUser--->checkUgenBuildersLimit", logger.Error(limitErr))
				return nil, limitErr
			}
		}
	}

	if len(userId) == 0 {
		user, err := s.strg.User().Create(ctx, &pb.CreateUserRequest{
			Login:     req.GetLogin(),
			Email:     req.GetEmail(),
			Phone:     req.GetPhone(),
			Password:  req.GetPassword(),
			CompanyId: project.GetCompanyId(),
			Name:      req.GetName(),
		})
		if err != nil {
			s.log.Error("!!!CreateUser--->UserCreate", logger.Error(err))
			return nil, err
		}
		userId = user.GetId()
		userCreated = true

		_, err = s.strg.User().AddUserToProject(ctx, &pb.AddUserToProjectReq{
			UserId:       userId,
			RoleId:       req.GetRoleId(),
			ProjectId:    req.GetProjectId(),
			ClientTypeId: req.GetClientTypeId(),
			CompanyId:    project.GetCompanyId(),
			EnvId:        req.GetEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!CreateUser--->AddUserToProject", logger.Error(err))
			return nil, err
		}
	} else {
		alreadyHashed = true
		password = user.GetPassword()

		exists, err := s.strg.User().GetUserProjectByAllFields(ctx, models.GetUserProjectByAllFieldsReq{
			UserId:       userId,
			RoleId:       req.GetRoleId(),
			ProjectId:    req.GetProjectId(),
			ClientTypeId: req.GetClientTypeId(),
			CompanyId:    project.GetCompanyId(),
			EnvId:        req.GetEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!CreateUser--->GetUserProjectByAllFields", logger.Error(err))
			return nil, err
		}

		if !exists {
			_, err = s.strg.User().AddUserToProject(ctx, &pb.AddUserToProjectReq{
				UserId:       userId,
				RoleId:       req.GetRoleId(),
				ProjectId:    req.GetProjectId(),
				ClientTypeId: req.GetClientTypeId(),
				EnvId:        req.GetEnvironmentId(),
				CompanyId:    project.GetCompanyId(),
			})
			if err != nil {
				s.log.Error("!!!CreateUser--->AddUserToProjectExists", logger.Error(err))
				return nil, err
			}
		} else {
			return nil, errors.New(config.ErrUserExists)
		}
	}

	requestData := map[string]any{
		"guid":               uuid.New().String(),
		"name":               req.GetName(),
		"login":              req.GetLogin(),
		"email":              req.GetEmail(),
		"phone":              req.GetPhone(),
		"password":           password,
		"project_id":         req.GetProjectId(),
		"role_id":            req.GetRoleId(),
		"client_type_id":     req.GetClientTypeId(),
		"photo":              req.GetPhotoUrl(),
		"birth_day":          req.GetYearOfBirth(),
		"active":             req.GetActive(),
		"user_id_auth":       userId,
		"from_auth_service":  true,
		"expires_at":         req.GetExpiresAt(),
		"client_platform_id": req.GetClientPlatformId(),
		"already_hashed":     alreadyHashed,
	}

	additionalData, err := ConvertStructToMap(req.AdditionalData)
	if err != nil {
		s.log.Error("!!!CreateUser-->ConvertStructToMap", logger.Error(err))
		return nil, err
	}

	maps.Copy(requestData, additionalData)

	// objectBuilder -> auth service
	structData, err := helper.ConvertRequestToSturct(requestData)
	if err != nil {
		s.log.Error("!!!V2CreateUser--->ConvertReq", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	services, err := s.serviceNode.GetByNodeType(req.ProjectId, req.NodeType)
	if err != nil {
		s.log.Error("!!!V2CreateUser--->GetByNodeType", logger.Error(err))
		return nil, err
	}

	switch req.ResourceType {
	case int32(pbc.ResourceType_MONGODB):
		clientType, err := services.GetObjectBuilderServiceByType(req.NodeType).GetSingle(ctx, &pbObject.CommonMessage{
			TableSlug: "client_type",
			Data:      &structpb.Struct{Fields: map[string]*structpb.Value{"id": structpb.NewStringValue(req.GetClientTypeId())}},
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!V2CreateUser--->GetSingle", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		response, ok := clientType.Data.AsMap()["response"].(map[string]any)
		if ok {
			clientTypeTableSlug, ok := response["table_slug"].(string)
			if ok && clientTypeTableSlug != "" {
				tableSlug = clientTypeTableSlug
			}
		}

		_, err = services.GetObjectBuilderServiceByType(req.NodeType).Create(ctx, &pbObject.CommonMessage{
			TableSlug: tableSlug,
			Data:      structData,
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			if userCreated {
				_, _ = s.strg.User().Delete(ctx, &pb.UserPrimaryKey{
					Id:        userId,
					ProjectId: req.GetResourceEnvironmentId(),
				})
			}
			s.log.Error("!!!V2CreateUser--->CreateObj", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
	case int32(pbc.ResourceType_POSTGRESQL):
		clientType, err := services.GoItemService().GetSingle(ctx, &nb.CommonMessage{
			TableSlug: "client_type",
			Data:      &structpb.Struct{Fields: map[string]*structpb.Value{"id": structpb.NewStringValue(req.GetClientTypeId())}},
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!V2CreateUser--->GetSingle", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		response, ok := clientType.Data.AsMap()["response"].(map[string]any)
		if ok {
			clientTypeTableSlug, ok := response["table_slug"].(string)
			if ok && clientTypeTableSlug != "" {
				tableSlug = clientTypeTableSlug
			}
		}

		_, err = services.GoItemService().Create(ctx, &nb.CommonMessage{
			TableSlug: tableSlug,
			Data:      structData,
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			if userCreated {
				_, err := s.strg.User().Delete(ctx, &pb.UserPrimaryKey{
					Id:        userId,
					ProjectId: req.GetResourceEnvironmentId(),
					IsTest:    true,
				})
				s.log.Error("!!!V2CreateUser->DeleteUser", logger.Error(err))
			}

			s.log.Error("!!!V2CreateUser->CreateObj", logger.Error(err))
			return nil, err
		}
	}

	// The user is now durably created and added to the project: the paid seat is
	// consumed, so later failures (e.g. the invite email) must not refund it.
	seatCommitted = true

	if req.GetEmail() != "" {
		host := "smtp.gmail.com"
		hostPort := ":587"

		to := req.GetEmail()
		login := req.GetLogin()
		password := originalPassword

		subject := "You're Invited – Access Your Account"
		body := fmt.Sprintf(
			"You are invited to join our platform!\r\n\r\n"+
				"Click the link below to access your account:\r\n"+
				"https://app.ucode.run\r\n\r\n"+
				"Your login credentials:\r\n"+
				"Login: %s\r\n"+
				"Password: %s\r\n\r\n"+
				"Welcome aboard! 🚀",
			login, password,
		)

		msg := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", to, subject, body)
		auth := smtp.PlainAuth("", s.cfg.Email, s.cfg.EmailPassword, host)
		err := smtp.SendMail(host+hostPort, auth, s.cfg.Email, []string{to}, []byte(msg))
		if err != nil {
			return nil, err
		}
	}

	return s.strg.User().GetByPK(ctx, &pb.UserPrimaryKey{Id: userId})
}

func (s *userService) V2GetUserByID(ctx context.Context, req *pb.UserPrimaryKey) (*pb.User, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_userv2.V2GetUserByID", req)
	defer dbSpan.Finish()

	s.log.Info("---V2GetUserByID--->", logger.Any("req", req))

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2GetUserByID", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2GetUserByID", memoryUsed))
		}
	}()

	var (
		result   *pbObject.CommonMessage
		resultGo *nb.CommonMessage
		userData map[string]any
		ok       bool
	)

	user, err := s.strg.User().GetByPK(ctx, req)
	if err != nil {
		s.log.Error("!!!GetUserByID--->", logger.Error(err))
		return nil, status.Error(codes.NotFound, err.Error())
	}

	userStatus, err := s.strg.User().GetUserStatus(ctx, user.GetId(), req.GetProjectId())
	if err != nil {
		s.log.Error("!!!GetUserByID--->", logger.Error(err))
		return nil, status.Error(codes.NotFound, err.Error())
	}

	structData, err := helper.ConvertRequestToSturct(map[string]any{"user_id_auth": req.Id})
	if err != nil {
		s.log.Error("!!!GetUserByID--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	services, err := s.serviceNode.GetByNodeType(req.ProjectId, req.NodeType)
	if err != nil {
		return nil, err
	}

	var tableSlug = "user"
	switch req.ResourceType {
	case 1:
		clientType, err := services.GetObjectBuilderServiceByType(req.NodeType).GetSingle(ctx, &pbObject.CommonMessage{
			TableSlug: "client_type",
			Data:      &structpb.Struct{Fields: map[string]*structpb.Value{"id": structpb.NewStringValue(req.GetClientTypeId())}},
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!GetUserByID--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		response, ok := clientType.Data.AsMap()["response"].(map[string]any)
		if ok {
			clientTypeTableSlug, ok := response["table_slug"].(string)
			if ok && clientTypeTableSlug != "" {
				tableSlug = clientTypeTableSlug
			}
		}

		result, err = services.GetObjectBuilderServiceByType(req.NodeType).GetSingle(ctx, &pbObject.CommonMessage{
			TableSlug: tableSlug,
			Data:      structData,
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!GetUserByID.ObjectBuilderService.GetSingle--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
	case 3:
		clientType, err := services.GoItemService().GetSingle(ctx, &nb.CommonMessage{
			TableSlug: "client_type",
			Data:      &structpb.Struct{Fields: map[string]*structpb.Value{"id": structpb.NewStringValue(req.GetClientTypeId())}},
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!GetUserByID--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		response, ok := clientType.Data.AsMap()["response"].(map[string]any)
		if ok {
			clientTypeTableSlug, ok := response["table_slug"].(string)
			if ok && clientTypeTableSlug != "" {
				tableSlug = clientTypeTableSlug
			}
		}

		resultGo, err = services.GoItemService().GetSingle(ctx, &nb.CommonMessage{
			TableSlug: tableSlug,
			Data:      structData,
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!GetUserByID.PostgresObjectBuilderService.GetSingle--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	if result != nil {
		userData, ok = result.Data.AsMap()["response"].(map[string]any)
	} else {
		userData, ok = resultGo.Data.AsMap()["response"].(map[string]any)
	}

	if !ok {
		err := errors.New("userData is nil")
		s.log.Error("!!!GetUserByID.ObjectBuilderService.GetSingle--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	roleId, ok := userData["role_id"].(string)
	if !ok {
		err := errors.New("role_id is nil")
		s.log.Error("!!!GetUserByID.ObjectBuilderService.GetSingle--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	user.RoleId = roleId

	clientTypeId, ok := userData["client_type_id"].(string)
	if !ok {
		err := errors.New("client_type_id is nil")
		s.log.Error("!!!GetUserByID.ObjectBuilderService.GetSingle--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	user.ClientTypeId = clientTypeId

	active, ok := userData["active"].(float64)
	if !ok {
		active = 0
	}

	user.Active = int32(active)

	projectId, ok := userData["project_id"].(string)
	if !ok {
		projectId = ""
	}

	name, ok := userData["name"].(string)
	if ok {
		user.Name = name
	}

	user.ProjectId = projectId
	user.Status = userStatus

	return user, nil
}

func (s *userService) V2GetUserList(ctx context.Context, req *pb.GetUserListRequest) (*pb.GetUserListResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_userv2.V2GetUserList", req)
	defer dbSpan.Finish()

	s.log.Info("---V2GetUserList--->", logger.Any("req", req))

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2GetUserList", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2GetUserList", memoryUsed))
		}
	}()

	var (
		resp      = &pb.GetUserListResponse{}
		usersResp *pbObject.CommonMessage
	)

	userIds, err := s.strg.User().GetUserIds(ctx, req)
	if err != nil {
		s.log.Error("!!!V2GetUserList--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	users, err := s.strg.User().GetListByPKs(ctx,
		&pb.UserPrimaryKeyList{
			Ids:       *userIds,
			ProjectId: req.ProjectId,
		},
	)
	if err != nil {
		s.log.Error("!!!V2GetUserList--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	var usersMap = make(map[string]*pb.User, users.Count)

	for _, user := range users.Users {
		usersMap[user.Id] = user
	}

	structReq := map[string]any{
		"offset":         req.GetOffset(),
		"limit":          req.GetLimit(),
		"with_relations": false,
		"user_id_auth":   map[string]any{"$in": userIds},
	}

	if util.IsValidUUID(req.ClientTypeId) {
		structReq["client_type_id"] = req.ClientTypeId
	}

	if util.IsValidUUID(req.ClientPlatformId) {
		structReq["client_platform_id"] = req.ClientPlatformId
	}

	structData, err := helper.ConvertRequestToSturct(structReq)
	if err != nil {
		s.log.Error("!!!V2GetUserList--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	services, err := s.serviceNode.GetByNodeType(req.ProjectId, req.NodeType)
	if err != nil {
		return nil, err
	}

	var tableSlug = "user"
	switch req.ResourceType {
	case 1:
		clientType, err := services.GetObjectBuilderServiceByType(req.NodeType).GetSingle(ctx, &pbObject.CommonMessage{
			TableSlug: "client_type",
			Data:      &structpb.Struct{Fields: map[string]*structpb.Value{"id": structpb.NewStringValue(req.GetClientTypeId())}},
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!V2GetUserList--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		response, ok := clientType.Data.AsMap()["response"].(map[string]any)
		if ok {
			clientTypeTableSlug, ok := response["table_slug"].(string)
			if ok && clientTypeTableSlug != "" {
				tableSlug = clientTypeTableSlug
			}
		}

		usersResp, err = services.GetObjectBuilderServiceByType(req.NodeType).GetList(ctx, &pbObject.CommonMessage{
			TableSlug: tableSlug,
			Data:      structData,
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!V2GetUserList.ObjectBuilderService.GetList--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
	case 3:
		clientType, err := services.GoItemService().GetSingle(ctx, &nb.CommonMessage{
			TableSlug: "client_type",
			Data:      &structpb.Struct{Fields: map[string]*structpb.Value{"id": structpb.NewStringValue(req.GetClientTypeId())}},
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!V2GetUserList--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		response, ok := clientType.Data.AsMap()["response"].(map[string]any)
		if ok {
			clientTypeTableSlug, ok := response["table_slug"].(string)
			if ok && clientTypeTableSlug != "" {
				tableSlug = clientTypeTableSlug
			}
		}

		goUsersResp, err := services.GoObjectBuilderService().GetList2(ctx, &nb.CommonMessage{
			TableSlug: tableSlug,
			Data:      structData,
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!GetUserList.PostgresObjectBuilderService.GetList--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		if err := helper.MarshalToStruct(goUsersResp, &usersResp); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	userCount, ok := usersResp.Data.AsMap()["count"].(float64)
	if !ok {
		err := errors.New("usersData is nil")
		s.log.Error("!!!GetUserList.ObjectBuilderService.GetList--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	usersData, ok := usersResp.Data.AsMap()["response"].([]any)
	if !ok {
		return &pb.GetUserListResponse{}, nil
	}

	resp.Users = make([]*pb.User, 0, int(userCount))
	resp.Count = int32(userCount)

	for _, userData := range usersData {
		userItem, ok := userData.(map[string]any)
		if !ok {
			err := errors.New("userItem is nil")
			s.log.Error("!!!GetUserList.ObjectBuilderService.GetList--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		userId, ok := userItem["user_id_auth"].(string)
		if !ok {
			err := errors.New("userId is nil")
			s.log.Error("!!!GetUserList.ObjectBuilderService.GetList--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		roleId, ok := userItem["role_id"].(string)
		if !ok {
			err := errors.New("roleId is nil")
			s.log.Error("!!!GetUserList.ObjectBuilderService.GetList--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		clientTypeId, ok := userItem["client_type_id"].(string)
		if !ok {
			err := errors.New("clientTypeId is nil")
			s.log.Error("!!!GetUserList.ObjectBuilderService.GetList--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		projectId, ok := userItem["project_id"].(string)
		if !ok {
			projectId = ""
		}

		active, ok := userItem["active"].(float64)
		if !ok {
			active = 0
		}

		user, ok := usersMap[userId]
		if !ok {
			err := errors.New("user is nil")
			s.log.Error("!!!GetUserList.ObjectBuilderService.GetList--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		name, ok := userItem["name"].(string)
		if ok {
			user.Name = name
		}

		user.Active = int32(active)
		user.ProjectId = projectId
		user.RoleId = roleId
		user.ClientTypeId = clientTypeId
		resp.Users = append(resp.Users, user)
	}

	return resp, nil
}

func (s *userService) V2UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.User, error) {
	s.log.Info("---V2UpdateUser--->", logger.Any("req", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_userv2.V2UpdateUser", req)
	defer dbSpan.Finish()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2UpdateUser", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2UpdateUser", memoryUsed))
		}
	}()

	rowsAffected, err := s.strg.User().Update(ctx, req)
	if err != nil {
		s.log.Error("!!!V2UpdateUser--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if rowsAffected <= 0 {
		return nil, status.Error(codes.InvalidArgument, "no rows were affected")
	}

	userProject, err := s.strg.User().UpdateUserToProject(ctx,
		&pb.AddUserToProjectReq{
			UserId:       req.Id,
			CompanyId:    req.CompanyId,
			ProjectId:    req.ProjectId,
			ClientTypeId: req.ClientTypeId,
			RoleId:       req.RoleId,
			EnvId:        req.EnvironmentId,
			Status:       req.Status,
		},
	)
	if err != nil {
		s.log.Error("!!!V2UpdateUser Update user project--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if userProject.UserId == "" {
		s.log.Error("!!!V2UpdateUser user project not update", logger.Error(err))
	}

	services, err := s.serviceNode.GetByNodeType(req.ProjectId, req.NodeType)
	if err != nil {
		return nil, err
	}

	structData, err := helper.ConvertRequestToSturct(req)
	if err != nil {
		s.log.Error("!!!V2UpdateUser--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	structData.Fields["from_auth_service"] = structpb.NewBoolValue(true)
	var tableSlug = "user"
	switch req.GetResourceType() {
	case 1:
		clientType, err := services.GetObjectBuilderServiceByType(req.NodeType).GetSingle(ctx, &pbObject.CommonMessage{
			TableSlug: "client_type",
			Data:      &structpb.Struct{Fields: map[string]*structpb.Value{"id": structpb.NewStringValue(req.GetClientTypeId())}},
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!V2UpdateUser--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		response, ok := clientType.Data.AsMap()["response"].(map[string]any)
		if ok {
			clientTypeTableSlug, ok := response["table_slug"].(string)
			if ok && clientTypeTableSlug != "" {
				tableSlug = clientTypeTableSlug
			}
		}

		_, err = services.GetObjectBuilderServiceByType(req.NodeType).UpdateByUserIdAuth(ctx, &pbObject.CommonMessage{
			TableSlug: tableSlug,
			Data:      structData,
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!UpdateUser.ObjectBuilderService.Update--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
	case 3:
		clientType, err := services.GoItemService().GetSingle(ctx, &nb.CommonMessage{
			TableSlug: "client_type",
			Data:      &structpb.Struct{Fields: map[string]*structpb.Value{"id": structpb.NewStringValue(req.GetClientTypeId())}},
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!V2GetUserSingle--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		response, ok := clientType.Data.AsMap()["response"].(map[string]any)
		if ok {
			clientTypeTableSlug, ok := response["table_slug"].(string)
			if ok && clientTypeTableSlug != "" {
				tableSlug = clientTypeTableSlug
			}
		}

		_, err = services.GoItemService().UpdateByUserIdAuth(ctx, &nb.CommonMessage{
			TableSlug: tableSlug,
			Data:      structData,
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!V2UpdateUser.PostgresObjectBuilderService.Update--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	res, err := s.strg.User().GetByPK(ctx, &pb.UserPrimaryKey{Id: req.Id})
	if err != nil {
		s.log.Error("!!!V2UpdateUser.GetByPK--->", logger.Error(err))
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return res, nil
}

func (s *userService) V2DeleteUser(ctx context.Context, req *pb.UserPrimaryKey) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_userv2.V2DeleteUser", req)
	defer dbSpan.Finish()

	s.log.Info("---V2DeleteUser--->", logger.Any("req", req))

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2DeleteUser", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2DeleteUser", memoryUsed))
		}
	}()

	var (
		res                    = &emptypb.Empty{}
		responseFromDeleteUser = &pbObject.CommonMessage{}
	)

	services, err := s.serviceNode.GetByNodeType(req.ProjectId, req.NodeType)
	if err != nil {
		return nil, err
	}

	var tableSlug = "user"
	switch req.GetResourceType() {
	case 1:
		clientType, err := services.GetObjectBuilderServiceByType(req.NodeType).GetSingle(ctx, &pbObject.CommonMessage{
			TableSlug: "client_type",
			Data: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"id": structpb.NewStringValue(req.GetClientTypeId()),
				},
			},
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!V2DeleteUser--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		response, ok := clientType.Data.AsMap()["response"].(map[string]any)
		if ok {
			clientTypeTableSlug := cast.ToString(response["table_slug"])
			if clientTypeTableSlug != "" {
				tableSlug = clientTypeTableSlug
			}
		}
		responseFromDeleteUser, err = services.GetObjectBuilderServiceByType(req.NodeType).Delete(ctx, &pbObject.CommonMessage{
			TableSlug: tableSlug,
			Data: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"id":                structpb.NewStringValue(req.Id),
					"from_auth_service": structpb.NewBoolValue(true),
				},
			},
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!V2DeleteUser.ObjectBuilderService.Update--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
		_, err = s.strg.User().DeleteUserFromProject(context.Background(), &pb.DeleteSyncUserRequest{
			UserId:       req.GetId(),
			ProjectId:    req.GetProjectId(),
			CompanyId:    req.GetCompanyId(),
			ClientTypeId: req.GetClientTypeId(),
			RoleId:       cast.ToString(responseFromDeleteUser.GetData().AsMap()["role_id"]),
		})
		if err != nil {
			s.log.Error("!!!V2DeleteUser--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
	case 3:
		clientType, err := services.GoItemService().GetSingle(ctx, &nb.CommonMessage{
			TableSlug: "client_type",
			Data: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"id": structpb.NewStringValue(req.GetClientTypeId()),
				},
			},
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!V2DeleteUser--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		response, ok := clientType.Data.AsMap()["response"].(map[string]any)
		if ok {
			clientTypeTableSlug := cast.ToString(response["table_slug"])
			if clientTypeTableSlug != "" {
				tableSlug = clientTypeTableSlug
			}
		}
		responseFromDeleteUser, err := services.GoItemService().Delete(ctx, &nb.CommonMessage{
			TableSlug: tableSlug,
			Data: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"id":                structpb.NewStringValue(req.Id),
					"from_auth_service": structpb.NewBoolValue(true),
				},
			},
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!V2DeleteUser--->", logger.Error(err))
			return nil, err
		}
		_, err = s.strg.User().DeleteUserFromProject(context.Background(), &pb.DeleteSyncUserRequest{
			UserId:       req.GetId(),
			ProjectId:    req.GetProjectId(),
			CompanyId:    req.GetCompanyId(),
			ClientTypeId: req.GetClientTypeId(),
			RoleId:       responseFromDeleteUser.Data.AsMap()["role_id"].(string),
		})
		if err != nil {
			s.log.Error("!!!V2DeleteUser--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	_, _ = s.strg.User().Delete(ctx, req)

	return res, nil
}

func (s *userService) AddUserToProject(ctx context.Context, req *pb.AddUserToProjectReq) (*pb.AddUserToProjectRes, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_userv2.AddUserToProject", req)
	defer dbSpan.Finish()

	s.log.Info("AddUserToProject", logger.Any("req", req))

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the AddUserToProject", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("AddUserToProject", memoryUsed))
		}
	}()

	proj, err := s.services.ProjectServiceClient().GetById(ctx, &pbc.GetProjectByIdRequest{ProjectId: req.GetProjectId()})
	if err != nil {
		s.log.Error("!!!AddUserToProject--->GetProjectById", logger.Error(err))
		return nil, status.Error(codes.Internal, "error getting project info")
	}

	// A project imported from a priced template charges the head balance per user
	// added instead of enforcing the fare user limit.
	if proj.GetPerUserPrice() > 0 {
		exists, existErr := s.strg.User().GetUserProjectByAllFields(ctx, models.GetUserProjectByAllFieldsReq{
			UserId:       req.GetUserId(),
			RoleId:       req.GetRoleId(),
			ProjectId:    req.GetProjectId(),
			ClientTypeId: req.GetClientTypeId(),
			CompanyId:    proj.GetCompanyId(),
			EnvId:        req.GetEnvId(),
		})
		if existErr != nil {
			s.log.Error("!!!AddUserToProject--->GetUserProjectByAllFields", logger.Error(existErr))
			return nil, status.Error(codes.Internal, existErr.Error())
		}
		if exists {
			return nil, status.Error(codes.AlreadyExists, config.ErrUserAlradyMember.Error())
		}

		seatCharge, seatErr := s.reserveUserSeat(ctx, proj, "")
		if seatErr != nil {
			return nil, seatErr
		}

		res, err := s.strg.User().AddUserToProject(ctx, req)
		if err != nil {
			s.releaseUserSeat(seatCharge)
			s.log.Error("cant add project to user", logger.Error(err))
			return nil, status.Error(codes.Internal, config.ErrUserAlradyMember.Error())
		}

		return res, nil
	}

	if limitErr := checkUserProjectLimit(ctx, s.services, s.strg, proj.GetFareId(), proj.GetCompanyId()); limitErr != nil {
		s.log.Error("!!!AddUserToProject--->checkUserProjectLimit", logger.Error(limitErr))
		return nil, limitErr
	}

	res, err := s.strg.User().AddUserToProject(ctx, req)
	if err != nil {
		errUserAdd := config.ErrUserAlradyMember
		s.log.Error("cant add project to user", logger.Error(err))
		return nil, status.Error(codes.Internal, errUserAdd.Error())
	}

	return res, nil
}

func (s *userService) GetProjectsByUserId(ctx context.Context, req *pb.GetProjectsByUserIdReq) (*pb.GetProjectsByUserIdRes, error) {
	s.log.Info("GetProjectsByUserId", logger.Any("req", req))

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the GetProjectsByUserId", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("GetProjectsByUserId", memoryUsed))
		}
	}()

	res, err := s.strg.User().GetProjectsByUserId(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *userService) V2GetUserByLoginTypes(ctx context.Context, req *pb.GetUserByLoginTypesRequest) (*pb.GetUserByLoginTypesResponse, error) {
	s.log.Info("GetProjectsByUserId", logger.Any("req", req))

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2GetUserByLoginTypes", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2GetUserByLoginTypes", memoryUsed))
		}
	}()

	res, err := s.strg.User().GetUserByLoginType(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *userService) GetUserByUsername(ctx context.Context, req *pb.GetUserByUsernameRequest) (*pb.User, error) {
	s.log.Info("GetUserByUsername -> ", logger.Any("req: ", req))
	res, err := s.strg.User().GetByUsername(ctx, req.GetUsername())
	if err != nil {
		return nil, err
	}
	s.log.Info("GetUserByUsername <- ", logger.Any("res: ", res))
	return res, nil
}
func (s *userService) GetUserProjects(ctx context.Context, req *pb.UserPrimaryKey) (*pb.GetUserProjectsRes, error) {
	userProjects, err := s.strg.User().GetUserProjects(ctx, req.Id)
	if err != nil {
		errGetProjects := errors.New("cant get user projects")
		s.log.Error("!!!MultiCompanyLogin--->", logger.Error(err))
		return nil, status.Error(codes.NotFound, errGetProjects.Error())
	}

	return userProjects, nil
}

func (s *userService) V2ResetPassword(ctx context.Context, req *pb.V2UserResetPasswordRequest) (*pb.User, error) {
	s.log.Info("GetProjectsByUserId", logger.Any("req", req))

	var (
		user             = &pb.User{}
		unHashedPassword = req.GetPassword()
		userIdAuth       string
	)

	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "grpc_userv2.V2ResetPassword")
	defer dbSpan.Finish()

	if len(req.GetOldPassword()) < 6 {
		err := config.ErrPasswordLength
		s.log.Error("!!!V2UserResetPassword--->", logger.Error(err))
		return nil, err
	}

	services, err := s.serviceNode.GetByNodeType(req.ProjectId, req.NodeType)
	if err != nil {
		s.log.Error("!!!V2UserResetPassword--->GetByNodeType", logger.Error(err))
		return nil, err
	}

	if req.GetClientTypeId() != "" {
		resource, err := s.services.ServiceResource().GetSingle(ctx, &pbc.GetSingleServiceResourceReq{
			ProjectId:     req.GetProjectId(),
			EnvironmentId: req.GetEnvironmentId(),
			ServiceType:   pbc.ServiceType_BUILDER_SERVICE,
		})
		if err != nil {
			err = errors.New("password updated in auth but not found resource in this project")
			s.log.Error("!!!V2UserResetPassword--->", logger.Error(err))
			return nil, err
		}
		switch req.ResourceType {
		case 1:
			updateUserResp, err := services.GetLoginServiceByType(resource.NodeType).UpdateUserPassword(ctx, &pbObject.UpdateUserPasswordRequest{
				Guid:                  req.UserId,
				ResourceEnvironmentId: resource.ResourceEnvironmentId,
				Password:              unHashedPassword,
				ClientTypeId:          req.ClientTypeId,
			})
			if err != nil {
				err = config.ErrFailedUpdate
				s.log.Error("!!!V2UserResetPassword.GetLoginServiceByUpdateUserPassword--->", logger.Error(err))
				return nil, err
			}

			userIdAuth = updateUserResp.GetUserIdAuth()
		case 3:
			updateUserResp, err := services.GoLoginService().UpdateUserPassword(ctx, &nb.UpdateUserPasswordRequest{
				Guid:                  req.UserId,
				ResourceEnvironmentId: resource.ResourceEnvironmentId,
				Password:              unHashedPassword,
				ClientTypeId:          req.ClientTypeId,
			})
			if err != nil {
				err = config.ErrFailedUpdate
				s.log.Error("!!!V2UserResetPassword.GoLoginService.UpdateUserPassword--->", logger.Error(err))
				return nil, err
			}
			userIdAuth = updateUserResp.GetUserIdAuth()
		}

		user, err = s.strg.User().GetByPK(ctx, &pb.UserPrimaryKey{
			Id: userIdAuth,
		})
		if err != nil {
			s.log.Error("!!!V2UserResetPassword-->UserGetByPK", logger.Error(err))
			return nil, err
		}

		hashType := user.GetHashType()
		switch config.HashTypes[hashType] {
		case 1:
			match, err := security.ComparePassword(user.GetPassword(), req.OldPassword)
			if err != nil {
				s.log.Error("!!!V2UserResetPassword-->ComparePasswordArgon", logger.Error(err))
				return nil, err
			}
			if !match {
				err := errors.New("wrong old password")
				s.log.Error("!!!V2UserResetPassword--->", logger.Error(err))
				return nil, err
			}
		case 2:
			match, err := security.ComparePasswordBcrypt(user.GetPassword(), req.OldPassword)
			if err != nil {
				s.log.Error("!!!V2UserResetPassword-->ComparePasswordBcrypt", logger.Error(err))
				return nil, err
			}
			if !match {
				err := errors.New("wrong old password")
				s.log.Error("!!!V2UserResetPassword--->", logger.Error(err))
				return nil, err
			}
		default:
			err := errors.New("hash type not found")
			s.log.Error("!!!V2ResetPassword--->", logger.Error(err))
			return nil, err
		}

		hashedPassword, err := security.HashPasswordBcrypt(req.Password)
		if err != nil {
			s.log.Error("!!!V2UserResetPassword--->HashPasswordBcrypt", logger.Error(err))
			return nil, err
		}

		req.Password = hashedPassword
		rowsAffected, err := s.strg.User().V2ResetPassword(ctx, &pb.V2ResetPasswordRequest{
			UserId:   userIdAuth,
			Password: req.Password,
		})
		if err != nil {
			s.log.Error("!!!V2UserResetPassword--->V2ResetPassword", logger.Error(err))
			return nil, err
		}
		if rowsAffected <= 0 {
			return nil, status.Error(codes.InvalidArgument, "no rows were affected")
		}
		user.Password = hashedPassword
	}

	return user, nil
}

func (s *userService) GetUserProjectClientTypes(ctx context.Context, req *pb.UserInfoPrimaryKey) (*pb.GetUserProjectClientTypesResponse, error) {
	s.log.Info("GetUserProjectClientTypes", logger.Any("req", req))

	clientTypes, err := s.strg.User().GetUserProjectClientTypes(ctx, req)
	if err != nil {
		s.log.Error("!!!GetUserProjectClientTypes--->", logger.Error(err))
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return clientTypes, nil
}
