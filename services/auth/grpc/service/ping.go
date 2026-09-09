package service

import (
	"context"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"

	l "github.com/Ucode-io/ucode-opensource/services/auth/pkg/logger"
)

// / AdminService ...
type PingService struct {
	logger l.LoggerI
	pb.UnimplementedAuthPingServiceServer
}

func NewPingService(log l.LoggerI, services ServiceNodesI) *PingService {
	return &PingService{
		logger: log,
	}
}

func (s *PingService) Ping(ctx context.Context, req *pb.PingRequest) (res *pb.PingResponse, err error) {
	s.logger.Info("--AuthServicePing-- requested")

	return &pb.PingResponse{Message: "Pong"}, nil
}
