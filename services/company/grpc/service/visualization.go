package service

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/company/config"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	span "github.com/Ucode-io/ucode-opensource/services/company/pkg/jaeger"
	l "github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/metabase"
)

type VisualizationService struct {
	logger l.Logger
	cfg    config.Config
	pb.UnimplementedVisualizationServiceServer
}

func NewVisualizationService(log l.Logger, config config.Config) *VisualizationService {
	return &VisualizationService{
		logger: log,
		cfg:    config,
	}
}

func (s *VisualizationService) GetMetabaseDashboards(ctx context.Context, req *pb.GetMetabaseDashboardsRequest) (*pb.GetMetabaseDashboardsResponse, error) {
	dbSpan, _ := span.StartSpanFromContext(ctx, "grpc_billing.GetMetabaseDashboards", req)
	defer dbSpan.Finish()

	session, err := metabase.GetSession(metabase.GetSessionRequest{
		Username: req.Username,
		Password: req.Password,
		Cfg:      s.cfg,
	})
	if err != nil {
		return nil, err
	}

	resp, err := metabase.GetDashboards(metabase.GetDashboardsRequest{
		Session: session,
		Cfg:     s.cfg,
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *VisualizationService) GetMetabasePublicUrl(ctx context.Context, req *pb.GetMetabasePublicUrlRequest) (*pb.GetMetabasePublicUrlResponse, error) {
	dbSpan, _ := span.StartSpanFromContext(ctx, "grpc_billing.GetMetabasePublicUrl", req)
	defer dbSpan.Finish()

	publicUrl, err := metabase.GetPublicUrl(metabase.GetPublicUrlRequest{
		DashboardId: int(req.DashboardId),
		Cfg:         s.cfg,
	})
	if err != nil {
		return nil, err
	}

	return &pb.GetMetabasePublicUrlResponse{Url: publicUrl}, nil
}
