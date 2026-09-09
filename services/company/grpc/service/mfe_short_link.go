package service

import (
	"context"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	span "github.com/Ucode-io/ucode-opensource/services/company/pkg/jaeger"
	l "github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/company/storage"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type MfeShortLinkService struct {
	storage storage.StorageI
	logger  l.Logger
	pb.UnimplementedMfeShortLinkServiceServer
}

func NewMfeShortLinkService(strg storage.StorageI, log l.Logger) *MfeShortLinkService {
	return &MfeShortLinkService{storage: strg, logger: log}
}

func (s *MfeShortLinkService) Create(ctx context.Context, req *pb.MfeShortLink) (*pb.MfeShortLink, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_mfe_short_link.Create", req)
	defer dbSpan.Finish()

	if req.Slug == "" || req.Url == "" || req.McpProjectId == "" || req.FunctionId == "" {
		return nil, status.Error(codes.InvalidArgument, "slug, url, mcp_project_id and function_id are required")
	}

	// Storage returns properly-coded gRPC status errors via HandleDatabaseError.
	// AlreadyExists (23505) is preserved so the caller can retry with a different slug.
	return s.storage.MfeShortLink().Create(ctx, req)
}

func (s *MfeShortLinkService) GetBySlug(ctx context.Context, req *pb.MfeShortLinkSlugReq) (*pb.MfeShortLink, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_mfe_short_link.GetBySlug", req)
	defer dbSpan.Finish()

	if req.Slug == "" {
		return nil, status.Error(codes.InvalidArgument, "slug is required")
	}

	return s.storage.MfeShortLink().GetBySlug(ctx, req)
}

func (s *MfeShortLinkService) GetByFunctionId(ctx context.Context, req *pb.MfeShortLinkFunctionReq) (*pb.MfeShortLink, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_mfe_short_link.GetByFunctionId", req)
	defer dbSpan.Finish()

	if req.FunctionId == "" {
		return nil, status.Error(codes.InvalidArgument, "function_id is required")
	}

	return s.storage.MfeShortLink().GetByFunctionId(ctx, req)
}

func (s *MfeShortLinkService) DeleteByProjectId(ctx context.Context, req *pb.MfeShortLinkProjectReq) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_mfe_short_link.DeleteByProjectId", req)
	defer dbSpan.Finish()

	if req.ProjectId == "" {
		return nil, status.Error(codes.InvalidArgument, "project_id is required")
	}

	return s.storage.MfeShortLink().DeleteByProjectId(ctx, req)
}