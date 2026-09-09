package gpt

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
)

func CreateRelation(req *models.CreateRelationAI) ([]*models.CreateVersionHistoryRequest, error) {
	var (
		resource   = req.Resource
		services   = req.Service
		respLogReq = []*models.CreateVersionHistoryRequest{}
	)

	attributes, err := helper.ConvertMapToStruct(map[string]any{
		"label_to_en": req.TableFrom,
		"label_en":    req.TableTo,
	})
	if err != nil {
		return respLogReq, err
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		tablesFrom, err := services.GetBuilderServiceByType(resource.NodeType).Table().GetTablesByLabel(context.Background(), &obs.GetTablesByLabelReq{
			ProjectId: resource.ResourceEnvironmentId,
			Label:     req.TableFrom,
		})
		if err != nil {
			return respLogReq, err
		}

		tablesTo, err := services.GetBuilderServiceByType(resource.NodeType).Table().GetTablesByLabel(context.Background(), &obs.GetTablesByLabelReq{
			ProjectId: resource.ResourceEnvironmentId,
			Label:     req.TableTo,
		})
		if err != nil {
			return respLogReq, err
		}

		for _, from := range tablesFrom.Tables {
			for _, to := range tablesTo.Tables {

				var (
					relation = &obs.CreateRelationRequest{
						TableFrom:         from.Slug,
						TableTo:           to.Slug,
						Attributes:        attributes,
						RelationTableSlug: to.Slug,
						Type:              req.RelationType,
						ProjectId:         resource.ResourceEnvironmentId,
					}
				)

				relation.AutoFilters = append(relation.AutoFilters, &obs.AutoFilter{
					FieldTo:   "",
					FieldFrom: "",
				})

				fields, err := services.GetBuilderServiceByType(resource.NodeType).Field().GetIdsByLabel(context.Background(), &obs.GetIdsByLabelReq{
					ProjectId:  resource.ResourceEnvironmentId,
					FieldLabel: req.ViewField,
					TableId:    to.Id,
				})
				if err != nil {
					return respLogReq, err
				}

				relation.ViewFields = fields.Ids

				if req.RelationType == "Many2Many" {
					relation.ViewType = req.ViewType
				}

				_, err = services.GetBuilderServiceByType(resource.NodeType).Relation().Create(
					context.Background(),
					relation,
				)
				if err != nil {
					return respLogReq, err
				}
			}
		}

	case pb.ResourceType_POSTGRESQL:
	}

	return nil, nil
}

func DeleteRelation(req *models.DeleteRelationAI) ([]*models.CreateVersionHistoryRequest, error) {

	var (
		resource   = req.Resource
		services   = req.Service
		respLogReq = []*models.CreateVersionHistoryRequest{}
	)

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		tablesFrom, err := services.GetBuilderServiceByType(resource.NodeType).Table().GetTablesByLabel(context.Background(), &obs.GetTablesByLabelReq{
			ProjectId: resource.ResourceEnvironmentId,
			Label:     req.TableFrom,
		})
		if err != nil {
			return respLogReq, err
		}

		tablesTo, err := services.GetBuilderServiceByType(resource.NodeType).Table().GetTablesByLabel(context.Background(), &obs.GetTablesByLabelReq{
			ProjectId: resource.ResourceEnvironmentId,
			Label:     req.TableTo,
		})
		if err != nil {
			return respLogReq, err
		}

		relation, err := services.GetBuilderServiceByType(resource.NodeType).Relation().GetIds(context.Background(), &obs.GetIdsReq{
			ProjectId: resource.ResourceEnvironmentId,
			TableFrom: tablesFrom.Tables[0].Slug,
			TableTo:   tablesTo.Tables[0].Slug,
			Type:      req.RelationType,
		})
		if err != nil {
			return respLogReq, err
		}

		for _, id := range relation.Ids {
			_, err := services.GetBuilderServiceByType(resource.NodeType).Relation().Delete(context.Background(), &obs.RelationPrimaryKey{
				ProjectId: resource.ResourceEnvironmentId,
				Id:        id,
			})
			if err != nil {
				return respLogReq, err
			}
		}

	case pb.ResourceType_POSTGRESQL:
	}

	return nil, nil
}
