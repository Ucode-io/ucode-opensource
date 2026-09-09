package gpt

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"

	"github.com/google/uuid"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/structpb"
)

func CreateField(req *models.CreateFieldAI) ([]*models.CreateVersionHistoryRequest, error) {

	var (
		resource   = req.Resource
		services   = req.Service
		respLogReq = []*models.CreateVersionHistoryRequest{}
		attributes = &structpb.Struct{}
		err        error
	)

	attributes, err = helper.ConvertMapToStruct(map[string]any{
		"label":        "",
		"defaultValue": "",
		"label_en":     req.Label,
	})
	if err != nil {
		return respLogReq, err
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		tables, err := services.GetBuilderServiceByType(resource.NodeType).Table().GetTablesByLabel(context.Background(), &obs.GetTablesByLabelReq{
			ProjectId: resource.ResourceEnvironmentId,
			Label:     req.Table,
		})
		if err != nil {
			return respLogReq, err
		}

		for _, table := range tables.Tables {
			var (
				logReq = models.CreateVersionHistoryRequest{
					Services:     services,
					NodeType:     resource.NodeType,
					ProjectId:    resource.ResourceEnvironmentId,
					ActionSource: "FIELD",
					ActionType:   "CREATE FIELD",
					UserInfo:     cast.ToString(req.UserId),
					TableSlug:    table.Slug,
				}
				field = &obs.CreateFieldRequest{
					Id:         uuid.NewString(),
					Attributes: attributes,
					Default:    "",
					Index:      "string",
					Label:      req.Label,
					Slug:       req.Slug,
					Type:       req.Type,
					TableId:    table.Id,
					ShowLabel:  true,
					ProjectId:  resource.ResourceEnvironmentId,
				}
			)

			resp, err := services.GetBuilderServiceByType(resource.NodeType).Field().Create(
				context.Background(),
				field,
			)
			if err != nil {
				logReq.Request = field
				logReq.Response = err.Error()
				return respLogReq, err
			}
			logReq.Request = field
			logReq.Response = resp
			logReq.Current = resp
			respLogReq = append(respLogReq, &logReq)
		}

	case pb.ResourceType_POSTGRESQL:
		tables, err := services.GoObjectBuilderService().Table().GetTablesByLabel(context.Background(), &nb.GetTablesByLabelReq{
			ProjectId: resource.ResourceEnvironmentId,
			Label:     req.Table,
		})
		if err != nil {
			return respLogReq, err
		}

		for _, table := range tables.Tables {
			var (
				logReq = models.CreateVersionHistoryRequest{
					Services:     services,
					NodeType:     resource.NodeType,
					ProjectId:    resource.ResourceEnvironmentId,
					ActionSource: "FIELD",
					ActionType:   "CREATE FIELD",
					UserInfo:     cast.ToString(req.UserId),
					TableSlug:    table.Slug,
				}
				field = &nb.CreateFieldRequest{
					Id:         uuid.NewString(),
					Attributes: attributes,
					Default:    "",
					Index:      "string",
					Label:      req.Label,
					Slug:       req.Slug,
					Type:       req.Type,
					TableId:    table.Id,
					ShowLabel:  true,
					ProjectId:  resource.ResourceEnvironmentId,
				}
			)

			resp, err := services.GoObjectBuilderService().Field().Create(
				context.Background(),
				field,
			)
			if err != nil {
				logReq.Request = field
				logReq.Response = err.Error()
				respLogReq = append(respLogReq, &logReq)
				return respLogReq, err
			}
			logReq.Request = field
			logReq.Response = resp
			logReq.Current = resp
			respLogReq = append(respLogReq, &logReq)
		}
	}

	return respLogReq, nil
}

