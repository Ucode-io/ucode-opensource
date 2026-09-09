package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	pbc "github.com/Ucode-io/ucode-opensource/services/auth/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"
	span "github.com/Ucode-io/ucode-opensource/services/auth/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/security"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/util"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/saidamir98/udevs_pkg/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type apiKeysService struct {
	cfg         config.BaseConfig
	log         logger.LoggerI
	strg        storage.StorageI
	services    client.ServiceManagerI
	serviceNode ServiceNodesI
	pb.UnimplementedApiKeysServer
}

func NewApiKeysService(cfg config.BaseConfig, log logger.LoggerI, strg storage.StorageI, svcs client.ServiceManagerI, projectServiceNodes ServiceNodesI) *apiKeysService {
	return &apiKeysService{
		cfg:         cfg,
		log:         log,
		strg:        strg,
		services:    svcs,
		serviceNode: projectServiceNodes,
	}
}

func (s *apiKeysService) Create(ctx context.Context, req *pb.CreateReq) (*pb.CreateRes, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_apikey_v2.Create", req)
	defer dbSpan.Finish()

	s.log.Info("---CreateApiKey--->", logger.Any("req", req))

	project, err := s.services.ProjectServiceClient().GetById(
		ctx, &pbc.GetProjectByIdRequest{
			ProjectId: req.GetProjectId(),
		},
	)
	if err != nil {
		s.log.Error("!!!Create--->GetProjectById", logger.Error(err))
		return nil, status.Error(codes.Internal, "error getting project info")
	}

	if len(project.GetFareId()) != 0 {
		count, err := s.strg.ApiKeys().GetProjectApiKeysCount(ctx, req.GetProjectId())
		if err != nil {
			s.log.Error("!!!Create--->GetProjectApiKeysCount", logger.Error(err))
			return nil, status.Error(codes.Internal, "error getting api keys count")
		}

		limitResp, err := s.services.BillingServiceClient().CompareFunction(ctx, &pbc.CompareFunctionRequest{
			Type:   config.FARE_API_KEYS,
			FareId: project.GetFareId(),
			Count:  count + 1,
		})
		if err != nil {
			s.log.Error("!!!Create--->CompareFunction", logger.Error(err))
			return nil, status.Error(codes.Internal, "error checking api key limit")
		}

		if !limitResp.HasAccess {
			return nil, status.Error(codes.ResourceExhausted, "you have reached the limit of API keys on your current plan. Please upgrade to create more.")
		}
	}

	id, err := uuid.NewUUID()
	if err != nil {
		s.log.Error("!!!Create--->NewUUID", logger.Error(err))
		return nil, status.Error(codes.Internal, "internal")
	}

	secretKey := "S-" + helper.GenerateSecretKey(32)
	secretId := "P-" + helper.GenerateSecretKey(32)
	clientId := helper.GenerateSecretKey(32)

	hashedSecretKey, _ := security.HashPasswordBcrypt(secretKey)
	req.ClientId = clientId

	res, err := s.strg.ApiKeys().Create(ctx, req, hashedSecretKey, secretId, id.String())
	if err != nil {
		s.log.Error("!!!Create--->ApiKeys", logger.Error(err))
		return nil, status.Error(codes.Internal, "error on creating new api key")
	}

	res.AppSecret = secretKey

	return res, nil
}

