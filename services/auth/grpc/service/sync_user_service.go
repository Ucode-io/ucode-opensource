package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"runtime"

	"github.com/Ucode-io/ucode-opensource/services/auth/api/models"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	pbc "github.com/Ucode-io/ucode-opensource/services/auth/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/grpc/client"
	span "github.com/Ucode-io/ucode-opensource/services/auth/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/security"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/saidamir98/udevs_pkg/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type syncUserService struct {
	cfg         config.BaseConfig
	log         logger.LoggerI
	strg        storage.StorageI
	services    client.ServiceManagerI
	serviceNode ServiceNodesI
	pb.UnimplementedSyncUserServiceServer
}

func NewSyncUserService(cfg config.BaseConfig, log logger.LoggerI, strg storage.StorageI, svcs client.ServiceManagerI, projectServiceNodes ServiceNodesI) *syncUserService {
	return &syncUserService{
		cfg:         cfg,
		log:         log,
		strg:        strg,
		services:    svcs,
		serviceNode: projectServiceNodes,
	}
}

func (sus *syncUserService) CreateUser(ctx context.Context, req *pb.CreateSyncUserRequest) (*pb.SyncUserResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_sync_user.CreateUser", req)
	defer dbSpan.Finish()

	sus.log.Info("---CreateSyncUser--->", logger.Any("req", req))

	var (
		response = pb.SyncUserResponse{}
		before   runtime.MemStats
		userId   string
		err      error
		username string
	)

	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		sus.log.Info("Memory used by the SyncUserCreateUser", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			sus.log.Info("Memory used over 300 mb", logger.Any("SyncUserCreateUser", memoryUsed))
		}
	}()

	for _, loginStrategy := range req.GetLoginStrategy() {
		//skip := false

		switch loginStrategy {
		case "login":
			username = req.GetLogin()
			//skip = true
		case "email":
			username = req.GetEmail()
		case "phone":
			username = req.GetPhone()
		case "e-imzo":
			username = req.GetTin()
		}
		if username != "" {
			user, err := sus.strg.User().GetByUsername(ctx, username)
			if err != nil {
				sus.log.Error("!!!CreateUser-->UserGetByUsername", logger.Error(err))
				return nil, err
			}

			//if skip {
			//	if len(user.GetId()) > 0 {
			//		sus.log.Error("!!!CreateSyncUser-->LoginCheck", logger.Error(err))
			//		return nil, status.Error(codes.InvalidArgument, "user with this login already exists")
			//	}
			//}

			if len(userId) == 0 {
				userId = user.GetId()
			}
		}
	}

	project, err := sus.services.ProjectServiceClient().GetById(ctx, &pbc.GetProjectByIdRequest{ProjectId: req.GetProjectId()})
	if err != nil {
		sus.log.Error("!!!CreateUser-->ProjectGetById", logger.Error(err))
		return nil, err
	}

	if limitErr := checkUserProjectLimit(ctx, sus.services, sus.strg, project.GetFareId(), project.GetCompanyId()); limitErr != nil {
		sus.log.Error("!!!CreateUser--->checkUserProjectLimit", logger.Error(limitErr))
		return nil, limitErr
	}

	if userId == "" {
		if req.GetPassword() != "" {
			hashedPassword, err := security.HashPasswordBcrypt(req.GetPassword())
			if err != nil {
				sus.log.Error("!!!CreateUser-->HashPassword", logger.Error(err))
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			req.Password = hashedPassword
		}

		user, err := sus.strg.User().Create(ctx, &pb.CreateUserRequest{
			Login:     req.GetLogin(),
			Password:  req.GetPassword(),
			Email:     req.GetEmail(),
			Phone:     req.GetPhone(),
			CompanyId: project.GetCompanyId(),
			Tin:       req.GetTin(),
		})
		if err != nil {
			sus.log.Error("!!!CreateUser--->UserCreate", logger.Error(err))
			return nil, err
		}
		userId = user.GetId()

		_, err = sus.strg.User().AddUserToProject(ctx, &pb.AddUserToProjectReq{
			UserId:       userId,
			CompanyId:    project.GetCompanyId(),
			RoleId:       req.GetRoleId(),
			ProjectId:    req.GetProjectId(),
			ClientTypeId: req.GetClientTypeId(),
			EnvId:        req.GetEnvironmentId(),
		})
		if err != nil {
			sus.log.Error("!!!CreateUser--->AddUserToProject", logger.Error(err))
			return nil, err
		}
	} else {
		exists, err := sus.strg.User().GetUserProjectByAllFields(ctx, models.GetUserProjectByAllFieldsReq{
			ClientTypeId: req.GetClientTypeId(),
			RoleId:       req.GetRoleId(),
			UserId:       userId,
			CompanyId:    project.GetCompanyId(),
			ProjectId:    req.GetProjectId(),
			EnvId:        req.GetEnvironmentId(),
		})
		if err != nil {
			sus.log.Error("!!!CreateUser--->GetUserProjectByAllFields", logger.Error(err))
			return nil, err
		}
		if !exists {
			_, err = sus.strg.User().AddUserToProject(ctx, &pb.AddUserToProjectReq{
				UserId:       userId,
				CompanyId:    project.GetCompanyId(),
				RoleId:       req.GetRoleId(),
				ProjectId:    req.GetProjectId(),
				ClientTypeId: req.GetClientTypeId(),
				EnvId:        req.GetEnvironmentId(),
			})
			if err != nil {
				sus.log.Error("!!!CreateUser--->AddUserToProjectExists", logger.Error(err))
				return nil, err
			}
		}
	}

	response.UserId = userId
	return &response, nil
}