func UpdateField(req *models.UpdateFieldAI) ([]*models.CreateVersionHistoryRequest, error) {

	var (
		resource   = req.Resource
		services   = req.Service
		respLogReq = []*models.CreateVersionHistoryRequest{}
	)

	// attributes, err := helper.ConvertMapToStruct(map[string]any{
	// 	"label":        "",
	// 	"defaultValue": "",
	// 	"label_en":     req.NewLabel,
	// 	"options":      options,
	// })
	// if err != nil {
	// 	return respLogReq, err
	// }
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:

		tables, err := services.GetBuilderServiceByType(resource.NodeType).Table().GetTablesByLabel(context.Background(), &obs.GetTablesByLabelReq{
			ProjectId: resource.ResourceEnvironmentId,
			Label:     req.Table,
		})
		if err != nil {
			return respLogReq, err
		}

		for _, table := range tables.Tables {

			fields, err := services.GetBuilderServiceByType(resource.NodeType).Field().GetAllByLabel(context.Background(), &obs.GetAllByLabelReq{
				ProjectId:  resource.ResourceEnvironmentId,
				FieldLabel: req.OldLabel,
				TableId:    table.Id,
			})
			if err != nil {
				return respLogReq, err
			}

			for _, field := range fields.Fields {

				if req.NewLabel != "" {
					field.Label = req.NewLabel
					atr, err := helper.ConvertStructToResponse(field.Attributes)
					if err != nil {
						return respLogReq, err
					}
					atr["label_en"] = req.NewLabel

					field.Attributes, err = helper.ConvertMapToStruct(atr)
					if err != nil {
						return respLogReq, err
					}
				}

				if req.NewType != "" {
					field.Type = req.NewType
				}

				field.ProjectId = resource.ResourceEnvironmentId

				_, _ = services.GetBuilderServiceByType(resource.NodeType).Field().Update(
					context.Background(),
					field,
				)
			}
		}
	case pb.ResourceType_POSTGRESQL:
		tables, err := services.GoObjectBuilderService().Table().GetTablesByLabel(context.Background(), &nb.GetTablesByLabelReq{
			ProjectId: resource.ResourceEnvironmentId,
			Label:     req.Table,
		})
		if err != nil {
			return respLogReq, err
		}

		for _, table := range tables.Tables {
			fields, err := services.GoObjectBuilderService().Field().GetAllByLabel(context.Background(), &nb.GetAllByLabelReq{
				ProjectId:  resource.ResourceEnvironmentId,
				FieldLabel: req.OldLabel,
				TableId:    table.Id,
			})
			if err != nil {
				return respLogReq, err
			}

			for _, field := range fields.Fields {

				if req.NewLabel != "" {
					field.Label = req.NewLabel
					atr, err := helper.ConvertStructToResponse(field.Attributes)
					if err != nil {
						return respLogReq, err
					}
					atr["label_en"] = req.NewLabel

					field.Attributes, err = helper.ConvertMapToStruct(atr)
					if err != nil {
						return respLogReq, err
					}
				}

				if req.NewType != "" {
					field.Type = req.NewType
				}

				field.ProjectId = resource.ResourceEnvironmentId

				_, _ = services.GoObjectBuilderService().Field().Update(
					context.Background(),
					field,
				)
			}

		}
	}

	return nil, nil
}

func DeleteField(req *models.DeleteFieldAI) ([]*models.CreateVersionHistoryRequest, error) {

	var (
		resource   = req.Resource
		services   = req.Service
		respLogReq = []*models.CreateVersionHistoryRequest{}
	)

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		tables, err := services.GetBuilderServiceByType(resource.NodeType).Table().GetTablesByLabel(context.Background(), &obs.GetTablesByLabelReq{
			ProjectId: resource.ResourceEnvironmentId,
			Label:     req.Table,
		})
		if err != nil {
			return respLogReq, err
		}

		for _, table := range tables.Tables {

			fields, err := services.GetBuilderServiceByType(resource.NodeType).Field().GetAllByLabel(context.Background(), &obs.GetAllByLabelReq{
				ProjectId:  resource.ResourceEnvironmentId,
				FieldLabel: req.Label,
				TableId:    table.Id,
			})
			if err != nil {
				return respLogReq, err
			}

			for _, field := range fields.Fields {

				field.ProjectId = resource.ResourceEnvironmentId

				_, _ = services.GetBuilderServiceByType(resource.NodeType).Field().Delete(
					context.Background(),
					&obs.FieldPrimaryKey{
						Id:        field.Id,
						ProjectId: resource.ResourceEnvironmentId,
					},
				)
			}
		}
	case pb.ResourceType_POSTGRESQL:
		tables, err := services.GoObjectBuilderService().Table().GetTablesByLabel(context.Background(), &nb.GetTablesByLabelReq{
			ProjectId: resource.ResourceEnvironmentId,
			Label:     req.Table,
		})
		if err != nil {
			return respLogReq, err
		}

		for _, table := range tables.Tables {

			fields, err := services.GoObjectBuilderService().Field().GetAllByLabel(context.Background(), &nb.GetAllByLabelReq{
				ProjectId:  resource.ResourceEnvironmentId,
				FieldLabel: req.Label,
				TableId:    table.Id,
			})
			if err != nil {
				return respLogReq, err
			}

			for _, field := range fields.Fields {

				field.ProjectId = resource.ResourceEnvironmentId

				_, _ = services.GoObjectBuilderService().Field().Delete(
					context.Background(),
					&nb.FieldPrimaryKey{
						Id:        field.Id,
						ProjectId: resource.ResourceEnvironmentId,
					},
				)
			}
		}
	}

	return respLogReq, nil
}
