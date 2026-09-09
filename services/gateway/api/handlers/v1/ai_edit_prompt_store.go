package v1

import (
	"context"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	pbo "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type aiEditPromptStore interface {
	GetAll(ctx context.Context, service services.ServiceManagerI, resourceEnvID string) ([]models.AIEditPrompt, error)
	Upsert(ctx context.Context, service services.ServiceManagerI, resourceEnvID string, prompt models.AIEditPrompt, expectedRevision int64) (models.AIEditPrompt, error)
	Delete(ctx context.Context, service services.ServiceManagerI, resourceEnvID, promptKind string, expectedRevision int64) error
}

// grpcAIEditPromptStore persists prompt overrides through the object-builder
// AiEditPromptService in the builder database selected by resourceEnvID.
type grpcAIEditPromptStore struct{}

func (grpcAIEditPromptStore) GetAll(ctx context.Context, service services.ServiceManagerI, resourceEnvID string) ([]models.AIEditPrompt, error) {
	resp, err := service.GoObjectBuilderService().AiEditPrompt().GetAiEditPrompts(
		ctx, &pbo.GetAiEditPromptsRequest{ResourceEnvId: resourceEnvID},
	)
	if err != nil {
		return nil, err
	}

	prompts := make([]models.AIEditPrompt, 0, len(resp.GetPrompts()))
	for _, prompt := range resp.GetPrompts() {
		prompts = append(prompts, aiEditPromptFromProto(prompt))
	}
	return prompts, nil
}

func (grpcAIEditPromptStore) Upsert(ctx context.Context, service services.ServiceManagerI, resourceEnvID string, prompt models.AIEditPrompt, expectedRevision int64) (models.AIEditPrompt, error) {
	resp, err := service.GoObjectBuilderService().AiEditPrompt().UpsertAiEditPrompt(
		ctx, &pbo.UpsertAiEditPromptRequest{
			ResourceEnvId:    resourceEnvID,
			PromptKind:       prompt.PromptKind,
			Content:          prompt.Content,
			ExpectedRevision: expectedRevision,
			UpdatedByUserId:  prompt.UpdatedByUserID,
		},
	)
	if err != nil {
		return models.AIEditPrompt{}, err
	}
	return aiEditPromptFromProto(resp), nil
}

func (grpcAIEditPromptStore) Delete(ctx context.Context, service services.ServiceManagerI, resourceEnvID, promptKind string, expectedRevision int64) error {
	_, err := service.GoObjectBuilderService().AiEditPrompt().DeleteAiEditPrompt(
		ctx, &pbo.DeleteAiEditPromptRequest{
			ResourceEnvId:    resourceEnvID,
			PromptKind:       promptKind,
			ExpectedRevision: expectedRevision,
		},
	)
	return err
}

func aiEditPromptFromProto(prompt *pbo.AiEditPrompt) models.AIEditPrompt {
	return models.AIEditPrompt{
		PromptKind:      prompt.GetPromptKind(),
		Content:         prompt.GetContent(),
		Revision:        prompt.GetRevision(),
		UpdatedByUserID: prompt.GetUpdatedByUserId(),
		CreatedAt:       prompt.GetCreatedAt(),
		UpdatedAt:       prompt.GetUpdatedAt(),
	}
}

type unavailableAIEditPromptStore struct{}

func (unavailableAIEditPromptStore) GetAll(context.Context, services.ServiceManagerI, string) ([]models.AIEditPrompt, error) {
	return nil, status.Error(codes.Unimplemented, "AI edit prompt storage client is not generated")
}

func (unavailableAIEditPromptStore) Upsert(context.Context, services.ServiceManagerI, string, models.AIEditPrompt, int64) (models.AIEditPrompt, error) {
	return models.AIEditPrompt{}, status.Error(codes.Unimplemented, "AI edit prompt storage client is not generated")
}

func (unavailableAIEditPromptStore) Delete(context.Context, services.ServiceManagerI, string, string, int64) error {
	return status.Error(codes.Unimplemented, "AI edit prompt storage client is not generated")
}
