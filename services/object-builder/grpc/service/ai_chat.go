package service

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/grpc/client"
	span "github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"
)

type AiChatService struct {
	nb.UnimplementedAiChatServiceServer
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
}

func NewAiChatService(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) *AiChatService {
	return &AiChatService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

// ==================== Chats ====================

func (s *AiChatService) CreateChat(ctx context.Context, req *nb.CreateChatRequest) (*nb.Chat, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ai_chat.CreateChat", req)
	defer dbSpan.Finish()

	s.log.Info("---CreateChat--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.AiChat().CreateChat(ctx, req)
	if err != nil {
		s.log.Error("---CreateChat--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *AiChatService) GetChat(ctx context.Context, req *nb.ChatPrimaryKey) (*nb.Chat, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ai_chat.GetChat", req)
	defer dbSpan.Finish()

	s.log.Info("---GetChat--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.AiChat().GetChatById(ctx, req)
	if err != nil {
		s.log.Error("---GetChat--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *AiChatService) GetChatByProjectId(ctx context.Context, req *nb.ChatByProjectIdRequest) (*nb.Chat, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ai_chat.GetChatByProjectId", req)
	defer dbSpan.Finish()

	s.log.Info("---GetChatByProjectId--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.AiChat().GetChatByProjectId(ctx, req)
	if err != nil {
		s.log.Error("---GetChatByProjectId--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *AiChatService) GetAllChats(ctx context.Context, req *nb.GetAllChatsRequest) (*nb.GetAllChatsResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ai_chat.GetAllChats", req)
	defer dbSpan.Finish()

	s.log.Info("---GetAllChats--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.AiChat().GetAllChats(ctx, req)
	if err != nil {
		s.log.Error("---GetAllChats--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *AiChatService) UpdateChat(ctx context.Context, req *nb.UpdateChatRequest) (*nb.Chat, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ai_chat.UpdateChat", req)
	defer dbSpan.Finish()

	s.log.Info("---UpdateChat--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.AiChat().UpdateChat(ctx, req)
	if err != nil {
		s.log.Error("---UpdateChat--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *AiChatService) DeleteChat(ctx context.Context, req *nb.ChatPrimaryKey) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ai_chat.DeleteChat", req)
	defer dbSpan.Finish()

	s.log.Info("---DeleteChat--->>>", logger.Any("request", compactRequest(req)))

	err := s.strg.AiChat().DeleteChat(ctx, req)
	if err != nil {
		s.log.Error("---DeleteChat--->>>", logger.Error(err))
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// ==================== Messages ====================

func (s *AiChatService) CreateMessage(ctx context.Context, req *nb.CreateMessageRequest) (*nb.Message, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ai_chat.CreateMessage", req)
	defer dbSpan.Finish()

	s.log.Info("---CreateMessage--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.AiChat().CreateMessage(ctx, req)
	if err != nil {
		s.log.Error("---CreateMessage--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *AiChatService) GetMessages(ctx context.Context, req *nb.GetMessagesRequest) (*nb.GetMessagesResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ai_chat.GetMessages", req)
	defer dbSpan.Finish()

	s.log.Info("---GetMessages--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.AiChat().GetMessages(ctx, req)
	if err != nil {
		s.log.Error("---GetMessages--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *AiChatService) DeleteMessage(ctx context.Context, req *nb.MessagePrimaryKey) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ai_chat.DeleteMessage", req)
	defer dbSpan.Finish()

	s.log.Info("---DeleteMessage--->>>", logger.Any("request", compactRequest(req)))

	err := s.strg.AiChat().DeleteMessage(ctx, req)
	if err != nil {
		s.log.Error("---DeleteMessage--->>>", logger.Error(err))
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *AiChatService) SetMessageReaction(ctx context.Context, req *nb.SetMessageReactionRequest) (*nb.MessageReaction, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ai_chat.SetMessageReaction", req)
	defer dbSpan.Finish()

	s.log.Info("---SetMessageReaction--->>>", logger.Any("request", compactRequest(req)))

	if err := validateSetMessageReactionRequest(req); err != nil {
		return nil, err
	}

	resp, err := s.strg.AiChat().SetMessageReaction(ctx, req)
	if err != nil {
		s.log.Error("---SetMessageReaction--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *AiChatService) DeleteMessageReaction(ctx context.Context, req *nb.DeleteMessageReactionRequest) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ai_chat.DeleteMessageReaction", req)
	defer dbSpan.Finish()

	s.log.Info("---DeleteMessageReaction--->>>", logger.Any("request", compactRequest(req)))

	if err := validateDeleteMessageReactionRequest(req); err != nil {
		return nil, err
	}

	if err := s.strg.AiChat().DeleteMessageReaction(ctx, req); err != nil {
		s.log.Error("---DeleteMessageReaction--->>>", logger.Error(err))
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func validateSetMessageReactionRequest(req *nb.SetMessageReactionRequest) error {
	if strings.TrimSpace(req.GetResourceEnvId()) == "" {
		return status.Error(codes.InvalidArgument, "resource_env_id is required")
	}
	if strings.TrimSpace(req.GetMessageId()) == "" {
		return status.Error(codes.InvalidArgument, "message_id is required")
	}
	if strings.TrimSpace(req.GetUserId()) == "" {
		return status.Error(codes.InvalidArgument, "user_id is required")
	}
	switch req.GetReactionType() {
	case nb.MessageReactionType_MESSAGE_REACTION_TYPE_LIKE,
		nb.MessageReactionType_MESSAGE_REACTION_TYPE_DISLIKE:
		return nil
	default:
		return status.Error(codes.InvalidArgument, "reaction_type must be like or dislike")
	}
}

func validateDeleteMessageReactionRequest(req *nb.DeleteMessageReactionRequest) error {
	if strings.TrimSpace(req.GetResourceEnvId()) == "" {
		return status.Error(codes.InvalidArgument, "resource_env_id is required")
	}
	if strings.TrimSpace(req.GetMessageId()) == "" {
		return status.Error(codes.InvalidArgument, "message_id is required")
	}
	if strings.TrimSpace(req.GetUserId()) == "" {
		return status.Error(codes.InvalidArgument, "user_id is required")
	}
	return nil
}

// ==================== File Versions ====================

func (s *AiChatService) CreateFileVersion(ctx context.Context, req *nb.CreateFileVersionRequest) (*nb.FileVersion, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ai_chat.CreateFileVersion", req)
	defer dbSpan.Finish()

	s.log.Info("---CreateFileVersion--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.AiChat().CreateFileVersion(ctx, req)
	if err != nil {
		s.log.Error("---CreateFileVersion--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *AiChatService) GetFileVersions(ctx context.Context, req *nb.GetFileVersionsRequest) (*nb.GetFileVersionsResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ai_chat.GetFileVersions", req)
	defer dbSpan.Finish()

	s.log.Info("---GetFileVersions--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.AiChat().GetFileVersions(ctx, req)
	if err != nil {
		s.log.Error("---GetFileVersions--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *AiChatService) GetFileVersionsByMessage(ctx context.Context, req *nb.GetFileVersionsByMessageRequest) (*nb.GetFileVersionsResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ai_chat.GetFileVersionsByMessage", req)
	defer dbSpan.Finish()

	s.log.Info("---GetFileVersionsByMessage--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.AiChat().GetFileVersionsByMessage(ctx, req)
	if err != nil {
		s.log.Error("---GetFileVersionsByMessage--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}
