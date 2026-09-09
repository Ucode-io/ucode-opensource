package service

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/grpc/client"
	span "github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"
)

type AgentService struct {
	nb.UnimplementedAgentServiceServer
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
}

func NewAgentService(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) *AgentService {
	return &AgentService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

// ==================== Agents ====================

func (s *AgentService) CreateAgent(ctx context.Context, req *nb.CreateAgentRequest) (*nb.Agent, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_agent.CreateAgent", req)
	defer dbSpan.Finish()

	s.log.Info("---CreateAgent--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.Agent().CreateAgent(ctx, req)
	if err != nil {
		s.log.Error("---CreateAgent--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *AgentService) GetAgent(ctx context.Context, req *nb.AgentPrimaryKey) (*nb.Agent, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_agent.GetAgent", req)
	defer dbSpan.Finish()

	s.log.Info("---GetAgent--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.Agent().GetAgentById(ctx, req)
	if err != nil {
		s.log.Error("---GetAgent--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *AgentService) GetAllAgents(ctx context.Context, req *nb.GetAllAgentsRequest) (*nb.GetAllAgentsResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_agent.GetAllAgents", req)
	defer dbSpan.Finish()

	s.log.Info("---GetAllAgents--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.Agent().GetAllAgents(ctx, req)
	if err != nil {
		s.log.Error("---GetAllAgents--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *AgentService) UpdateAgent(ctx context.Context, req *nb.UpdateAgentRequest) (*nb.Agent, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_agent.UpdateAgent", req)
	defer dbSpan.Finish()

	s.log.Info("---UpdateAgent--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.Agent().UpdateAgent(ctx, req)
	if err != nil {
		s.log.Error("---UpdateAgent--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *AgentService) DeleteAgent(ctx context.Context, req *nb.AgentPrimaryKey) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_agent.DeleteAgent", req)
	defer dbSpan.Finish()

	s.log.Info("---DeleteAgent--->>>", logger.Any("request", compactRequest(req)))

	err := s.strg.Agent().DeleteAgent(ctx, req)
	if err != nil {
		s.log.Error("---DeleteAgent--->>>", logger.Error(err))
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// ==================== Agent Runs ====================

func (s *AgentService) CreateAgentRun(ctx context.Context, req *nb.CreateAgentRunRequest) (*nb.AgentRun, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_agent.CreateAgentRun", req)
	defer dbSpan.Finish()

	s.log.Info("---CreateAgentRun--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.Agent().CreateAgentRun(ctx, req)
	if err != nil {
		s.log.Error("---CreateAgentRun--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *AgentService) UpdateAgentRun(ctx context.Context, req *nb.UpdateAgentRunRequest) (*nb.AgentRun, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_agent.UpdateAgentRun", req)
	defer dbSpan.Finish()

	s.log.Info("---UpdateAgentRun--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.Agent().UpdateAgentRun(ctx, req)
	if err != nil {
		s.log.Error("---UpdateAgentRun--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *AgentService) GetAgentRun(ctx context.Context, req *nb.AgentRunPrimaryKey) (*nb.AgentRun, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_agent.GetAgentRun", req)
	defer dbSpan.Finish()

	s.log.Info("---GetAgentRun--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.Agent().GetAgentRunById(ctx, req)
	if err != nil {
		s.log.Error("---GetAgentRun--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *AgentService) GetAllAgentRuns(ctx context.Context, req *nb.GetAllAgentRunsRequest) (*nb.GetAllAgentRunsResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_agent.GetAllAgentRuns", req)
	defer dbSpan.Finish()

	s.log.Info("---GetAllAgentRuns--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.Agent().GetAllAgentRuns(ctx, req)
	if err != nil {
		s.log.Error("---GetAllAgentRuns--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}
