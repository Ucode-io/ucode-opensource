package service

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/grpc/client"
	span "github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"
)

type loginService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	nb.UnimplementedLoginServiceServer
}

func NewLoginService(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) *loginService {
	return &loginService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

func (l *loginService) LoginData(ctx context.Context, req *nb.LoginDataReq) (resp *nb.LoginDataRes, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_login.LoginData", req)
	defer dbSpan.Finish()

	l.log.Info("---LoginData--->>>", logger.Any("request", compactRequest(req)))

	resp, err = l.strg.Login().LoginData(ctx, req)
	if err != nil {
		l.log.Error("---LoginData--->>>", logger.Error(err))
		return &nb.LoginDataRes{}, err
	}

	return resp, nil
}

func (l *loginService) GetConnetionOptions(ctx context.Context, req *nb.GetConnetionOptionsRequest) (resp *nb.GetConnectionOptionsResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_login.GetConnetionOptions", req)
	defer dbSpan.Finish()

	l.log.Info("---GetConnetionOptions--->>>", logger.Any("request", compactRequest(req)))

	resp, err = l.strg.Login().GetConnectionOptions(ctx, req)
	if err != nil {
		l.log.Error("---GetConnectionOptions--->>>", logger.Error(err))
		return
	}

	return
}

func (l *loginService) UpdateUserPassword(ctx context.Context, req *nb.UpdateUserPasswordRequest) (resp *nb.LoginDataRes, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_login.UpdateUserPassword", req)
	defer dbSpan.Finish()

	l.log.Info("---UpdateUserPassword--->>>", logger.Any("request", compactRequest(req)))

	resp, err = l.strg.Login().UpdateUserPassword(ctx, req)
	if err != nil {
		l.log.Error("---UpdateUserPassword--->>>", logger.Error(err))
		return &nb.LoginDataRes{}, err
	}

	return resp, nil
}
