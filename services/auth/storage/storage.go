package storage

import (
	"context"
	"errors"

	"github.com/Ucode-io/ucode-opensource/services/auth/api/models"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/emptypb"
)

var ErrorTheSameId = errors.New("cannot use the same uuid for 'id' and 'parent_id' fields")

// ErrMergeAccountInUse is returned by MergeContact when the account being merged
// away is still referenced by projects other than the one performing the merge.
// Deleting it then would leave a dangling user_id_auth in those projects' builder
// DBs (which the auth service cannot reach), breaking their logins.
var ErrMergeAccountInUse = errors.New("current account is used in other projects")
var ErrorProjectId = errors.New("not valid 'project_id'")

type StorageI interface {
	CloseDB()
	ClientPlatform() ClientPlatformRepoI
	ClientType() ClientTypeRepoI
	Client() ClientRepoI
	User() UserRepoI
	Session() SessionRepoI
	Company() CompanyRepoI
	Project() ProjectRepoI
	ApiKeys() ApiKeysRepoI
	AppleSettings() AppleSettingsI
	ApiKeyUsage() ApiKeyUsageRepoI
}

type ClientPlatformRepoI interface {
	Create(ctx context.Context, entity *pb.CreateClientPlatformRequest) (pKey *pb.ClientPlatformPrimaryKey, err error)
	GetList(ctx context.Context, queryParam *pb.GetClientPlatformListRequest) (res *pb.GetClientPlatformListResponse, err error)
	GetByPK(ctx context.Context, pKey *pb.ClientPlatformPrimaryKey) (res *pb.ClientPlatform, err error)
}

type ClientTypeRepoI interface {
	Create(ctx context.Context, entity *pb.CreateClientTypeRequest) (pKey *pb.ClientTypePrimaryKey, err error)
	GetList(ctx context.Context, queryParam *pb.GetClientTypeListRequest) (res *pb.GetClientTypeListResponse, err error)
	GetByPK(ctx context.Context, pKey *pb.ClientTypePrimaryKey) (res *pb.ClientType, err error)
	Update(ctx context.Context, entity *pb.UpdateClientTypeRequest) (rowsAffected int64, err error)
	Delete(ctx context.Context, pKey *pb.ClientTypePrimaryKey) (rowsAffected int64, err error)
	GetCompleteByPK(ctx context.Context, pKey *pb.ClientTypePrimaryKey) (res *pb.CompleteClientType, err error)
}

type ClientRepoI interface {
	Add(ctx context.Context, projectID string, entity *pb.AddClientRequest) (err error)
	GetByPK(ctx context.Context, entity *pb.ClientPrimaryKey) (res *pb.Client, err error)
	Update(ctx context.Context, entity *pb.UpdateClientRequest) (rowsAffected int64, err error)
	Remove(ctx context.Context, entity *pb.ClientPrimaryKey) (rowsAffected int64, err error)
	GetList(ctx context.Context, queryParam *pb.GetClientListRequest) (res *pb.GetClientListResponse, err error)
	GetMatrix(ctx context.Context, req *pb.GetClientMatrixRequest) (res *pb.GetClientMatrixResponse, err error)
}

