package service

import (
	"context"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	l "github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
)

// AdminService ...
type pingService struct {
	logger l.Logger
	pb.UnimplementedCompanyPingServiceServer
}

func NewCompanyPingService(log l.Logger, services ServiceNodesI) *pingService {
	return &pingService{
		logger: log,
	}
}

func (s *pingService) Ping(ctx context.Context, req *pb.PingRequest) (res *pb.PingResponse, err error) {
	s.logger.Info("--CompanyServicePing-- requested")

	return &pb.PingResponse{Message: "Pong"}, nil
}