func (sus *syncUserService) DeleteUser(ctx context.Context, req *pb.DeleteSyncUserRequest) (*empty.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_sync_user.DeleteUser", req)
	defer dbSpan.Finish()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		sus.log.Info("Memory used by the SyncUserDeleteUser", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			sus.log.Info("Memory used over 300 mb", logger.Any("SyncUserDeleteUser", memoryUsed))
		}
	}()

	_, err := sus.strg.User().DeleteUserFromProject(ctx, &pb.DeleteSyncUserRequest{
		UserId:        req.GetUserId(),
		RoleId:        req.GetRoleId(),
		ProjectId:     req.GetProjectId(),
		ClientTypeId:  req.GetClientTypeId(),
		EnvironmentId: req.GetEnvironmentId(),
	})
	if err != nil {
		sus.log.Info("!!!DeleteSyncUser-->DeleteUserFromProject", logger.Error(err))
		return nil, err
	}

	_, _ = sus.strg.User().Delete(ctx, &pb.UserPrimaryKey{Id: req.GetUserId()})
	return &empty.Empty{}, nil
}

func (sus *syncUserService) UpdateUser(ctx context.Context, req *pb.UpdateSyncUserRequest) (*pb.SyncUserResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_sync_user.UpdateUser", req)
	defer dbSpan.Finish()

	sus.log.Info("---UpdateSyncUser--->", logger.Any("req", req))

	var (
		before           runtime.MemStats
		hashedPassword   string
		err              error
		syncUser         = &pb.SyncUserResponse{}
		hasLoginStrategy bool
	)

	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		sus.log.Info("Memory used by the SyncUserUpdateUser", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			sus.log.Info("Memory used over 300 mb", logger.Any("SyncUserUpdateUser", memoryUsed))
		}
	}()

	project, err := sus.services.ProjectServiceClient().GetById(ctx,
		&pbc.GetProjectByIdRequest{ProjectId: req.GetProjectId()})
	if err != nil {
		sus.log.Error("!!!UpdateSyncUser-->ProjectGetById", logger.Error(err))
		return nil, err
	}

	req.CompanyId = project.GetCompanyId()

	if len(req.Password) > 0 {
		if len(req.Password) < 6 {
			err = fmt.Errorf("password must not be less than 6 characters")
			sus.log.Error("!!!UpdateSyncUser--->CheckPassword", logger.Error(err))
			return nil, err
		}

		hashedPassword, err = security.HashPasswordBcrypt(req.Password)
		if err != nil {
			sus.log.Error("!!!UpdateSyncUser--->HashPassword", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		req.Password = hashedPassword
	}

	if req.GetIsChangedLogin() {
		if len(req.Login) < 6 {
			err = fmt.Errorf("login must not be less than 6 characters")
			sus.log.Error("!!!UpdateSyncUser--->CheckLogin", logger.Error(err))
			return nil, err
		}

		syncUser, err = sus.strg.User().UpdateSyncUser(ctx, req, "login")
		if err != nil {
			sus.log.Error("!!!UpdateSyncUser--->UpdateSyncUserLogin", logger.Error(err))
			return nil, err
		}

		hasLoginStrategy = true
	}

	if req.GetIsChangedEmail() {
		if !IsValidEmailNew(req.Email) {
			err = config.ErrInvalidEmail
			sus.log.Error("!!!UpdateSyncUser--->CheckValidEmail", logger.Error(err))
			return nil, err
		}

		syncUser, err = sus.strg.User().UpdateSyncUser(ctx, req, "email")
		if err != nil {
			sus.log.Error("!!!UpdateSyncUser--->UpdateSyncUserEmail", logger.Error(err))
			return nil, err
		}

		hasLoginStrategy = true
	}

	if req.GetIsChangedPhone() {
		syncUser, err = sus.strg.User().UpdateSyncUser(ctx, req, "phone")
		if err != nil {
			sus.log.Error("!!!UpdateSyncUser--->UpdateSyncUserPhone", logger.Error(err))
			return nil, err
		}

		hasLoginStrategy = true
	}

	if req.GetIsChangedTin() {
		syncUser, err = sus.strg.User().UpdateSyncUser(ctx, req, "tin")
		if err != nil {
			sus.log.Error("!!!UpdateSyncUser--->UpdateSyncUserTin", logger.Error(err))
			return nil, err
		}

		hasLoginStrategy = true
	}

	if !hasLoginStrategy {
		_, err = sus.strg.User().ResetPassword(ctx, &pb.ResetPasswordRequest{
			UserId:   req.GetGuid(),
			Password: hashedPassword,
			Login:    req.GetLogin(),
			Email:    req.GetEmail(),
			Phone:    req.GetPhone(),
			Tin:      req.GetTin(),
		}, nil)
		if err != nil {
			sus.log.Error("!!!UpdateSyncUser--->UpdateSyncUserPassword", logger.Error(err))
			return nil, err
		}

		syncUser.UserId = req.GetGuid()
	}

	return syncUser, nil
}

// MergeContact links two auth accounts that belong to the same person: it folds
// the caller's current account into the account that already owns a just-added
// contact (phone/email), which survives. The contact must have been ownership-
// verified by the caller (e.g. an OTP-verified add-contact flow) — this RPC
// trusts current_user_id / found_user_id as given.
//
// company_id is resolved from the project (same as UpdateUser) so the membership
// move targets the exact user_project row regardless of what the caller sent.
// ErrMergeAccountInUse (current account still used elsewhere) surfaces as
// FailedPrecondition so the caller can fall back to a plain contact update.
func (sus *syncUserService) MergeContact(ctx context.Context, req *pb.MergeContactRequest) (*pb.SyncUserResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_sync_user.MergeContact", req)
	defer dbSpan.Finish()

	sus.log.Info("---MergeContact--->", logger.Any("req", req))

	if req.GetCurrentUserId() == "" || req.GetFoundUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "current_user_id and found_user_id are required")
	}

	project, err := sus.services.ProjectServiceClient().GetById(ctx,
		&pbc.GetProjectByIdRequest{ProjectId: req.GetProjectId()})
	if err != nil {
		sus.log.Error("!!!MergeContact-->ProjectGetById", logger.Error(err))
		return nil, err
	}
	req.CompanyId = project.GetCompanyId()

	resp, err := sus.strg.User().MergeContact(ctx, req)
	if err != nil {
		if errors.Is(err, storage.ErrMergeAccountInUse) {
			sus.log.Error("!!!MergeContact-->AccountInUse", logger.Error(err))
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		sus.log.Error("!!!MergeContact-->Merge", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (sus *syncUserService) DeleteManyUser(ctx context.Context, req *pb.DeleteManyUserRequest) (*empty.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_sync_user.DeleteManyUser", req)
	defer dbSpan.Finish()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		sus.log.Info("Memory used by the SyncDeleteManyUser", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			sus.log.Info("Memory used over 300 mb", logger.Any("SyncDeleteManyUser", memoryUsed))
		}
	}()

	project, err := sus.services.ProjectServiceClient().GetById(ctx, &pbc.GetProjectByIdRequest{
		ProjectId: req.GetProjectId(),
	})
	if err != nil {
		sus.log.Error("!!!DeleteManyUser--->GetProjectById", logger.Error(err))
		return nil, err
	}
	req.CompanyId = project.CompanyId

	_, err = sus.strg.User().DeleteUsersFromProject(ctx, req)
	if err != nil {
		sus.log.Error("!!!DeleteManyUser--->DeleteUsersFromProject", logger.Error(err))
		return nil, err
	}

	return &empty.Empty{}, nil
}

func (sus *syncUserService) CreateUsers(ctx context.Context, in *pb.CreateSyncUsersRequest) (*pb.SyncUsersResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_sync_user.CreateUsers", in)
	defer dbSpan.Finish()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		sus.log.Info("Memory used by the SyncCreateUsers", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			sus.log.Info("Memory used over 300 mb", logger.Any("SyncCreateUsers", memoryUsed))
		}
	}()

	var (
		response = pb.SyncUsersResponse{}
		user_ids = make([]string, 0, len(in.Users))
		err      error
	)
	for _, req := range in.Users {
		var (
			user     *pb.User
			username string
		)
		for _, loginStrategy := range req.GetLoginStrategy() {
			switch loginStrategy {
			case "login":
				username = req.GetLogin()
			case "email":
				username = req.GetEmail()
			case "phone":
				username = req.GetPhone()
			}
			if username != "" {
				user, err = sus.strg.User().GetByUsername(ctx, username)
				if err != nil {
					sus.log.Error("!!!CreateSyncUsers--->GetUserByUsername", logger.Error(err))
					return nil, err
				}
			}
		}

		userId := user.GetId()
		project, err := sus.services.ProjectServiceClient().GetById(
			ctx, &pbc.GetProjectByIdRequest{
				ProjectId: req.GetProjectId(),
			},
		)
		if err != nil {
			sus.log.Error("!!!CreateSyncUsers--->GetProjectById", logger.Error(err))
			return nil, err
		}

		if limitErr := checkUserProjectLimit(ctx, sus.services, sus.strg, project.GetFareId(), project.GetCompanyId()); limitErr != nil {
			sus.log.Error("!!!CreateSyncUsers--->checkUserProjectLimit", logger.Error(limitErr))
			return nil, limitErr
		}

		if userId == "" {
			if req.GetPassword() != "" {
				hashedPassword, err := security.HashPasswordBcrypt(req.GetPassword())
				if err != nil {
					sus.log.Error("!!!CreateSyncUsers--->HashPasswordBcrypt", logger.Error(err))
					return nil, status.Error(codes.InvalidArgument, err.Error())
				}
				req.Password = hashedPassword
			}

			user, err := sus.strg.User().Create(ctx, &pb.CreateUserRequest{
				Login:     req.GetLogin(),
				Password:  req.GetPassword(),
				Email:     req.GetEmail(),
				Phone:     req.GetPhone(),
				CompanyId: project.GetCompanyId(),
			})
			if err != nil {
				sus.log.Error("!!!CreateSyncUsers--->CreateUser", logger.Error(err))
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			userId = user.GetId()
			_, err = sus.strg.User().AddUserToProject(ctx, &pb.AddUserToProjectReq{
				UserId:       userId,
				CompanyId:    project.GetCompanyId(),
				RoleId:       req.GetRoleId(),
				ProjectId:    req.GetProjectId(),
				ClientTypeId: req.GetClientTypeId(),
				EnvId:        req.GetEnvironmentId(),
			})
			if err != nil {
				sus.log.Error("!!!CreateSyncUsers-->AddUserToProject", logger.Error(err))
				return nil, err
			}
		} else {
			exists, err := sus.strg.User().GetUserProjectByAllFields(ctx, models.GetUserProjectByAllFieldsReq{
				ClientTypeId: req.GetClientTypeId(),
				RoleId:       req.GetRoleId(),
				UserId:       userId,
				CompanyId:    project.GetCompanyId(),
				ProjectId:    req.GetProjectId(),
				EnvId:        req.GetEnvironmentId(),
			})
			if err != nil {
				sus.log.Error("!!!CreateSyncUsers-->GetUserProjectByAllFields", logger.Error(err))
				return nil, err
			}
			if !exists {
				_, err = sus.strg.User().AddUserToProject(ctx, &pb.AddUserToProjectReq{
					UserId:       userId,
					CompanyId:    project.GetCompanyId(),
					RoleId:       req.GetRoleId(),
					ProjectId:    req.GetProjectId(),
					ClientTypeId: req.GetClientTypeId(),
					EnvId:        req.GetEnvironmentId(),
				})
				if err != nil {
					sus.log.Error("!!!CreateSyncUsers-->AddUserToProjectExists", logger.Error(err))
					return nil, err
				}
			}
		}

		user_ids = append(user_ids, userId)
	}

	response.UserIds = user_ids

	return &response, nil
}

func IsValidEmailNew(email string) bool {
	emailRegex := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`

	re := regexp.MustCompile(emailRegex)

	return re.MatchString(email)
}