type UserRepoI interface {
	GetListByPKs(ctx context.Context, pKeys *pb.UserPrimaryKeyList) (res *pb.GetUserListResponse, err error)
	Create(ctx context.Context, entity *pb.CreateUserRequest) (pKey *pb.UserPrimaryKey, err error)
	GetList(ctx context.Context, queryParam *pb.GetUserListRequest) (res *pb.GetUserListResponse, err error)
	GetByPK(ctx context.Context, pKey *pb.UserPrimaryKey) (res *pb.User, err error)
	Update(ctx context.Context, entity *pb.UpdateUserRequest) (rowsAffected int64, err error)
	Delete(ctx context.Context, pKey *pb.UserPrimaryKey) (rowsAffected int64, err error)
	GetByUsername(ctx context.Context, username string) (res *pb.User, err error)
	GetByGoogleID(ctx context.Context, googleID string) (res *pb.User, err error)
	UpdateGoogleID(ctx context.Context, userID, googleID string) error
	ResetPassword(ctx context.Context, user *pb.ResetPasswordRequest, tx pgx.Tx) (rowsAffected int64, err error)
	GetUserProjects(ctx context.Context, userId string) (*pb.GetUserProjectsRes, error)
	GetUserProjectsEnv(ctx context.Context, userId, envId string) (*pb.GetUserProjectsRes, error)
	GetUserProjectByUserIdProjectIdEnvId(ctx context.Context, userId, projectId, envId string) (string, error)

	GetUserProjectClientTypes(ctx context.Context, req *pb.UserInfoPrimaryKey) (*pb.GetUserProjectClientTypesResponse, error)
	AddUserToProject(ctx context.Context, req *pb.AddUserToProjectReq) (*pb.AddUserToProjectRes, error)
	UpdateUserToProject(ctx context.Context, req *pb.AddUserToProjectReq) (*pb.AddUserToProjectRes, error)
	GetProjectsByUserId(ctx context.Context, req *pb.GetProjectsByUserIdReq) (*pb.GetProjectsByUserIdRes, error)
	GetUserIds(ctx context.Context, req *pb.GetUserListRequest) (*[]string, error)
	GetUserByLoginType(ctx context.Context, req *pb.GetUserByLoginTypesRequest) (*pb.GetUserByLoginTypesResponse, error)
	GetListTimezone(ctx context.Context, in *pb.GetListSettingReq) (*models.ListTimezone, error)
	GetListLanguage(ctx context.Context, in *pb.GetListSettingReq) (*models.ListLanguage, error)
	V2ResetPassword(ctx context.Context, req *pb.V2ResetPasswordRequest) (int64, error)
	GetUserProjectByAllFields(ctx context.Context, req models.GetUserProjectByAllFieldsReq) (bool, error)
	DeleteUserFromProject(ctx context.Context, req *pb.DeleteSyncUserRequest) (*emptypb.Empty, error)
	DeleteUsersFromProject(ctx context.Context, req *pb.DeleteManyUserRequest) (*emptypb.Empty, error)
	GetAllUserProjects(ctx context.Context) ([]string, error)
	UpdateUserProjects(ctx context.Context, envId, projectId string) (*emptypb.Empty, error)
	GetUserEnvProjects(ctx context.Context, userId string) (*models.GetUserEnvProjectRes, error)
	GetUserEnvProjectsV2(ctx context.Context, userId, envId string) (*models.GetUserEnvProjectRes, error)
	CHeckUserProject(ctx context.Context, id, projectId string) (res *pb.User, err error)
	UpdatePassword(ctx context.Context, userId, password string) error
	V2GetByUsername(ctx context.Context, username, strategy string) (res *pb.User, err error)
	UpdateSyncUser(ctx context.Context, req *pb.UpdateSyncUserRequest, loginType string) (*pb.SyncUserResponse, error)
	UpdateLoginStrategy(ctx context.Context, req *pb.UpdateSyncUserRequest, user *pb.ResetPasswordRequest, tx pgx.Tx) (string, error)
	MergeContact(ctx context.Context, req *pb.MergeContactRequest) (*pb.SyncUserResponse, error)
	GetUserStatus(ctx context.Context, userId, projectId string) (status string, err error)
	GetProjectUsersCount(ctx context.Context, projectId string) (int32, error)
	GetCompanyUsersCount(ctx context.Context, companyId string) (int32, error)
}

type SessionRepoI interface {
	Create(ctx context.Context, entity *pb.CreateSessionRequest) (pKey *pb.SessionPrimaryKey, err error)
	GetList(ctx context.Context, queryParam *pb.GetSessionListRequest) (res *pb.GetSessionListResponse, err error)
	GetByPK(ctx context.Context, pKey *pb.SessionPrimaryKey) (res *pb.Session, err error)
	Update(ctx context.Context, entity *pb.UpdateSessionRequest) (rowsAffected int64, err error)
	Delete(ctx context.Context, pKey *pb.SessionPrimaryKey) (rowsAffected int64, err error)
	DeleteExpiredUserSessions(ctx context.Context, userID string) (rowsAffected int64, err error)
	GetSessionListByUserID(ctx context.Context, userID string) (res *pb.GetSessionListResponse, err error)
	ExpireSessions(ctx context.Context, entity *pb.ExpireSessionsRequest) (err error)
	DeleteByParams(ctx context.Context, entity *pb.DeleteByParamsRequest) (deletedIDs []string, err error)
	GetSessionDevices(ctx context.Context, req *pb.GetSessionDevicesRequest) (*pb.GetSessionDevicesResponse, error)
	DeleteSessionsByDevice(ctx context.Context, req *pb.DeleteSessionsByDeviceRequest) error
	DeleteSessionsExceptCurrent(ctx context.Context, req *pb.DeleteSessionsExceptCurrentRequest) error
	UpdateLastActivity(ctx context.Context, sessionID string) error
}

