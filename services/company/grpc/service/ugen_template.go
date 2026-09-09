package service

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	span "github.com/Ucode-io/ucode-opensource/services/company/pkg/jaeger"
	l "github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/company/storage"
)

type UgenTemplateService struct {
	storage  storage.StorageI
	logger   l.Logger
	services ServiceNodesI
	pb.UnimplementedUgenTemplateServiceServer
}

func NewUgenTemplateService(strg storage.StorageI, log l.Logger, services ServiceNodesI) *UgenTemplateService {
	return &UgenTemplateService{
		storage:  strg,
		logger:   log,
		services: services,
	}
}

func (s *UgenTemplateService) Create(ctx context.Context, req *pb.CreateUgenTemplateReq) (*pb.UgenTemplate, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ugen_template.Create", req)
	defer dbSpan.Finish()

	resp, err := s.storage.UgenTemplate().Create(ctx, req)
	if err != nil {
		s.logger.Error("--CreateUgenTemplate--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return resp, nil
}

func (s *UgenTemplateService) GetById(ctx context.Context, req *pb.GetUgenTemplateByIdReq) (*pb.UgenTemplate, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ugen_template.GetById", req)
	defer dbSpan.Finish()

	resp, err := s.storage.UgenTemplate().GetById(ctx, req.GetId(), req.GetUserId())
	if err != nil {
		s.logger.Error("--GetUgenTemplateById--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return resp, nil
}

func (s *UgenTemplateService) List(ctx context.Context, req *pb.GetUgenTemplateListReq) (*pb.GetUgenTemplateListResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ugen_template.List", req)
	defer dbSpan.Finish()

	resp, err := s.storage.UgenTemplate().GetList(ctx, req)
	if err != nil {
		s.logger.Error("--ListUgenTemplate--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return resp, nil
}

func (s *UgenTemplateService) Update(ctx context.Context, req *pb.UpdateUgenTemplateReq) (*pb.UgenTemplate, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ugen_template.Update", req)
	defer dbSpan.Finish()

	resp, err := s.storage.UgenTemplate().Update(ctx, req)
	if err != nil {
		s.logger.Error("--UpdateUgenTemplate--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return resp, nil
}

func (s *UgenTemplateService) SetPrice(ctx context.Context, req *pb.SetUgenTemplatePriceReq) (*pb.UgenTemplate, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ugen_template.SetPrice", req)
	defer dbSpan.Finish()

	resp, err := s.storage.UgenTemplate().SetPrice(ctx, req)
	if err != nil {
		s.logger.Error("--SetUgenTemplatePrice--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return resp, nil
}

func (s *UgenTemplateService) Delete(ctx context.Context, req *pb.DeleteUgenTemplateReq) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ugen_template.Delete", req)
	defer dbSpan.Finish()

	if err := s.storage.UgenTemplate().Delete(ctx, req.GetId()); err != nil {
		s.logger.Error("--DeleteUgenTemplate--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &emptypb.Empty{}, nil
}

func (s *UgenTemplateService) SetReaction(ctx context.Context, req *pb.SetUgenTemplateReactionReq) (*pb.UgenTemplateReaction, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ugen_template.SetReaction", req)
	defer dbSpan.Finish()

	if err := validateSetUgenTemplateReactionReq(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	resp, err := s.storage.UgenTemplate().SetReaction(ctx, req)
	if err != nil {
		s.logger.Error("--SetUgenTemplateReaction--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return resp, nil
}

func (s *UgenTemplateService) DeleteReaction(ctx context.Context, req *pb.DeleteUgenTemplateReactionReq) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ugen_template.DeleteReaction", req)
	defer dbSpan.Finish()

	if req.GetTemplateId() == "" {
		return nil, status.Error(codes.InvalidArgument, "template_id is required")
	}
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	if err := s.storage.UgenTemplate().DeleteReaction(ctx, req); err != nil {
		s.logger.Error("--DeleteUgenTemplateReaction--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &emptypb.Empty{}, nil
}

func (s *UgenTemplateService) ListReactions(ctx context.Context, req *pb.GetUgenTemplateReactionListReq) (*pb.GetUgenTemplateReactionListResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ugen_template.ListReactions", req)
	defer dbSpan.Finish()

	if req.GetTemplateId() == "" {
		return nil, status.Error(codes.InvalidArgument, "template_id is required")
	}
	if req.GetReactionType() != pb.UgenTemplateReactionType_UGEN_TEMPLATE_REACTION_TYPE_UNSPECIFIED &&
		req.GetReactionType() != pb.UgenTemplateReactionType_UGEN_TEMPLATE_REACTION_TYPE_LIKE &&
		req.GetReactionType() != pb.UgenTemplateReactionType_UGEN_TEMPLATE_REACTION_TYPE_DISLIKE {
		return nil, status.Error(codes.InvalidArgument, "reaction_type must be like or dislike")
	}

	resp, err := s.storage.UgenTemplate().ListReactions(ctx, req)
	if err != nil {
		s.logger.Error("--ListUgenTemplateReactions--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return resp, nil
}

func validateSetUgenTemplateReactionReq(req *pb.SetUgenTemplateReactionReq) error {
	if req.GetTemplateId() == "" {
		return fmt.Errorf("template_id is required")
	}
	if req.GetUserId() == "" {
		return fmt.Errorf("user_id is required")
	}
	switch req.GetReactionType() {
	case pb.UgenTemplateReactionType_UGEN_TEMPLATE_REACTION_TYPE_LIKE,
		pb.UgenTemplateReactionType_UGEN_TEMPLATE_REACTION_TYPE_DISLIKE:
		return nil
	default:
		return fmt.Errorf("reaction_type must be like or dislike")
	}
}
