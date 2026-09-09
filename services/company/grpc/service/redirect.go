package service

import (
	"context"
	"strings"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	span "github.com/Ucode-io/ucode-opensource/services/company/pkg/jaeger"
	l "github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/company/storage"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// RedirectService ...
type RedirectService struct {
	storage storage.StorageI
	logger  l.Logger
	pb.UnimplementedRedirectUrlServiceServer
}

// NewRedirectService ...
func NewRedirectService(strg storage.StorageI, log l.Logger) *RedirectService {
	return &RedirectService{
		storage: strg,
		logger:  log,
	}
}

func (s *RedirectService) Create(ctx context.Context, req *pb.RedirectUrl) (*pb.RedirectUrl, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_redirect.Create", req)
	defer dbSpan.Finish()

	s.logger.Info("--CreateRedirect-- requested")

	req.From = "/" + strings.Trim(req.From, "/")
	req.To = "/" + strings.Trim(req.To, "/")

	res, err := s.storage.Redirect().Create(ctx, req)
	if err != nil {
		s.logger.Error("--CreateRedirect--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return res, nil
}

func (s *RedirectService) GetSingle(ctx context.Context, req *pb.GetSingleRedirectUrlReq) (*pb.RedirectUrl, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_redirect.GetSingle", req)
	defer dbSpan.Finish()

	s.logger.Info("--GetSingleRedirect-- requested")

	res, err := s.storage.Redirect().GetSingle(ctx, req)
	if err != nil {
		s.logger.Error("--GetSingleRedirect--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return res, nil
}

func (s *RedirectService) GetList(ctx context.Context, req *pb.GetListRedirectUrlReq) (*pb.GetListRedirectUrlRes, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_redirect.GetList", req)
	defer dbSpan.Finish()

	s.logger.Info("--GetListRedirect-- requested")

	res, err := s.storage.Redirect().GetList(ctx, req)
	if err != nil {
		s.logger.Error("--GetListRedirect--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	s.logger.Info("--GetListRedirect-- requested", l.Any("response", res))

	return res, nil
}

func (s *RedirectService) Update(ctx context.Context, req *pb.RedirectUrl) (*pb.RedirectUrl, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_redirect.Update", req)
	defer dbSpan.Finish()

	s.logger.Info("--UpdateRedirect-- requested", l.Any("req", req))

	req.From = "/" + strings.Trim(req.From, "/")
	req.To = "/" + strings.Trim(req.To, "/")

	res, err := s.storage.Redirect().Update(ctx, req)
	if err != nil {
		s.logger.Error("--UpdateRedirect--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return res, nil
}

func (s *RedirectService) Delete(ctx context.Context, req *pb.DeleteRedirectUrlReq) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_redirect.Delete", req)
	defer dbSpan.Finish()

	s.logger.Info("--DeleteRedirect-- requested")

	res, err := s.storage.Redirect().Delete(ctx, req)
	if err != nil {
		s.logger.Error("--DeleteRedirect--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return res, nil
}

func (s *RedirectService) UpdateOrder(ctx context.Context, req *pb.UpdateOrderRedirectUrlReq) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_redirect.UpdateOrder", req)
	defer dbSpan.Finish()

	s.logger.Info("--UpdateOrder-- requested")

	res, err := s.storage.Redirect().UpdateOrder(ctx, req)
	if err != nil {
		s.logger.Error("--UpdateOrder--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return res, nil
}