type CompanyRepoI interface {
	Register(ctx context.Context, entity *pb.RegisterCompanyRequest) (pKey *pb.CompanyPrimaryKey, err error)
	Update(ctx context.Context, entity *pb.UpdateCompanyRequest) (rowsAffected int64, err error)
	Remove(ctx context.Context, pKey *pb.CompanyPrimaryKey) (rowsAffected int64, err error)
	GetList(ctx context.Context, queryParam *pb.GetComapnyListRequest) (*pb.GetListCompanyResponse, error)
	GetByID(ctx context.Context, pKey *pb.CompanyPrimaryKey) (*pb.Company, error)
	TransferOwnership(ctx context.Context, companyID, ownerID string) (rowsAffected int64, err error)
}

type ProjectRepoI interface {
	Create(ctx context.Context, entity *pb.CreateProjectRequest) (pKey *pb.ProjectPrimaryKey, err error)
	GetList(ctx context.Context, queryParam *pb.GetProjectListRequest) (res *pb.GetProjectListResponse, err error)
	GetByPK(ctx context.Context, pKey *pb.ProjectPrimaryKey) (res *pb.Project, err error)
	Update(ctx context.Context, entity *pb.UpdateProjectRequest) (rowsAffected int64, err error)
	Delete(ctx context.Context, pKey *pb.ProjectPrimaryKey) (rowsAffected int64, err error)
}

type ApiKeysRepoI interface {
	Create(ctx context.Context, req *pb.CreateReq, appSecret, appId, id string) (res *pb.CreateRes, err error)
	GetList(ctx context.Context, req *pb.GetListReq) (res *pb.GetListRes, err error)
	Get(ctx context.Context, req *pb.GetReq) (res *pb.GetRes, err error)
	Update(ctx context.Context, req *pb.UpdateReq) (rowsAffected int64, err error)
	Delete(ctx context.Context, req *pb.DeleteReq) (rowsAffected int64, err error)
	GetByAppId(ctx context.Context, appId string) (*pb.GetRes, error)
	GetEnvID(ctx context.Context, req *pb.GetReq) (*pb.GetRes, error)
	UpdateIsMonthlyLimitReached(ctx context.Context) error
	ListClientToken(ctx context.Context, req *pb.ListClientTokenRequest) (res *pb.ListClientTokenResponse, err error)
	CreateClientToken(ctx context.Context, clientId string, info map[string]any) error
	CheckClientIdStatus(ctx context.Context, clientId string) (bool, error)
	GetProjectApiKeysCount(ctx context.Context, projectId string) (int32, error)
}

type AppleSettingsI interface {
	Create(ctx context.Context, input *pb.AppleIdSettings) (*pb.AppleIdSettings, error)
	GetByPK(ctx context.Context, pKey *pb.AppleIdSettingsPrimaryKey) (res *pb.AppleIdSettings, err error)
	UpdateAppleSettings(ctx context.Context, input *pb.AppleIdSettings) (string, error)
	GetListAppleSettings(ctx context.Context, input *pb.GetListAppleIdSettingsRequest) (*pb.GetListAppleIdSettingsResponse, error)
	DeleteAppleSettings(ctx context.Context, input *pb.AppleIdSettingsPrimaryKey) (*emptypb.Empty, error)
}

type ApiKeyUsageRepoI interface {
	CheckLimit(ctx context.Context, req *pb.CheckLimitRequest) (res *pb.CheckLimitResponse, err error)
	Create(ctx context.Context, req *pb.ApiKeyUsage) error
	Upsert(ctx context.Context, req *pb.ApiKeyUsage) error
	UpdateMonthlyLimit(ctx context.Context) error
}
