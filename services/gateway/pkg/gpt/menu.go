package gpt

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"

	"github.com/spf13/cast"
)

func CreateMenu(reqBody *models.CreateMenuAI) ([]*models.CreateVersionHistoryRequest, error) {

	attributes, err := helper.ConvertMapToStruct(map[string]any{
		"label":    "",
		"label_en": reqBody.Label,
	})
	if err != nil {
		return nil, err
	}

	var (
		respLogReq   = []*models.CreateVersionHistoryRequest{}
		mongoRequest = &obs.CreateMenuRequest{
			Label:      reqBody.Label,
			ParentId:   "c57eedc3-a954-4262-a0af-376c65b5a284",
			Type:       "FOLDER",
			ProjectId:  reqBody.Resource.ResourceEnvironmentId,
			Attributes: attributes,
		}
		psqlRequest = &nb.CreateMenuRequest{
			Label:      reqBody.Label,
			ParentId:   "c57eedc3-a954-4262-a0af-376c65b5a284",
			Type:       "FOLDER",
			ProjectId:  reqBody.Resource.ResourceEnvironmentId,
			Attributes: attributes,
		}
		logReq = models.CreateVersionHistoryRequest{
			Services:     reqBody.Service,
			NodeType:     reqBody.Resource.NodeType,
			ProjectId:    reqBody.Resource.ResourceEnvironmentId,
			ActionSource: "MENU",
			ActionType:   "CREATE MENU",
			UserInfo:     cast.ToString(reqBody.UserId),
			Request:      &mongoRequest,
			TableSlug:    "Menu",
		}
	)

	// ? there in each ResourceType should write to db or redis
	switch reqBody.Resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := reqBody.Service.GetBuilderServiceByType(reqBody.Resource.NodeType).Menu().Create(
			context.Background(),
			mongoRequest,
		)

		if err != nil {
			logReq.Response = err.Error()
			respLogReq = append(respLogReq, &logReq)
			return respLogReq, err
		} else {
			logReq.Response = resp
			logReq.Current = resp
			respLogReq = append(respLogReq, &logReq)
		}

	case pb.ResourceType_POSTGRESQL:
		resp, err := reqBody.Service.GoObjectBuilderService().Menu().Create(
			context.Background(),
			psqlRequest,
		)

		if err != nil {
			logReq.Response = err.Error()
			return nil, err
		} else {
			logReq.Response = resp
			logReq.Current = resp
			respLogReq = append(respLogReq, &logReq)
		}
	}

	return respLogReq, nil
}

func DeleteMenu(reqBody *models.DeleteMenuAI) ([]*models.CreateVersionHistoryRequest, error) {
	var (
		respLogReq = []*models.CreateVersionHistoryRequest{}
		logReq     = models.CreateVersionHistoryRequest{
			Services:     reqBody.Service,
			NodeType:     reqBody.Resource.NodeType,
			ProjectId:    reqBody.Resource.ResourceEnvironmentId,
			ActionSource: "MENU",
			ActionType:   "DELETE MENU",
			UserInfo:     cast.ToString(reqBody.UserId),
			TableSlug:    "Menu",
		}
	)

	menus, err := reqBody.Service.GetBuilderServiceByType(reqBody.Resource.NodeType).Menu().GetByLabel(
		context.Background(),
		&obs.MenuPrimaryKey{Label: reqBody.Label, ProjectId: reqBody.Resource.ResourceEnvironmentId},
	)
	if err != nil {
		return respLogReq, err
	}

	for _, menu := range menus.Menus {

		_, err := reqBody.Service.GetBuilderServiceByType(reqBody.Resource.NodeType).Menu().Delete(
			context.Background(),
			&obs.MenuPrimaryKey{
				ProjectId: reqBody.Resource.ResourceEnvironmentId,
				Id:        menu.Id,
			},
		)
		if err != nil {
			logReq.Response = err.Error()
			respLogReq = append(respLogReq, &logReq)
			return respLogReq, err
		}

		respLogReq = append(respLogReq, &logReq)
	}

	return respLogReq, nil
}

func UpdateMenu(reqBody *models.UpdateMenuAI) ([]*models.CreateVersionHistoryRequest, error) {
	var (
		respLogReq = []*models.CreateVersionHistoryRequest{}
		logReq     = models.CreateVersionHistoryRequest{
			Services:     reqBody.Service,
			NodeType:     reqBody.Resource.NodeType,
			ProjectId:    reqBody.Resource.ResourceEnvironmentId,
			ActionSource: "MENU",
			ActionType:   "UPDATE MENU",
			UserInfo:     cast.ToString(reqBody.UserId),
			TableSlug:    "Menu",
		}
	)

	menus, err := reqBody.Service.GetBuilderServiceByType(reqBody.Resource.NodeType).Menu().GetByLabel(
		context.Background(),
		&obs.MenuPrimaryKey{Label: reqBody.OldLabel, ProjectId: reqBody.Resource.ResourceEnvironmentId},
	)
	if err != nil {
		return respLogReq, err
	}

	for _, menu := range menus.Menus {
		resp, err := reqBody.Service.GetBuilderServiceByType(reqBody.Resource.NodeType).Menu().Update(
			context.Background(),
			&obs.Menu{
				ProjectId:       reqBody.Resource.ResourceEnvironmentId,
				Label:           reqBody.NewLabel,
				Id:              menu.Id,
				Icon:            menu.Icon,
				TableId:         menu.TableId,
				LayoutId:        menu.LayoutId,
				ParentId:        menu.ParentId,
				Type:            menu.Type,
				MicrofrontendId: menu.MicrofrontendId,
				IsStatic:        menu.IsStatic,
			},
		)
		if err != nil {
			logReq.Response = err.Error()
			return respLogReq, err
		} else {
			logReq.Response = resp
			logReq.Current = resp
			respLogReq = append(respLogReq, &logReq)
		}
	}

	return respLogReq, nil
}
