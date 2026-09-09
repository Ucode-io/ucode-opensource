package service

import (
	"context"
	"errors"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
	span "github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// aiEditPromptService serves the per-project AI edit prompt overrides.
// Prompt content may carry project-specific instructions, so requests and
// responses are logged/traced only as redacted summaries, never with content.
type aiEditPromptService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	nb.UnimplementedAiEditPromptServiceServer
}

func NewAiEditPromptService(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) *aiEditPromptService {
	return &aiEditPromptService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

func (v *aiEditPromptService) GetAiEditPrompt(ctx context.Context, req *nb.GetAiEditPromptRequest) (*nb.AiEditPrompt, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ai_edit_prompt.GetAiEditPrompt", req)
	defer dbSpan.Finish()

	prompt, err := v.strg.AiEditPrompt().Get(ctx, req.GetResourceEnvId(), req.GetPromptKind())
	if err != nil {
		v.logAiEditPromptError("GetAiEditPrompt", req.GetPromptKind(), err)
		return nil, aiEditPromptStatusError(err)
	}

	return aiEditPromptToProto(prompt), nil
}

func (v *aiEditPromptService) GetAiEditPrompts(ctx context.Context, req *nb.GetAiEditPromptsRequest) (*nb.GetAiEditPromptsResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ai_edit_prompt.GetAiEditPrompts", req)
	defer dbSpan.Finish()

	prompts, err := v.strg.AiEditPrompt().GetAll(ctx, req.GetResourceEnvId())
	if err != nil {
		v.logAiEditPromptError("GetAiEditPrompts", "", err)
		return nil, aiEditPromptStatusError(err)
	}

	resp := &nb.GetAiEditPromptsResponse{Prompts: make([]*nb.AiEditPrompt, 0, len(prompts))}
	for _, prompt := range prompts {
		resp.Prompts = append(resp.Prompts, aiEditPromptToProto(prompt))
	}
	return resp, nil
}

func (v *aiEditPromptService) UpsertAiEditPrompt(ctx context.Context, req *nb.UpsertAiEditPromptRequest) (*nb.AiEditPrompt, error) {
	// The raw request carries prompt content; trace only a redacted summary.
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ai_edit_prompt.UpsertAiEditPrompt", map[string]any{
		"resource_env_id":   req.GetResourceEnvId(),
		"prompt_kind":       req.GetPromptKind(),
		"content_bytes":     len(req.GetContent()),
		"expected_revision": req.GetExpectedRevision(),
	})
	defer dbSpan.Finish()

	prompt, err := v.strg.AiEditPrompt().Upsert(ctx, &models.UpsertAiEditPromptRequest{
		ResourceEnvID:    req.GetResourceEnvId(),
		PromptKind:       req.GetPromptKind(),
		Content:          req.GetContent(),
		ExpectedRevision: req.GetExpectedRevision(),
		UpdatedByUserID:  req.GetUpdatedByUserId(),
	})
	if err != nil {
		v.logAiEditPromptError("UpsertAiEditPrompt", req.GetPromptKind(), err)
		return nil, aiEditPromptStatusError(err)
	}

	return aiEditPromptToProto(prompt), nil
}

func (v *aiEditPromptService) DeleteAiEditPrompt(ctx context.Context, req *nb.DeleteAiEditPromptRequest) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_ai_edit_prompt.DeleteAiEditPrompt", req)
	defer dbSpan.Finish()

	err := v.strg.AiEditPrompt().Delete(ctx, &models.DeleteAiEditPromptRequest{
		ResourceEnvID:    req.GetResourceEnvId(),
		PromptKind:       req.GetPromptKind(),
		ExpectedRevision: req.GetExpectedRevision(),
	})
	if err != nil {
		v.logAiEditPromptError("DeleteAiEditPrompt", req.GetPromptKind(), err)
		return nil, aiEditPromptStatusError(err)
	}

	return &emptypb.Empty{}, nil
}

func (v *aiEditPromptService) logAiEditPromptError(method, promptKind string, err error) {
	v.log.Error("---"+method+"--->>>", logger.String("prompt_kind", promptKind), logger.Error(err))
}

func aiEditPromptToProto(prompt *models.AiEditPrompt) *nb.AiEditPrompt {
	return &nb.AiEditPrompt{
		PromptKind:      prompt.PromptKind,
		Content:         prompt.Content,
		Revision:        prompt.Revision,
		UpdatedByUserId: prompt.UpdatedByUserID,
		CreatedAt:       prompt.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       prompt.UpdatedAt.Format(time.RFC3339),
	}
}

// aiEditPromptStatusError maps storage errors to gRPC codes the gateway's REST
// layer translates into 400/404/409/503 responses.
func aiEditPromptStatusError(err error) error {
	switch {
	case errors.Is(err, storage.ErrAiEditPromptNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, storage.ErrAiEditPromptRevisionConflict):
		return status.Error(codes.Aborted, err.Error())
	case errors.Is(err, storage.ErrAiEditPromptTableMissing):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, storage.ErrInvalidAiEditPromptKind),
		errors.Is(err, storage.ErrInvalidAiEditPromptContent),
		errors.Is(err, storage.ErrAiEditPromptContentTooLarge),
		errors.Is(err, storage.ErrInvalidAiEditPromptRevision):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return err
	}
}