func (s *apiKeysService) Update(ctx context.Context, req *pb.UpdateReq) (*pb.UpdateRes, error) {
	s.log.Info("---UpdateApiKey--->", logger.Any("req", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_apikey_v2.Update", req)
	defer dbSpan.Finish()

	res, err := s.strg.ApiKeys().Update(ctx, req)
	if err != nil {
		s.log.Error("!!!Update--->", logger.Error(err))
		return &pb.UpdateRes{RowEffected: int32(res)},
			status.Error(codes.Internal, "error on updating new api key")
	}

	return &pb.UpdateRes{
		RowEffected: int32(res),
	}, nil
}

func (s *apiKeysService) Get(ctx context.Context, req *pb.GetReq) (*pb.GetRes, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_apikey_v2.Get", req)
	defer dbSpan.Finish()

	s.log.Info("---GetApiKey--->", logger.Any("req", req))

	res, err := s.strg.ApiKeys().Get(ctx, req)
	if err == pgx.ErrNoRows {
		s.log.Error("!!!GetApiKey--->", logger.Error(err))
		return nil, status.Error(codes.NotFound, "api-key not found")
	}
	if err != nil {
		s.log.Error("!!!GetApiKey--->", logger.Error(err))
		return nil, status.Error(codes.Internal, "error on getting api key")
	}

	return res, nil
}

func (s *apiKeysService) GetList(ctx context.Context, req *pb.GetListReq) (*pb.GetListRes, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_apikey_v2.GetList", req)
	defer dbSpan.Finish()

	s.log.Info("---GetListApiKey--->", logger.Any("req", req))

	if !util.IsValidUUID(req.GetProjectId()) {
		s.log.Error("!!!GetListApiKey--->", logger.Error(errors.New("project id is invalid uuid")))
		return nil, status.Error(codes.Internal, "error on getting api keys, project id is invalid uuid")
	}

	res, err := s.strg.ApiKeys().GetList(ctx, req)
	if err != nil {
		s.log.Error("!!!GetListApiKey--->", logger.Error(err))
		return nil, status.Error(codes.Internal, "error on getting api keys")
	}

	return res, nil
}

func (s *apiKeysService) Delete(ctx context.Context, req *pb.DeleteReq) (*pb.DeleteRes, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_apikey_v2.Delete", req)
	defer dbSpan.Finish()

	s.log.Info("---DeleteApiKey--->", logger.Any("req", req))

	res, err := s.strg.ApiKeys().Delete(ctx, req)
	if err != nil {
		s.log.Error("!!!DeleteApiKey--->", logger.Error(err))
		return &pb.DeleteRes{
			RowEffected: int32(res),
		}, status.Error(codes.Internal, "error on deleting api keys")
	}

	return &pb.DeleteRes{
		RowEffected: int32(res),
	}, nil
}

func (s *apiKeysService) GenerateApiToken(ctx context.Context, req *pb.GenerateApiTokenReq) (*pb.GenerateApiTokenRes, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_apikey_v2.GenerateApiToken", req)
	defer dbSpan.Finish()

	s.log.Info("---GenerateApiToken--->", logger.Any("req", req))

	if len(req.GetAppId()) != 34 || req.GetAppId()[:2] != "P-" {
		errAppId := errors.New("invalid api id or api secret")
		s.log.Error("!!!GenerateApiToken--->", logger.Error(errAppId))
		return nil, status.Error(codes.InvalidArgument, errAppId.Error())
	}

	if len(req.GetAppSecret()) != 34 || req.GetAppSecret()[:2] != "S-" {
		errAppSecret := errors.New("invalid api id or api secret")
		s.log.Error("!!!GenerateApiToken--->", logger.Error(errAppSecret))
		return nil, status.Error(codes.InvalidArgument, errAppSecret.Error())
	}

	apiKey, err := s.strg.ApiKeys().GetByAppId(ctx, req.AppId)
	if err != nil {
		s.log.Error("!!!GenerateApiToken--->", logger.Error(err))
		if err == sql.ErrNoRows {
			errNoRows := errors.New("no api keys found")
			return nil, status.Error(codes.NotFound, errNoRows.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	match, err := security.ComparePasswordBcrypt(apiKey.GetAppSecret(), req.GetAppSecret())
	if err != nil {
		errComparePass := errors.New("invalid api id or api secret")
		s.log.Error("!!!GenerateApiToken--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, errComparePass.Error())
	}
	if !match {
		errComparePass := errors.New("invalid api id or api secret")
		s.log.Error("!!!GenerateApiToken--->", logger.String("match", "false"))
		return nil, status.Error(codes.InvalidArgument, errComparePass.Error())
	}

	m := map[string]any{
		"environment_id": apiKey.GetEnvironmentId(),
		"role_id":        apiKey.GetRoleId(),
		"app_id":         apiKey.GetAppId(),
		"client_type_id": apiKey.GetClientTypeId(),
	}

	apiKeyToken, err := security.GenerateJWT(m, config.AccessTokenExpiresInTime, s.cfg.SecretKey)
	if err != nil {
		errGenerateJWT := errors.New("error on generating token")
		s.log.Error("!!!GenerateApiToken--->GenerateAccessJWT", logger.Error(err))
		return nil, status.Error(codes.Unavailable, errGenerateJWT.Error())
	}

	refreshToken, err := security.GenerateJWT(m, config.RefreshTokenExpiresInTime, s.cfg.SecretKey)
	if err != nil {
		errGenerateJWT := errors.New("error on generating token")
		s.log.Error("!!!GenerateApiToken--->GenerateRefreshJWT", logger.Error(err))
		return nil, status.Error(codes.Internal, errGenerateJWT.Error())
	}

	return &pb.GenerateApiTokenRes{
		Token: &pb.Token{
			AccessToken:      apiKeyToken,
			RefreshToken:     refreshToken,
			CreatedAt:        time.Now().Format(config.DatabaseTimeLayout),
			UpdatedAt:        time.Now().Format(config.DatabaseTimeLayout),
			ExpiresAt:        time.Now().Add(config.RefreshTokenExpiresInTime).Format(config.DatabaseTimeLayout),
			RefreshInSeconds: int32(config.AccessTokenExpiresInTime.Seconds()),
		},
	}, nil
}

func (s *apiKeysService) RefreshApiToken(ctx context.Context, req *pb.RefreshApiTokenReq) (*pb.RefreshApiTokenRes, error) {
	dbSpan, _ := span.StartSpanFromContext(ctx, "grpc_apikey_v2.RefreshApiToken", req)
	defer dbSpan.Finish()

	m, err := security.ExtractClaims(req.GetRefreshToken(), s.cfg.SecretKey)
	if err != nil {
		errExtractToken := errors.New("error on extracting refresh token")
		s.log.Error("!!!GenerateApiToken--->", logger.Error(err))
		return nil, status.Error(codes.Internal, errExtractToken.Error())
	}
	_ = m["environment_id"].(string)
	_ = m["role_id"].(string)
	_ = m["app_id"].(string)
	_ = m["client_type_id"].(string)

	apiKeyToken, err := security.GenerateJWT(m, config.AccessTokenExpiresInTime, s.cfg.SecretKey)
	if err != nil {
		errGenerateJWT := errors.New("error on refreshing token")
		s.log.Error("!!!GenerateApiToken--->", logger.Error(err))
		return nil, status.Error(codes.Unavailable, errGenerateJWT.Error())
	}

	refreshToken, err := security.GenerateJWT(m, config.RefreshTokenExpiresInTime, s.cfg.SecretKey)
	if err != nil {
		errGenerateJWT := errors.New("error on refreshing token")
		s.log.Error("!!!GenerateApiToken--->", logger.Error(err))
		return nil, status.Error(codes.Internal, errGenerateJWT.Error())
	}

	return &pb.RefreshApiTokenRes{
		Token: &pb.Token{
			AccessToken:      apiKeyToken,
			RefreshToken:     refreshToken,
			CreatedAt:        time.Now().Format(config.DatabaseTimeLayout),
			UpdatedAt:        time.Now().Format(config.DatabaseTimeLayout),
			ExpiresAt:        time.Now().Add(config.RefreshTokenExpiresInTime).Format(config.DatabaseTimeLayout),
			RefreshInSeconds: int32(config.AccessTokenExpiresInTime.Seconds()),
		},
	}, nil
}

func (s *apiKeysService) GetEnvID(ctx context.Context, req *pb.GetReq) (resp *pb.GetRes, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_apikey_v2.GetEnvID", req)
	defer dbSpan.Finish()

	s.log.Info("---GetEnvID--->>>>", logger.Any("req", req))

	resp, err = s.strg.ApiKeys().GetEnvID(ctx, req)
	if err != nil {
		s.log.Error("---GetEnvID->Error--->>>", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return resp, nil
}

func (s *apiKeysService) ListClientToken(ctx context.Context, req *pb.ListClientTokenRequest) (resp *pb.ListClientTokenResponse, err error) {
	s.log.Info("---ListClientToken--->>>>", logger.Any("req", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_apikey_v2.ListClientToken", req)
	defer dbSpan.Finish()

	resp, err = s.strg.ApiKeys().ListClientToken(ctx, req)
	if err != nil {
		s.log.Error("---ListClientToken->Error--->>>", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return resp, nil
}
func (s *apiKeysService) GetProjectApiKeysCount(ctx context.Context, req *pb.GetProjectApiKeysCountRequest) (*pb.GetProjectApiKeysCountResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_apikey_v2.GetProjectApiKeysCount", req)
	defer dbSpan.Finish()

	s.log.Info("---GetProjectApiKeysCount--->>>>", logger.Any("req", req))

	count, err := s.strg.ApiKeys().GetProjectApiKeysCount(ctx, req.GetProjectId())
	if err != nil {
		s.log.Error("---GetProjectApiKeysCount->Error--->>>", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.GetProjectApiKeysCountResponse{
		Count: count,
	}, nil
}
