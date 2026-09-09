package gpt

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"

	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	ob "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"

	"github.com/google/uuid"
	"github.com/spf13/cast"
)

func CreateItems(req *models.CreateItemsAI) ([]*models.CreateVersionHistoryRequest, error) {

	var (
		resource   = req.Resource
		services   = req.Service
		respLogReq = []*models.CreateVersionHistoryRequest{}
	)

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		field, err := services.GetBuilderServiceByType(resource.NodeType).ItemsService().GetSlugsByTable(
			context.Background(),
			&ob.GetSlugsByTableReq{
				ProjectId:  resource.ResourceEnvironmentId,
				TableLabel: req.Table,
			},
		)
		if err != nil {
			return respLogReq, err
		}

		params := make(map[string]any)

		for i, s := range field.Slugs {
			params[s] = req.Arguments[i]
		}

		params["guid"] = uuid.NewString()

		data, err := helper.ConvertMapToStruct(params)
		if err != nil {
			return respLogReq, err
		}

		_, err = services.GetBuilderServiceByType(resource.NodeType).ItemsService().Create(
			context.Background(),
			&ob.CommonMessage{
				TableSlug: field.TableSlug,
				ProjectId: resource.ResourceEnvironmentId,
				Data:      data,
			},
		)
		if err != nil {
			return respLogReq, err
		}

	case pb.ResourceType_POSTGRESQL:
	}

	return respLogReq, nil
}

func GenerateItems(req *models.GenerateItemsAI) ([]*models.CreateVersionHistoryRequest, error) {

	var (
		resource   = req.Resource
		services   = req.Service
		respLogReq = []*models.CreateVersionHistoryRequest{}
		prompt     string
		tableSlug  string
	)

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		field, err := services.GetBuilderServiceByType(resource.NodeType).ItemsService().GetSlugsByTable(
			context.Background(),
			&ob.GetSlugsByTableReq{
				ProjectId:  resource.ResourceEnvironmentId,
				TableLabel: req.Table,
			},
		)
		if err != nil {
			return respLogReq, err
		}

		prompt = fmt.Sprintf("Generate data for %s columns. Note: Use function generate_values and generate it %d times", strings.Join(field.Slugs, ", "), req.Count)

		tableSlug = field.TableSlug

	case pb.ResourceType_POSTGRESQL:
	}

	respMessages := []models.Message{
		{
			Role:    "system",
			Content: "Function generate_values which generate data for table n times. Use this function n times to create more data. Note: n gives in prompt",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	toolCalls, err := SendReqToGPT(respMessages)
	if err != nil {
		return respLogReq, err
	}

	for _, toolCall := range toolCalls {
		var (
			functionCall = toolCall.Function
			functionName = functionCall.Name
			arguments    map[string]any
		)

		err = json.Unmarshal([]byte(functionCall.Arguments), &arguments)
		if err != nil {
			continue
		}

		if functionName == "generate_values" {

			params := make(map[string]any)

			for _, arg := range cast.ToSlice(arguments["arguments"]) {
				col := cast.ToStringMap(arg)

				params[cast.ToString(col["key"])] = col["value"]
			}

			params["guid"] = uuid.NewString()

			data, err := helper.ConvertMapToStruct(params)
			if err != nil {
				return respLogReq, err
			}

			switch resource.ResourceType {
			case pb.ResourceType_MONGODB:
				_, err = services.GetBuilderServiceByType(resource.NodeType).ItemsService().Create(
					context.Background(),
					&ob.CommonMessage{
						TableSlug: tableSlug,
						ProjectId: resource.ResourceEnvironmentId,
						Data:      data,
					},
				)
				if err != nil {
					return respLogReq, err
				}
			case pb.ResourceType_POSTGRESQL:
			}
		}

	}

	return nil, nil
}

func UpdateItems(req *models.UpdateItemsAI) ([]*models.CreateVersionHistoryRequest, error) {

	var (
		resource   = req.Resource
		services   = req.Service
		respLogReq = []*models.CreateVersionHistoryRequest{}
	)

	data, err := helper.ConvertMapToStruct(map[string]any{
		"column":     req.OldColumn,
		"new_column": req.NewColumn,
	})
	if err != nil {
		return nil, err
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:

		_, err := services.GetBuilderServiceByType(resource.NodeType).ItemsService().UpdateBySearch(
			context.Background(),
			&ob.UpdateBySearchReq{
				Data:      data,
				Table:     req.Table,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			return respLogReq, err
		}
	case pb.ResourceType_POSTGRESQL:
	}

	return respLogReq, nil
}

func DeleteItems(req *models.UpdateItemsAI) ([]*models.CreateVersionHistoryRequest, error) {

	var (
		resource   = req.Resource
		services   = req.Service
		respLogReq = []*models.CreateVersionHistoryRequest{}
	)

	data, err := helper.ConvertMapToStruct(map[string]any{
		"column": req.OldColumn,
	})
	if err != nil {
		return nil, err
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		_, err := services.GetBuilderServiceByType(resource.NodeType).ItemsService().DeleteBySearch(
			context.Background(),
			&ob.DeleteBySearchReq{
				Data:      data,
				Table:     req.Table,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			return respLogReq, err
		}
	case pb.ResourceType_POSTGRESQL:
	}

	return respLogReq, nil
}
