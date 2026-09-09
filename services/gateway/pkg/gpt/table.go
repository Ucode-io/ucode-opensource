package gpt

import (
	"context"
	"fmt"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"

	"github.com/google/uuid"
	"github.com/spf13/cast"
)

func CreateTable(reqBody *models.CreateTableAI) ([]*models.CreateVersionHistoryRequest, error) {

	attributes, err := helper.ConvertMapToStruct(map[string]any{
		"label":    "",
		"label_en": reqBody.Label,
	})
	if err != nil {
		return nil, err
	}

	var (
		respLogReq = []*models.CreateVersionHistoryRequest{}
		resource   = reqBody.Resource
		services   = reqBody.Service
		tableMongo = &obs.CreateTableRequest{
			Label:      reqBody.Label,
			Slug:       reqBody.TableSlug,
			ShowInMenu: true,
			Name:       fmt.Sprintf("Auto Created Commit Create table - %s", time.Now().Format(time.RFC1123)),
			CommitType: config.COMMIT_TYPE_TABLE,
			OrderBy:    false,
			Attributes: attributes,
			EnvId:      reqBody.EnvironmentId,
			ViewId:     uuid.NewString(),
			LayoutId:   uuid.NewString(),
			ProjectId:  resource.ResourceEnvironmentId,
		}
		tablePsql = &nb.CreateTableRequest{
			Label:      reqBody.Label,
			Slug:       reqBody.TableSlug,
			ShowInMenu: true,
			Name:       fmt.Sprintf("Auto Created Commit Create table - %s", time.Now().Format(time.RFC1123)),
			CommitType: config.COMMIT_TYPE_TABLE,
			OrderBy:    false,
			Attributes: attributes,
			EnvId:      reqBody.EnvironmentId,
			ViewId:     uuid.NewString(),
			LayoutId:   uuid.NewString(),
			ProjectId:  resource.ResourceEnvironmentId,
		}
		logReq = models.CreateVersionHistoryRequest{
			Services:     reqBody.Service,
			NodeType:     reqBody.Resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: "TABLE",
			ActionType:   "CREATE TABLE",
			UserInfo:     cast.ToString(reqBody.UserId),
			Request:      tableMongo,
			TableSlug:    reqBody.TableSlug,
		}
	)

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(reqBody.Resource.NodeType).Table().Create(
			context.Background(),
			tableMongo,
		)
		if err != nil {
			logReq.Response = err.Error()
			respLogReq = append(respLogReq, &logReq)
			return respLogReq, err
		} else {
			tableMongo.Id = resp.Id
			logReq.Current = &tableMongo
			logReq.Response = &tableMongo
			respLogReq = append(respLogReq, &logReq)
		}

		if reqBody.Menu != "c57eedc3-a954-4262-a0af-376c65b5a284" {
			menus, err := services.GetBuilderServiceByType(resource.NodeType).Menu().GetByLabel(
				context.Background(),
				&obs.MenuPrimaryKey{Label: reqBody.Menu, ProjectId: reqBody.Resource.ResourceEnvironmentId},
			)
			if err != nil {
				return respLogReq, err
			}

			for _, menu := range menus.Menus {
				var (
					mongoRequest = &obs.CreateMenuRequest{
						Label:      reqBody.Label,
						ParentId:   menu.Id,
						Type:       "TABLE",
						ProjectId:  reqBody.Resource.ResourceEnvironmentId,
						Attributes: attributes,
						TableId:    resp.Id,
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

				resp, err := reqBody.Service.GetBuilderServiceByType(reqBody.Resource.NodeType).Menu().Create(
					context.Background(),
					mongoRequest,
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
		} else {
			var (
				mongoRequest = &obs.CreateMenuRequest{
					Label:      reqBody.Label,
					ParentId:   "c57eedc3-a954-4262-a0af-376c65b5a284",
					Type:       "TABLE",
					ProjectId:  reqBody.Resource.ResourceEnvironmentId,
					Attributes: attributes,
					TableId:    resp.Id,
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

			resp, err := reqBody.Service.GetBuilderServiceByType(reqBody.Resource.NodeType).Menu().Create(
				context.Background(),
				mongoRequest,
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

	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Table().Create(
			context.Background(),
			tablePsql,
		)
		if err != nil {
			logReq.Response = err.Error()
			respLogReq = append(respLogReq, &logReq)
			return respLogReq, err
		} else {
			tablePsql.Id = resp.Id
			logReq.Current = &tablePsql
			logReq.Response = &tablePsql
			respLogReq = append(respLogReq, &logReq)
		}
	}

	return respLogReq, nil
}

func UpdateTable(req *models.UpdateTableAI) ([]*models.CreateVersionHistoryRequest, error) {
	var (
		resource   = req.Resource
		services   = req.Service
		respLogReq = []*models.CreateVersionHistoryRequest{}
	)

	attributes, err := helper.ConvertMapToStruct(map[string]any{
		"label":    "",
		"label_en": req.NewLabel,
	})
	if err != nil {
		return respLogReq, err
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		tables, err := services.GetBuilderServiceByType(resource.NodeType).Table().GetTablesByLabel(context.Background(), &obs.GetTablesByLabelReq{
			ProjectId: resource.ResourceEnvironmentId,
			Label:     req.OldLabel,
		})
		if err != nil {
			return nil, err
		}

		for _, table := range tables.Tables {

			var (
				updateTable = &obs.UpdateTableRequest{}
				logReq      = models.CreateVersionHistoryRequest{
					Services:     services,
					NodeType:     resource.NodeType,
					ProjectId:    resource.ResourceEnvironmentId,
					ActionSource: "TABLE",
					ActionType:   "UPDATE TABLE",
					UserInfo:     cast.ToString(req.UserId),
					TableSlug:    table.Slug,
				}
			)

			table.Label = req.NewLabel

			err = helper.MarshalToStruct(table, &updateTable)
			if err != nil {
				return respLogReq, err
			}

			updateTable.ProjectId = resource.ResourceEnvironmentId
			updateTable.Attributes = attributes

			logReq.Request = &updateTable

			resp, err := services.GetBuilderServiceByType(resource.NodeType).Table().Update(
				context.Background(),
				updateTable,
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

		menus, err := services.GetBuilderServiceByType(resource.NodeType).Menu().GetByLabel(
			context.Background(),
			&obs.MenuPrimaryKey{Label: req.OldLabel, ProjectId: resource.ResourceEnvironmentId},
		)
		if err != nil {
			return respLogReq, err
		}

		for _, menu := range menus.Menus {
			var (
				updateMenu = &obs.Menu{
					ProjectId:       resource.ResourceEnvironmentId,
					Label:           req.NewLabel,
					Id:              menu.Id,
					Icon:            menu.Icon,
					TableId:         menu.TableId,
					LayoutId:        menu.LayoutId,
					ParentId:        menu.ParentId,
					Type:            menu.Type,
					MicrofrontendId: menu.MicrofrontendId,
					IsStatic:        menu.IsStatic,
				}
				logReq = models.CreateVersionHistoryRequest{
					Services:     services,
					NodeType:     resource.NodeType,
					ProjectId:    resource.ResourceEnvironmentId,
					ActionSource: "MENU",
					ActionType:   "UPDATE MENU",
					UserInfo:     cast.ToString(req.UserId),
					TableSlug:    "Menu",
					Request:      updateMenu,
				}
			)

			resp, err := services.GetBuilderServiceByType(resource.NodeType).Menu().Update(
				context.Background(),
				updateMenu,
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

	case pb.ResourceType_POSTGRESQL:
		tables, err := services.GoObjectBuilderService().Table().GetTablesByLabel(context.Background(), &nb.GetTablesByLabelReq{
			ProjectId: resource.ResourceEnvironmentId,
			Label:     req.OldLabel,
		})
		if err != nil {
			return nil, err
		}

		for _, table := range tables.Tables {
			var (
				updateTable = &nb.UpdateTableRequest{}
				logReq      = models.CreateVersionHistoryRequest{
					Services:     services,
					NodeType:     resource.NodeType,
					ProjectId:    resource.ResourceEnvironmentId,
					ActionSource: "TABLE",
					ActionType:   "UPDATE TABLE",
					UserInfo:     cast.ToString(req.UserId),
					TableSlug:    table.Slug,
				}
			)

			table.Label = req.NewLabel

			err = helper.MarshalToStruct(table, &updateTable)
			if err != nil {
				return respLogReq, err
			}

			updateTable.ProjectId = resource.ResourceEnvironmentId
			updateTable.Attributes = attributes

			logReq.Request = &updateTable

			resp, err := services.GoObjectBuilderService().Table().Update(
				context.Background(),
				updateTable,
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
	}

	return respLogReq, nil
}

func LoginTable(req *models.LoginTableAI) ([]*models.CreateVersionHistoryRequest, error) {

	var (
		resource    = req.Resource
		services    = req.Service
		respLogReq  = []*models.CreateVersionHistoryRequest{}
		updateTable = &obs.UpdateTableRequest{}
	)

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:

		fields, err := services.GetBuilderServiceByType(resource.NodeType).Table().GetFieldsByTable(
			context.Background(),
			&obs.GetFieldsByTableReq{
				TableLabel: req.Table,
				ProjectId:  resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			return respLogReq, err
		}

		authInfo := make(map[string]any)

		for _, field := range fields.Fields {
			if field.Slug == "client_type_id" {
				authInfo["client_type_id"] = "client_type_id"
			} else if field.Slug == "role_id" {
				authInfo["role_id"] = "role_id"
			} else if field.Label == req.Login {
				authInfo["login"] = field.Slug
			} else if field.Label == req.Password {
				authInfo["password"] = field.Slug
			}
		}

		_, ok := authInfo["client_type_id"]
		if !ok {
			return respLogReq, fmt.Errorf("your table doesn't have relation with client_type")
		}

		_, ok = authInfo["role_id"]
		if !ok {
			return respLogReq, fmt.Errorf("your table doesn't have relation with role")
		}

		authInfo["login_strategy"] = []string{"login"}

		table := fields.Table

		atrb, err := helper.ConvertStructToResponse(fields.Table.Attributes)
		if err != nil {
			return respLogReq, err
		}

		atrb["auth_info"] = authInfo

		table.Attributes, err = helper.ConvertMapToStruct(atrb)
		if err != nil {
			return respLogReq, err
		}

		table.IsLoginTable = true

		err = helper.MarshalToStruct(table, &updateTable)
		if err != nil {
			return respLogReq, err
		}

		updateTable.ProjectId = resource.ResourceEnvironmentId

		_, err = services.GetBuilderServiceByType(resource.NodeType).Table().Update(
			context.Background(),
			updateTable,
		)
		if err != nil {
			// logReq.Response = err.Error()
			// respLogReq = append(respLogReq, logReq)
			return respLogReq, err
		} else {
			// logReq.Response = resp
			// logReq.Current = resp
			// respLogReq = append(respLogReq, logReq)
		}

	case pb.ResourceType_POSTGRESQL:
	}

	return nil, nil
}
