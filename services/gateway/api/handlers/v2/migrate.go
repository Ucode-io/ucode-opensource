package v2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

type DataTableWrapper struct {
	Data *obs.CreateTableRequest
}

type DataTableWrapperPsql struct {
	Data *nb.CreateTableRequest
}

type DataUpdateTableWrapper struct {
	Data *obs.UpdateTableRequest
}

type DataUpdateTableWrapperPsql struct {
	Data *nb.UpdateTableRequest
}

type DataCreateFieldWrapper struct {
	Data *obs.CreateFieldRequest
}

type DataCreateFieldWrapperPsql struct {
	Data *nb.CreateFieldRequest
}

type DataFieldWrapper struct {
	Data *obs.Field
}

type DataFieldWrapperPsql struct {
	Data *nb.Field
}

type DataCreateRelationWrapper struct {
	Data *obs.CreateRelationRequest
}

type DataCreateRelationWrapperPsql struct {
	Data *nb.CreateRelationRequest
}

type DataRelationWrapper struct {
	Data *obs.RelationForGetAll
}

type DataRelationWrapperPsql struct {
	Data *nb.RelationForGetAll
}

type DataCreateMenuWrapper struct {
	Data *obs.CreateMenuRequest
}

type DataCreateMenuWrapperPsql struct {
	Data *nb.CreateMenuRequest
}

type DataUpdateMenuWrapper struct {
	Data *obs.Menu
}

type DataUpdateMenuWrapperPsql struct {
	Data *nb.Menu
}
type DataCreateViewWrapper struct {
	Data *obs.CreateViewRequest
}

type DataCreateViewWrapperPsql struct {
	Data *nb.CreateViewRequest
}

type DataUpdateViewWrapper struct {
	Data *obs.View
}

type DataUpdateViewWrapperPsql struct {
	Data *nb.View
}

type DataCreateLayoutWrapper struct {
	Data *obs.CreateLayoutRequest
}

type DataCreateLayoutWrapperPsql struct {
	Data *nb.CreateLayoutRequest
}

type DataUpdateLayoutWrapper struct {
	Data *obs.LayoutRequest
}

type DataUpdateLayoutWrapperPsql struct {
	Data *nb.LayoutRequest
}

type DataUpdateRelationWrapper struct {
	Data *obs.UpdateRelationRequest
}

type DataUpdateRelationWrapperPsql struct {
	Data *nb.UpdateRelationRequest
}

// MigrateUp godoc
// @Security ApiKeyAuth
// @ID migrate_up
// @Router /v2/version/history/migrate/up/{environment_id} [POST]
// @Summary Migrate up
// @Description Migrate up
// @Tags VersionHistory
// @Accept json
// @Produce json
// @Param environment_id path string true "environment_id"
// @Param migrate body models.MigrateUpRequest true "MigrateUpRequest"
// @Success 200 {object} status_http.Response{data=models.MigrateUpResponse} "Upbody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) MigrateUp(c *gin.Context) {
	var (
		resp models.MigrateUpResponse
		req  models.MigrateUpRequest
		ids  []string
	)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 300*time.Second)
	defer cancel()

	if err := c.ShouldBindJSON(&req); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	migrateRequest := req.Data

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	currentEnvironmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(currentEnvironmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "error getting environment id | not valid")
		return
	}

	environmentId := currentEnvironmentId.(string)

	userId, _ := c.Get("user_id")

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId,
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	for _, v := range migrateRequest {
		var (
			actionSource  = v.ActionSource
			actionType    = strings.Split(v.ActionType, " ")[0]
			nodeType      = resource.NodeType
			resourceEnvId = resource.ResourceEnvironmentId

			logReq = &models.CreateVersionHistoryRequest{
				Services:     services,
				NodeType:     nodeType,
				ProjectId:    resourceEnvId,
				ActionSource: v.ActionSource,
				ActionType:   v.ActionType,
				UserInfo:     cast.ToString(userId),
			}
		)

		if actionSource == "TABLE" {
			defer func() {
				go h.versionHistory(logReq)
			}()

			var (
				previous      DataTableWrapper
				current       DataTableWrapper
				currentUpdate DataUpdateTableWrapper
			)

			err := json.Unmarshal([]byte(cast.ToString(v.Request)), &current)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Current)), &currentUpdate)
			if err != nil {
				continue
			}

			current.Data.ProjectId = resourceEnvId
			current.Data.EnvId = cast.ToString(environmentId)
			currentUpdate.Data.ProjectId = resourceEnvId
			currentUpdate.Data.EnvId = cast.ToString(environmentId)
			logReq.Request = current.Data
			logReq.TableSlug = current.Data.Slug

			switch actionType {
			case "CREATE":
				_, err = services.GetBuilderServiceByType(nodeType).Table().Create(
					ctx,
					current.Data,
				)
				if err != nil {
					logReq.Response = err.Error()
					h.log.Error("!!!MigrationUp--->Error while creating table", logger.Error(err))
					continue
				}
				logReq.Current = current.Data
				logReq.Response = current.Data
				//ids = append(ids, v.Id)
			case "UPDATE":
				logReq.Previous = previous.Data

				_, err = services.GetBuilderServiceByType(nodeType).Table().Update(
					ctx,
					currentUpdate.Data,
				)
				if err != nil {
					logReq.Response = err.Error()
					h.log.Error("!!!MigrationUp--->Error while updating table", logger.Error(err))
					continue
				}
				logReq.Current = current.Data
				logReq.Response = current.Data
				//ids = append(ids, v.Id)
			case "DELETE":
				logReq.Previous = previous.Data
				_, err := services.GetBuilderServiceByType(nodeType).Table().Delete(
					ctx,
					&obs.TablePrimaryKey{
						Id:        previous.Data.Id,
						ProjectId: resourceEnvId,
						// AuthorId:   authInfo.GetUserId(),
						Name:       fmt.Sprintf("Auto Created Commit Delete table - %s", time.Now().Format(time.RFC1123)),
						CommitType: config.COMMIT_TYPE_TABLE,
						EnvId:      cast.ToString(environmentId),
					},
				)
				if err != nil {
					logReq.Response = err.Error()
					h.log.Error("!!!MigrationUp--->Error while deleting table", logger.Error(err))
					continue
				}
				//ids = append(ids, v.Id)
			}
		} else if actionSource == "FIELD" {
			defer func() {
				go h.versionHistory(logReq)
			}()

			var (
				current       DataCreateFieldWrapper
				previous      DataFieldWrapper
				currentUpdate DataFieldWrapper
				request       DataCreateFieldWrapper
			)

			err := json.Unmarshal([]byte(cast.ToString(v.Request)), &request)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Current)), &current)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Current)), &currentUpdate)
			if err != nil {
				continue
			}

			current.Data.ProjectId = resourceEnvId
			current.Data.EnvId = cast.ToString(environmentId)
			currentUpdate.Data.ProjectId = resourceEnvId
			currentUpdate.Data.EnvId = cast.ToString(environmentId)
			previous.Data.ProjectId = resourceEnvId
			previous.Data.EnvId = cast.ToString(environmentId)
			logReq.TableSlug = current.Data.TableId
			request.Data.ProjectId = resourceEnvId
			request.Data.EnvId = cast.ToString(environmentId)

			switch actionType {
			case "CREATE":
				createField, err := services.GetBuilderServiceByType(nodeType).Field().Create(
					ctx,
					request.Data,
				)
				if err != nil {
					logReq.Response = err.Error()
					h.log.Error("!!!MigrationUp--->Error while creating field", logger.Error(err))
					continue
				}
				logReq.Request = current.Data
				logReq.Current = current.Data
				logReq.Response = createField
				//ids = append(ids, v.Id)
			case "UPDATE":
				logReq.Previous = previous.Data

				updateField, err := services.GetBuilderServiceByType(nodeType).Field().Update(
					ctx,
					currentUpdate.Data,
				)
				if err != nil {
					logReq.Response = err.Error()
					h.log.Error("!!!MigrationUp--->Error while updating field", logger.Error(err))
					continue
				}
				logReq.Request = currentUpdate.Data
				logReq.Current = updateField
				logReq.Response = updateField
				//ids = append(ids, v.Id)
			case "DELETE":
				logReq.Previous = previous.Data
				_, err := services.GetBuilderServiceByType(nodeType).Field().Delete(
					ctx,
					&obs.FieldPrimaryKey{
						Id:        previous.Data.Id,
						ProjectId: resourceEnvId,
						EnvId:     cast.ToString(environmentId),
					},
				)
				if err != nil {
					logReq.Response = err.Error()
					h.log.Error("!!!MigrationUp--->Error while deleting field", logger.Error(err))
					continue
				}
				//ids = append(ids, v.Id)
			}
		} else if actionSource == "RELATION" {
			defer func() {
				go h.versionHistory(logReq)
			}()

			var (
				request  DataCreateRelationWrapper
				response DataCreateRelationWrapper
				current  DataUpdateRelationWrapper
				previous DataCreateRelationWrapper
			)

			err := json.Unmarshal([]byte(cast.ToString(v.Request)), &request)
			if err != nil {
				h.log.Error("error in request", logger.Error(err))
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
			if err != nil {
				h.log.Error("error in previous", logger.Error(err))
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Request)), &current)
			if err != nil {
				h.log.Error("error in current", logger.Error(err))
				continue
			}

			request.Data.ProjectId = resourceEnvId
			request.Data.EnvId = cast.ToString(environmentId)
			logReq.TableSlug = request.Data.RelationTableSlug
			current.Data.ProjectId = resourceEnvId
			current.Data.EnvId = cast.ToString(environmentId)

			switch actionType {
			case "CREATE":
				createRelation, err := services.GetBuilderServiceByType(nodeType).Relation().Create(
					ctx,
					request.Data,
				)
				if err != nil {
					logReq.Response = err.Error()
					h.log.Error("!!!MigrationUp--->Error while creating relation", logger.Error(err))
					continue
				}
				logReq.Request = request.Data
				logReq.Current = createRelation
				logReq.Response = createRelation
				//ids = append(ids, v.Id)
			case "UPDATE":
				logReq.Previous = previous.Data
				updateRelation, err := services.GetBuilderServiceByType(nodeType).Relation().Update(
					ctx,
					current.Data,
				)
				if err != nil {
					logReq.Response = err.Error()
					h.log.Error("!!!MigrationUp--->Error while updating relation", logger.Error(err))
					continue
				}
				logReq.Request = current.Data
				logReq.Current = updateRelation
				logReq.Response = updateRelation
				//ids = append(ids, v.Id)
			case "DELETE":
				logReq.Previous = response.Data
				_, err := services.GetBuilderServiceByType(nodeType).Field().Delete(
					ctx,
					&obs.FieldPrimaryKey{
						Id:        response.Data.Id,
						ProjectId: resourceEnvId,
						EnvId:     cast.ToString(environmentId),
					},
				)
				if err != nil {
					logReq.Response = err.Error()
					h.log.Error("!!!MigrationUp--->Error while deleting relation", logger.Error(err))
					continue
				}
				//ids = append(ids, v.Id)
			}
		} else if actionSource == "MENU" {
			defer func() {
				go h.versionHistory(logReq)
			}()

			var (
				request  DataCreateMenuWrapper
				previous DataCreateMenuWrapper
				current  DataUpdateMenuWrapper
			)

			err := json.Unmarshal([]byte(cast.ToString(v.Request)), &request)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Current)), &current)
			if err != nil {
				continue
			}

			request.Data.ProjectId = resourceEnvId
			request.Data.EnvId = cast.ToString(environmentId)
			previous.Data.ProjectId = resourceEnvId
			previous.Data.EnvId = cast.ToString(environmentId)
			current.Data.ProjectId = resourceEnvId
			current.Data.EnvId = cast.ToString(environmentId)
			request.Data.Id = current.Data.Id
			logReq.TableSlug = "Menu"

			switch actionType {
			case "CREATE":
				createMenu, err := services.GetBuilderServiceByType(nodeType).Menu().Create(
					ctx,
					request.Data,
				)
				if err != nil {
					logReq.Response = err.Error()
					h.log.Error("!!!MigrationUp--->Error while creating menu", logger.Error(err))
					continue
				}
				logReq.Request = request.Data
				logReq.Current = createMenu
				logReq.Response = createMenu
				//ids = append(ids, v.Id)
			case "UPDATE":
				logReq.Previous = previous.Data
				updatemenu, err := services.GetBuilderServiceByType(nodeType).Menu().Update(
					ctx,
					current.Data,
				)
				if err != nil {
					logReq.Response = err.Error()
					h.log.Error("!!!MigrationUp--->Error while updating menu", logger.Error(err))
					continue
				}
				logReq.Request = current.Data
				logReq.Current = updatemenu
				logReq.Response = updatemenu
				//ids = append(ids, v.Id)
			case "DELETE":
				logReq.Previous = previous.Data
				_, err = services.GetBuilderServiceByType(nodeType).Menu().Delete(
					ctx,
					&obs.MenuPrimaryKey{
						Id:        previous.Data.Id,
						ProjectId: resourceEnvId,
						EnvId:     cast.ToString(environmentId),
					},
				)
				if err != nil {
					logReq.Response = err.Error()
					h.log.Error("!!!MigrationUp--->Error while deleting menu", logger.Error(err))
					continue
				}
				//ids = append(ids, v.Id)
			}
		} else if actionSource == "VIEW" {
			defer func() {
				go h.versionHistory(logReq)
			}()

			var (
				request  DataCreateViewWrapper
				previous DataCreateViewWrapper
				current  DataUpdateViewWrapper
			)

			err := json.Unmarshal([]byte(cast.ToString(v.Request)), &request)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Request)), &current)
			if err != nil {
				continue
			}

			request.Data.ProjectId = resourceEnvId
			request.Data.EnvId = cast.ToString(environmentId)
			previous.Data.ProjectId = resourceEnvId
			previous.Data.EnvId = cast.ToString(environmentId)
			current.Data.ProjectId = resourceEnvId
			current.Data.EnvId = cast.ToString(environmentId)
			logReq.TableSlug = "View"

			switch actionType {
			case "CREATE":
				request.Data.Id = current.Data.Id
				createView, err := services.GetBuilderServiceByType(nodeType).View().Create(
					ctx,
					request.Data,
				)
				if err != nil {
					logReq.Response = err.Error()
					h.log.Error("!!!MigrationUp--->Error while creating view", logger.Error(err))
					continue
				}
				logReq.Request = request.Data
				logReq.Current = createView
				logReq.Response = createView
				//ids = append(ids, v.Id)
			case "UPDATE":
				logReq.Previous = previous.Data
				updateView, err := services.GetBuilderServiceByType(nodeType).View().Update(
					ctx,
					current.Data,
				)
				if err != nil {
					logReq.Response = err.Error()
					h.log.Error("!!!MigrationUp--->Error while updating view", logger.Error(err))
					continue
				}
				logReq.Request = current.Data
				logReq.Current = updateView
				logReq.Response = updateView
				//ids = append(ids, v.Id)
			case "DELETE":
				logReq.Previous = previous.Data
				_, err = services.GetBuilderServiceByType(nodeType).View().Delete(
					ctx,
					&obs.ViewPrimaryKey{
						Id:        previous.Data.Id,
						ProjectId: resourceEnvId,
						EnvId:     cast.ToString(environmentId),
					},
				)
				if err != nil {
					logReq.Response = err.Error()
					h.log.Error("!!!MigrationUp--->Error while deleting view", logger.Error(err))
					continue
				}
				//ids = append(ids, v.Id)
			}
		} else if actionSource == "LAYOUT" {
			defer func() {
				go h.versionHistory(logReq)
			}()

			var (
				previous DataUpdateLayoutWrapper
				current  DataUpdateLayoutWrapper
			)

			err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Request)), &current)
			if err != nil {
				continue
			}

			previous.Data.ProjectId = resourceEnvId
			previous.Data.EnvId = cast.ToString(environmentId)
			current.Data.ProjectId = resourceEnvId
			current.Data.EnvId = cast.ToString(environmentId)
			logReq.TableSlug = "Layout"

			switch actionType {
			case "UPDATE":
				logReq.Previous = previous.Data
				updateLayout, err := services.GetBuilderServiceByType(nodeType).Layout().Update(
					ctx,
					current.Data,
				)
				if err != nil {
					h.log.Error("!!!MigrationUp--->Error while updating layout", logger.Error(err))
					logReq.Response = err.Error()
					continue
				}
				logReq.Request = current.Data
				logReq.Current = updateLayout
				logReq.Response = updateLayout
				//ids = append(ids, v.Id)
			case "DELETE":
				logReq.Previous = previous.Data
				_, err = services.GetBuilderServiceByType(nodeType).Layout().RemoveLayout(
					ctx,
					&obs.LayoutPrimaryKey{
						Id:        previous.Data.Id,
						ProjectId: resourceEnvId,
						EnvId:     cast.ToString(environmentId),
					},
				)
				if err != nil {
					h.log.Error("!!!MigrationUp--->Error while deleting layout", logger.Error(err))
					logReq.Response = err.Error()
					continue
				}
				//ids = append(ids, v.Id)
			}
		}
	}

	resp.Ids = ids

	h.HandleResponse(c, status_http.OK, resp)
}

func (h *HandlerV2) MigrateDown(c *gin.Context) {
	var (
		ids  []string
		req  models.MigrateUpRequest
		resp models.MigrateUpResponse
	)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 300*time.Second)
	defer cancel()

	if err := c.ShouldBindJSON(&req); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	migrateRequest := req.Data

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	currentEnvironmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(currentEnvironmentId.(string)) {
		err := errors.New("error getting environment id | not valid")
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	environmentId := currentEnvironmentId.(string)

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId,
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(
		c.Request.Context(),
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	for i := len(migrateRequest) - 1; i >= 0; i-- {
		v := migrateRequest[i]
		var (
			actionSource  = v.ActionSource
			actionType    = strings.Split(v.ActionType, " ")[0]
			nodeType      = resource.NodeType
			resourceEnvId = resource.ResourceEnvironmentId
		)

		if actionSource == "TABLE" {
			var (
				current        DataTableWrapper
				previous       DataUpdateTableWrapper
				createPrevious DataTableWrapper
			)

			if cast.ToString(v.Current) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Current)), &current)
				if err != nil {
					continue
				}
				current.Data.ProjectId = resourceEnvId
				current.Data.EnvId = environmentId
			}

			if cast.ToString(v.Previous) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
				if err != nil {
					continue
				}
				previous.Data.ProjectId = resourceEnvId
				previous.Data.EnvId = environmentId
			}

			if cast.ToString(v.Previous) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &createPrevious)
				if err != nil {
					continue
				}
				createPrevious.Data.ProjectId = resourceEnvId
				createPrevious.Data.EnvId = environmentId
			}

			switch actionType {
			case "CREATE":
				_, err := services.GetBuilderServiceByType(nodeType).Table().Delete(
					ctx,
					&obs.TablePrimaryKey{
						Id:        current.Data.Id,
						ProjectId: resourceEnvId,
						EnvId:     cast.ToString(environmentId),
					},
				)
				if err != nil {
					continue
				}

				_, _ = services.GetBuilderServiceByType(nodeType).View().Delete(
					ctx,
					&obs.ViewPrimaryKey{
						TableSlug: current.Data.Slug,
						ProjectId: resourceEnvId,
						EnvId:     cast.ToString(environmentId),
					},
				)
				//ids = append(ids, v.Id)
			case "UPDATE":
				previous.Data.CommitType = "TABLE"
				previous.Data.Name = fmt.Sprintf("Auto Created Commit Create table - %s", time.Now().Format(time.RFC1123))
				_, err := services.GetBuilderServiceByType(nodeType).Table().Update(
					ctx,
					previous.Data,
				)
				if err != nil {
					continue
				}

				//ids = append(ids, v.Id)
			case "DELETE":
				createPrevious.Data.CommitType = "TABLE"
				createPrevious.Data.Name = fmt.Sprintf("Auto Created Commit Create table - %s", time.Now().Format(time.RFC1123))
				_, err := services.GetBuilderServiceByType(nodeType).Table().Create(
					ctx,
					createPrevious.Data,
				)
				if err != nil {
					continue
				}
				//ids = append(ids, v.Id)
			}
		} else if actionSource == "FIELD" {
			var (
				current        DataCreateFieldWrapper
				previous       DataFieldWrapper
				createPrevious DataCreateFieldWrapper
			)

			if cast.ToString(v.Current) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Current)), &current)
				if err != nil {
					continue
				}
				current.Data.ProjectId = resourceEnvId
				current.Data.EnvId = environmentId
			}

			if cast.ToString(v.Previous) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
				if err != nil {
					continue
				}
				previous.Data.ProjectId = resourceEnvId
				previous.Data.EnvId = environmentId
			}

			if cast.ToString(v.Previous) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &createPrevious)
				if err != nil {
					continue
				}
				createPrevious.Data.ProjectId = resourceEnvId
				createPrevious.Data.EnvId = environmentId
			}

			switch actionType {
			case "CREATE":
				_, err := services.GetBuilderServiceByType(nodeType).Field().Delete(
					ctx,
					&obs.FieldPrimaryKey{
						ProjectId: current.Data.ProjectId,
						EnvId:     current.Data.EnvId,
						Id:        current.Data.Id,
					},
				)
				if err != nil {
					continue
				}
				//ids = append(ids, v.Id)
			case "UPDATE":
				_, err := services.GetBuilderServiceByType(nodeType).Field().Update(
					ctx,
					previous.Data,
				)
				if err != nil {
					continue
				}
				//ids = append(ids, v.Id)
			case "DELETE":
				_, err := services.GetBuilderServiceByType(nodeType).Field().Create(
					ctx,
					createPrevious.Data,
				)
				if err != nil {
					continue
				}
				//ids = append(ids, v.Id)
			}
		} else if actionSource == "RELATION" {
			var (
				previous DataRelationWrapper
			)

			if cast.ToString(v.Previous) != "" {
				err := json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
				if err != nil {
					continue
				}
			}

			switch actionType {
			case "CREATE":
				var (
					current DataCreateRelationWrapper
				)

				if cast.ToString(v.Current) != "" {
					err := json.Unmarshal([]byte(cast.ToString(v.Current)), &current)
					if err != nil {
						continue
					}
				}

				_, err := services.GetBuilderServiceByType(nodeType).Relation().Delete(
					ctx,
					&obs.RelationPrimaryKey{
						Id:        current.Data.Id,
						ProjectId: resourceEnvId,
						EnvId:     cast.ToString(environmentId),
					},
				)
				if err != nil {
					continue
				}
				//ids = append(ids, v.Id)
			case "UPDATE":
				var (
					updateRelation = &obs.UpdateRelationRequest{
						Id:                     previous.Data.Id,
						TableFrom:              previous.Data.TableFrom.Slug,
						TableTo:                previous.Data.TableTo.Slug,
						Type:                   previous.Data.Type,
						AutoFilters:            previous.Data.AutoFilters,
						Summaries:              previous.Data.Summaries,
						Editable:               previous.Data.Editable,
						IsEditable:             previous.Data.IsEditable,
						Title:                  previous.Data.Title,
						Columns:                previous.Data.Columns,
						QuickFilters:           previous.Data.QuickFilters,
						GroupFields:            previous.Data.GroupFields,
						RelationTableSlug:      previous.Data.RelationTableSlug,
						ViewType:               previous.Data.ViewType,
						DynamicTables:          previous.Data.DynamicTables,
						RelationFieldSlug:      previous.Data.RelationFieldSlug,
						DefaultValues:          previous.Data.DefaultValues,
						IsUserIdDefault:        previous.Data.IsUserIdDefault,
						Cascadings:             previous.Data.Cascadings,
						ObjectIdFromJwt:        previous.Data.ObjectIdFromJwt,
						CascadingTreeTableSlug: previous.Data.CascadingTreeFieldSlug,
						CascadingTreeFieldSlug: previous.Data.CascadingTreeFieldSlug,
						ActionRelations:        previous.Data.ActionRelations,
						DefaultLimit:           previous.Data.DefaultLimit,
						MultipleInsert:         previous.Data.MultipleInsert,
						UpdatedFields:          previous.Data.UpdatedFields,
						MultipleInsertField:    previous.Data.MultipleInsertField,
						ProjectId:              resourceEnvId,
						Creatable:              previous.Data.Creatable,
						DefaultEditable:        previous.Data.DefaultEditable,
						FunctionPath:           previous.Data.FunctionPath,
						RelationButtons:        previous.Data.RelationButtons,
						Attributes:             previous.Data.Attributes,
						EnvId:                  environmentId,
					}
					viewFields = []string{}
				)

				for _, v := range previous.Data.ViewFields {
					viewFields = append(viewFields, v.Id)
				}

				updateRelation.ViewFields = viewFields

				_, err := services.GetBuilderServiceByType(nodeType).Relation().Update(
					ctx,
					updateRelation,
				)
				if err != nil {
					continue
				}
				//ids = append(ids, v.Id)
			case "DELETE":
				var (
					createRelation = &obs.CreateRelationRequest{
						Id:                     previous.Data.Id,
						TableFrom:              previous.Data.TableFrom.Slug,
						TableTo:                previous.Data.TableTo.Slug,
						Type:                   previous.Data.Type,
						AutoFilters:            previous.Data.AutoFilters,
						Summaries:              previous.Data.Summaries,
						Editable:               previous.Data.Editable,
						IsEditable:             previous.Data.IsEditable,
						Title:                  previous.Data.Title,
						Columns:                previous.Data.Columns,
						QuickFilters:           previous.Data.QuickFilters,
						GroupFields:            previous.Data.GroupFields,
						RelationTableSlug:      previous.Data.RelationTableSlug,
						ViewType:               previous.Data.ViewType,
						DynamicTables:          previous.Data.DynamicTables,
						RelationFieldSlug:      previous.Data.RelationFieldSlug,
						DefaultValues:          previous.Data.DefaultValues,
						IsUserIdDefault:        previous.Data.IsUserIdDefault,
						Cascadings:             previous.Data.Cascadings,
						ObjectIdFromJwt:        previous.Data.ObjectIdFromJwt,
						CascadingTreeTableSlug: previous.Data.CascadingTreeFieldSlug,
						CascadingTreeFieldSlug: previous.Data.CascadingTreeFieldSlug,
						ActionRelations:        previous.Data.ActionRelations,
						DefaultLimit:           previous.Data.DefaultLimit,
						MultipleInsert:         previous.Data.MultipleInsert,
						UpdatedFields:          previous.Data.UpdatedFields,
						MultipleInsertField:    previous.Data.MultipleInsertField,
						ProjectId:              resourceEnvId,
						Creatable:              previous.Data.Creatable,
						DefaultEditable:        previous.Data.DefaultEditable,
						FunctionPath:           previous.Data.FunctionPath,
						RelationButtons:        previous.Data.RelationButtons,
						Attributes:             previous.Data.Attributes,
						EnvId:                  environmentId,
					}
					viewFields = []string{}
				)
				for _, v := range previous.Data.ViewFields {
					viewFields = append(viewFields, v.Id)
				}
				createRelation.ViewFields = viewFields

				_, err := services.GetBuilderServiceByType(nodeType).Relation().Create(
					ctx,
					createRelation,
				)
				if err != nil {
					continue
				}
				//ids = append(ids, v.Id)
			}
		} else if actionSource == "MENU" {

			var (
				current       DataUpdateMenuWrapper
				previous      DataUpdateMenuWrapper
				createRequest DataCreateMenuWrapper
				response      DataUpdateMenuWrapper
			)

			if cast.ToString(v.Current) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Current)), &current)
				if err != nil {
					continue
				}
				current.Data.ProjectId = resourceEnvId
				current.Data.EnvId = environmentId
			}

			if cast.ToString(v.Previous) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
				if err != nil {
					continue
				}
				previous.Data.ProjectId = resourceEnvId
				previous.Data.EnvId = environmentId
			}

			if cast.ToString(v.Previous) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &createRequest)
				if err != nil {
					continue
				}
				createRequest.Data.ProjectId = resourceEnvId
				createRequest.Data.EnvId = environmentId
			}

			if cast.ToString(v.Response) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Response)), &response)
				if err != nil {
					continue
				}
				response.Data.ProjectId = resourceEnvId
				response.Data.EnvId = environmentId
			}

			switch actionType {
			case "CREATE":
				_, err = services.GetBuilderServiceByType(nodeType).Menu().Delete(
					ctx,
					&obs.MenuPrimaryKey{
						Id:        current.Data.Id,
						ProjectId: resourceEnvId,
						EnvId:     cast.ToString(environmentId),
					},
				)
				if err != nil {
					continue
				}
				//ids = append(ids, v.Id)
			case "UPDATE":
				_, err := services.GetBuilderServiceByType(nodeType).Menu().Update(
					ctx,
					previous.Data,
				)
				if err != nil {
					continue
				}
				//ids = append(ids, v.Id)
			case "DELETE":
				_, err := services.GetBuilderServiceByType(nodeType).Menu().Create(
					ctx,
					createRequest.Data,
				)
				if err != nil {
					continue
				}
				//ids = append(ids, v.Id)
			}
		} else if actionSource == "VIEW" {

			var (
				current        DataCreateViewWrapper
				previous       DataUpdateViewWrapper
				createPrevious DataCreateViewWrapper
			)

			if cast.ToString(v.Current) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Current)), &current)
				if err != nil {
					continue
				}
				current.Data.ProjectId = resourceEnvId
				current.Data.EnvId = environmentId
			}

			if cast.ToString(v.Current) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
				if err != nil {
					continue
				}
				previous.Data.ProjectId = resourceEnvId
				previous.Data.EnvId = environmentId
			}

			if cast.ToString(v.Previous) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &createPrevious)
				if err != nil {
					continue
				}
				createPrevious.Data.ProjectId = resourceEnvId
				createPrevious.Data.EnvId = environmentId
			}

			switch actionType {
			case "CREATE":
				_, err := services.GetBuilderServiceByType(nodeType).View().Delete(
					ctx,
					&obs.ViewPrimaryKey{
						Id:        current.Data.Id,
						ProjectId: resourceEnvId,
						EnvId:     environmentId,
					},
				)
				if err != nil {
					continue
				}
				//ids = append(ids, v.Id)
			case "UPDATE":
				_, err := services.GetBuilderServiceByType(nodeType).View().Update(
					ctx,
					previous.Data,
				)
				if err != nil {
					continue
				}
				//ids = append(ids, v.Id)
			case "DELETE":
				_, err := services.GetBuilderServiceByType(nodeType).View().Create(
					ctx,
					createPrevious.Data,
				)
				if err != nil {
					continue
				}
				//ids = append(ids, v.Id)
			}
		} else if actionSource == "LAYOUT" {
			var (
				previous DataUpdateLayoutWrapper
			)

			if cast.ToString(v.Previous) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
				if err != nil {
					continue
				}
				previous.Data.ProjectId = resourceEnvId
				previous.Data.EnvId = environmentId
			}

			switch actionType {
			case "DELETE", "UPDATE":
				_, err := services.GetBuilderServiceByType(nodeType).Layout().Update(
					ctx,
					previous.Data,
				)
				if err != nil {
					continue
				}
				//ids = append(ids, v.Id)
			}
		}
	}

	resp.Ids = ids

	h.HandleResponse(c, status_http.OK, resp)
}

func (h *HandlerV2) MigrateUpByVersion(c *gin.Context, services services.ServiceManagerI, lists *obs.ListVersionHistory, environmentId, nodeType, userId string, resourceType pb.ResourceType) error {
	listData, err := json.Marshal(lists.Histories)
	if err != nil {
		h.log.Error("!!!MigrateUpByVersions--->Error while marshalling list data", logger.Error(err))
		return err
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 300*time.Second)
	defer cancel()

	req := models.MigrateUpRequest{}

	if err = json.Unmarshal(listData, &req.Data); err != nil {
		h.log.Error("!!!MigrationUpByVersion--->Error while unmarshalling list data", logger.Error(err))
		return err
	}

	migrateRequest := req.Data

	for _, v := range migrateRequest {
		var (
			actionSource  = v.ActionSource
			actionType    = strings.Split(v.ActionType, " ")[0]
			nodeType      = nodeType
			resourceEnvId = environmentId

			logReq = &models.CreateVersionHistoryRequest{
				Services:     services,
				NodeType:     nodeType,
				ProjectId:    resourceEnvId,
				ActionSource: v.ActionSource,
				ActionType:   v.ActionType,
				UserInfo:     userId,
				VersionId:    v.VersionId,
			}
		)

		if actionSource == "TABLE" {
			defer func() {
				go h.versionHistory(logReq)
			}()

			var (
				previous          DataTableWrapper
				previousPsql      DataTableWrapperPsql
				current           DataTableWrapper
				currentPsql       DataTableWrapperPsql
				currentUpdate     DataUpdateTableWrapper
				currentUpdatePsql DataUpdateTableWrapperPsql
			)

			err := json.Unmarshal([]byte(cast.ToString(v.Request)), &current)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Request)), &currentPsql)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previousPsql)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Current)), &currentUpdate)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Current)), &currentUpdatePsql)
			if err != nil {
				continue
			}

			current.Data.ProjectId = resourceEnvId
			current.Data.EnvId = cast.ToString(environmentId)

			currentPsql.Data.ProjectId = resourceEnvId
			currentPsql.Data.EnvId = cast.ToString(environmentId)

			currentUpdate.Data.ProjectId = resourceEnvId
			currentUpdate.Data.EnvId = cast.ToString(environmentId)

			currentUpdatePsql.Data.ProjectId = resourceEnvId
			currentUpdatePsql.Data.EnvId = cast.ToString(environmentId)

			logReq.Request = current.Data
			logReq.TableSlug = current.Data.Slug

			switch actionType {
			case "CREATE":
				switch resourceType {
				case pb.ResourceType_MONGODB:
					_, err = services.GetBuilderServiceByType(nodeType).Table().Create(
						ctx,
						current.Data,
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while creating table", logger.Error(err))
						continue
					}

				case pb.ResourceType_POSTGRESQL:
					_, err = services.GoObjectBuilderService().Table().Create(
						ctx,
						currentPsql.Data,
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while creating table", logger.Error(err))
						continue
					}
				}

				logReq.Current = current.Data
				logReq.Response = current.Data
				//ids = append(ids, v.Id)
			case "UPDATE":
				logReq.Previous = previous.Data

				switch resourceType {
				case pb.ResourceType_MONGODB:
					_, err = services.GetBuilderServiceByType(nodeType).Table().Update(
						ctx,
						currentUpdate.Data,
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while updating table", logger.Error(err))
						continue
					}
				case pb.ResourceType_POSTGRESQL:
					_, err = services.GoObjectBuilderService().Table().Update(
						ctx,
						currentUpdatePsql.Data,
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while updating table", logger.Error(err))
						continue
					}
				}

				logReq.Current = current.Data
				logReq.Response = current.Data
				//ids = append(ids, v.Id)
			case "DELETE":
				logReq.Previous = previous.Data

				switch resourceType {
				case pb.ResourceType_MONGODB:
					_, err := services.GetBuilderServiceByType(nodeType).Table().Delete(
						ctx,
						&obs.TablePrimaryKey{
							Id:        previous.Data.Id,
							ProjectId: resourceEnvId,
							// AuthorId:   authInfo.GetUserId(),
							Name:       fmt.Sprintf("Auto Created Commit Delete table - %s", time.Now().Format(time.RFC1123)),
							CommitType: config.COMMIT_TYPE_TABLE,
							EnvId:      cast.ToString(environmentId),
						},
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while deleting table", logger.Error(err))
						continue
					}
				case pb.ResourceType_POSTGRESQL:
					_, err := services.GoObjectBuilderService().Table().Delete(
						ctx,
						&nb.TablePrimaryKey{
							Id:         previous.Data.Id,
							ProjectId:  resourceEnvId,
							Name:       fmt.Sprintf("Auto Created Commit Delete table - %s", time.Now().Format(time.RFC1123)),
							CommitType: config.COMMIT_TYPE_TABLE,
							EnvId:      cast.ToString(environmentId),
						},
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while deleting table", logger.Error(err))
						continue
					}
				}

				//ids = append(ids, v.Id)
			}
		} else if actionSource == "FIELD" {
			defer func() {
				go h.versionHistory(logReq)
			}()

			var (
				current           DataCreateFieldWrapper
				previous          DataFieldWrapper
				currentUpdate     DataFieldWrapper
				currentUpdatePsql DataFieldWrapperPsql
				request           DataCreateFieldWrapper
				requestPsql       DataCreateFieldWrapperPsql
			)

			err := json.Unmarshal([]byte(cast.ToString(v.Request)), &request)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Request)), &requestPsql)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Current)), &current)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Current)), &currentUpdate)
			if err != nil {
				continue
			}

			current.Data.ProjectId = resourceEnvId
			current.Data.EnvId = cast.ToString(environmentId)
			currentUpdate.Data.ProjectId = resourceEnvId
			currentUpdate.Data.EnvId = cast.ToString(environmentId)
			previous.Data.ProjectId = resourceEnvId
			previous.Data.EnvId = cast.ToString(environmentId)
			logReq.TableSlug = current.Data.TableId
			request.Data.ProjectId = resourceEnvId
			request.Data.EnvId = cast.ToString(environmentId)

			switch actionType {
			case "CREATE":
				switch resourceType {
				case pb.ResourceType_MONGODB:
					createField, err := services.GetBuilderServiceByType(nodeType).Field().Create(
						ctx,
						request.Data,
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while creating field", logger.Error(err))
						continue
					}
					logReq.Response = createField
				case pb.ResourceType_POSTGRESQL:
					createField, err := services.GoObjectBuilderService().Field().Create(
						ctx,
						requestPsql.Data,
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while creating field", logger.Error(err))
						continue
					}
					logReq.Response = createField
				}

				logReq.Request = current.Data
				logReq.Current = current.Data

				//ids = append(ids, v.Id)
			case "UPDATE":
				logReq.Previous = previous.Data

				switch resourceType {
				case pb.ResourceType_MONGODB:
					updateField, err := services.GetBuilderServiceByType(nodeType).Field().Update(
						ctx,
						currentUpdate.Data,
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while updating field", logger.Error(err))
						continue
					}

					logReq.Current = updateField
					logReq.Response = updateField
					logReq.Request = currentUpdate.Data
				case pb.ResourceType_POSTGRESQL:
					updateField, err := services.GoObjectBuilderService().Field().Update(
						ctx,
						currentUpdatePsql.Data,
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while updating field", logger.Error(err))
						continue
					}

					logReq.Current = updateField
					logReq.Response = updateField
					logReq.Request = currentUpdatePsql.Data
				}

				//ids = append(ids, v.Id)
			case "DELETE":
				logReq.Previous = previous.Data

				switch resourceType {
				case pb.ResourceType_MONGODB:
					_, err := services.GetBuilderServiceByType(nodeType).Field().Delete(
						ctx,
						&obs.FieldPrimaryKey{
							Id:        previous.Data.Id,
							ProjectId: resourceEnvId,
							EnvId:     cast.ToString(environmentId),
						},
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while deleting field", logger.Error(err))
						continue
					}
				case pb.ResourceType_POSTGRESQL:
					_, err := services.GoObjectBuilderService().Field().Delete(
						ctx,
						&nb.FieldPrimaryKey{
							Id:        previous.Data.Id,
							ProjectId: resourceEnvId,
							EnvId:     cast.ToString(environmentId),
						},
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while deleting field", logger.Error(err))
						continue
					}
				}

				//ids = append(ids, v.Id)
			}
		} else if actionSource == "RELATION" {
			defer func() {
				go h.versionHistory(logReq)
			}()

			var (
				request     DataCreateRelationWrapper
				requestPsql DataCreateRelationWrapperPsql
				response    DataCreateRelationWrapper
				current     DataUpdateRelationWrapper
				currentPsql DataUpdateRelationWrapperPsql
				previous    DataCreateRelationWrapper
			)

			err := json.Unmarshal([]byte(cast.ToString(v.Request)), &request)
			if err != nil {
				h.log.Error("!!!MigrationUp--->Error while unmarshalling request", logger.Error(err))
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Request)), &requestPsql)
			if err != nil {
				h.log.Error("!!!MigrationUp--->Error while unmarshalling request", logger.Error(err))
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
			if err != nil {
				h.log.Error("!!!MigrationUp--->Error while unmarshalling previous", logger.Error(err))
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Request)), &current)
			if err != nil {
				h.log.Error("!!!MigrationUp--->Error while unmarshalling current", logger.Error(err))
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Request)), &currentPsql)
			if err != nil {
				h.log.Error("!!!MigrationUp--->Error while unmarshalling current", logger.Error(err))
				continue
			}

			request.Data.ProjectId = resourceEnvId
			request.Data.EnvId = cast.ToString(environmentId)

			requestPsql.Data.ProjectId = resourceEnvId
			requestPsql.Data.EnvId = cast.ToString(environmentId)

			logReq.TableSlug = request.Data.RelationTableSlug

			current.Data.ProjectId = resourceEnvId
			current.Data.EnvId = cast.ToString(environmentId)

			currentPsql.Data.ProjectId = resourceEnvId
			currentPsql.Data.EnvId = cast.ToString(environmentId)

			switch actionType {
			case "CREATE":
				switch resourceType {
				case pb.ResourceType_MONGODB:
					createRelation, err := services.GetBuilderServiceByType(nodeType).Relation().Create(
						ctx,
						request.Data,
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while creating relation", logger.Error(err))
						continue
					}

					logReq.Current = createRelation
					logReq.Response = createRelation
				case pb.ResourceType_POSTGRESQL:
					createRelation, err := services.GoObjectBuilderService().Relation().Create(
						ctx,
						requestPsql.Data,
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while creating relation", logger.Error(err))
						continue
					}

					logReq.Current = createRelation
					logReq.Response = createRelation
				}

				logReq.Request = request.Data

				//ids = append(ids, v.Id)
			case "UPDATE":
				logReq.Previous = previous.Data

				switch resourceType {
				case pb.ResourceType_MONGODB:
					updateRelation, err := services.GetBuilderServiceByType(nodeType).Relation().Update(
						ctx,
						current.Data,
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while updating relation", logger.Error(err))
						continue
					}

					logReq.Current = updateRelation
					logReq.Response = updateRelation
				case pb.ResourceType_POSTGRESQL:
					updateRelation, err := services.GoObjectBuilderService().Relation().Update(
						ctx,
						currentPsql.Data,
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while updating relation", logger.Error(err))
						continue
					}

					logReq.Current = updateRelation
					logReq.Response = updateRelation
				}

				logReq.Request = current.Data

				//ids = append(ids, v.Id)
			case "DELETE":
				logReq.Previous = response.Data

				switch resourceType {
				case pb.ResourceType_MONGODB:
					_, err := services.GetBuilderServiceByType(nodeType).Field().Delete(
						ctx,
						&obs.FieldPrimaryKey{
							Id:        response.Data.Id,
							ProjectId: resourceEnvId,
							EnvId:     cast.ToString(environmentId),
						},
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while deleting relation", logger.Error(err))
						continue
					}
				case pb.ResourceType_POSTGRESQL:
					_, err := services.GoObjectBuilderService().Field().Delete(
						ctx,
						&nb.FieldPrimaryKey{
							Id:        response.Data.Id,
							ProjectId: resourceEnvId,
							EnvId:     cast.ToString(environmentId),
						},
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while deleting relation", logger.Error(err))
						continue
					}
				}

				//ids = append(ids, v.Id)
			}
		} else if actionSource == "MENU" {
			defer func() {
				go h.versionHistory(logReq)
			}()

			var (
				request     DataCreateMenuWrapper
				requestPsql DataCreateMenuWrapperPsql
				previous    DataCreateMenuWrapper
				current     DataUpdateMenuWrapper
				currentPsql DataUpdateMenuWrapperPsql
			)

			err := json.Unmarshal([]byte(cast.ToString(v.Request)), &request)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Request)), &requestPsql)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Current)), &current)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Current)), &currentPsql)
			if err != nil {
				continue
			}

			request.Data.ProjectId = resourceEnvId
			request.Data.EnvId = cast.ToString(environmentId)

			previous.Data.ProjectId = resourceEnvId
			previous.Data.EnvId = cast.ToString(environmentId)
			current.Data.ProjectId = resourceEnvId
			current.Data.EnvId = cast.ToString(environmentId)
			request.Data.Id = current.Data.Id
			logReq.TableSlug = "Menu"

			switch actionType {
			case "CREATE":
				switch resourceType {
				case pb.ResourceType_MONGODB:
					createMenu, err := services.GetBuilderServiceByType(nodeType).Menu().Create(
						ctx,
						request.Data,
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while creating menu", logger.Error(err))
						continue
					}

					logReq.Current = createMenu
					logReq.Response = createMenu
				case pb.ResourceType_POSTGRESQL:
					createMenu, err := services.GoObjectBuilderService().Menu().Create(
						ctx,
						requestPsql.Data,
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while creating menu", logger.Error(err))
						continue
					}

					logReq.Current = createMenu
					logReq.Response = createMenu
				}

				logReq.Request = request.Data
				//ids = append(ids, v.Id)
			case "UPDATE":
				logReq.Previous = previous.Data
				switch resourceType {
				case pb.ResourceType_MONGODB:
					updatemenu, err := services.GetBuilderServiceByType(nodeType).Menu().Update(
						ctx,
						current.Data,
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while updating menu", logger.Error(err))
						continue
					}
					logReq.Current = updatemenu
					logReq.Response = updatemenu
				case pb.ResourceType_POSTGRESQL:
					updatemenu, err := services.GoObjectBuilderService().Menu().Update(
						ctx,
						currentPsql.Data,
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while updating menu", logger.Error(err))
						continue
					}
					logReq.Current = updatemenu
					logReq.Response = updatemenu
				}

				logReq.Request = current.Data
				//ids = append(ids, v.Id)
			case "DELETE":
				logReq.Previous = previous.Data

				switch resourceType {
				case pb.ResourceType_MONGODB:
					_, err = services.GetBuilderServiceByType(nodeType).Menu().Delete(
						ctx,
						&obs.MenuPrimaryKey{
							Id:        previous.Data.Id,
							ProjectId: resourceEnvId,
							EnvId:     cast.ToString(environmentId),
						},
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while deleting menu", logger.Error(err))
						continue
					}
				case pb.ResourceType_POSTGRESQL:
					_, err = services.GoObjectBuilderService().Menu().Delete(
						ctx,
						&nb.MenuPrimaryKey{
							Id:        previous.Data.Id,
							ProjectId: resourceEnvId,
							EnvId:     cast.ToString(environmentId),
						},
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while deleting menu", logger.Error(err))
						continue
					}
				}

				//ids = append(ids, v.Id)
			}
		} else if actionSource == "VIEW" {
			defer func() {
				go h.versionHistory(logReq)
			}()

			var (
				request     DataCreateViewWrapper
				requestPsql DataCreateViewWrapperPsql
				previous    DataCreateViewWrapper
				current     DataUpdateViewWrapper
				currentPsql DataUpdateViewWrapperPsql
			)

			err := json.Unmarshal([]byte(cast.ToString(v.Request)), &request)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Request)), &requestPsql)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Request)), &current)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Request)), &currentPsql)
			if err != nil {
				continue
			}

			request.Data.ProjectId = resourceEnvId
			request.Data.EnvId = cast.ToString(environmentId)
			previous.Data.ProjectId = resourceEnvId
			previous.Data.EnvId = cast.ToString(environmentId)
			current.Data.ProjectId = resourceEnvId
			current.Data.EnvId = cast.ToString(environmentId)
			logReq.TableSlug = "View"

			switch actionType {
			case "CREATE":

				switch resourceType {
				case pb.ResourceType_MONGODB:
					request.Data.Id = current.Data.Id

					createView, err := services.GetBuilderServiceByType(nodeType).View().Create(
						ctx,
						request.Data,
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while creating view", logger.Error(err))
						continue
					}

					logReq.Current = createView
					logReq.Response = createView
					logReq.Request = request.Data
				case pb.ResourceType_POSTGRESQL:
					requestPsql.Data.Id = current.Data.Id

					createView, err := services.GoObjectBuilderService().View().Create(
						ctx,
						requestPsql.Data,
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while creating view", logger.Error(err))
						continue
					}

					logReq.Current = createView
					logReq.Response = createView
					logReq.Request = requestPsql.Data
				}

				//ids = append(ids, v.Id)
			case "UPDATE":
				logReq.Previous = previous.Data
				switch resourceType {
				case pb.ResourceType_MONGODB:
					updateView, err := services.GetBuilderServiceByType(nodeType).View().Update(
						ctx,
						current.Data,
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while updating view", logger.Error(err))
						continue
					}

					logReq.Current = updateView
					logReq.Response = updateView
				case pb.ResourceType_POSTGRESQL:
					updateView, err := services.GoObjectBuilderService().View().Update(
						ctx,
						currentPsql.Data,
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while updating view", logger.Error(err))
						continue
					}

					logReq.Current = updateView
					logReq.Response = updateView
				}

				logReq.Request = current.Data

				//ids = append(ids, v.Id)
			case "DELETE":
				logReq.Previous = previous.Data

				switch resourceType {
				case pb.ResourceType_MONGODB:
					_, err = services.GetBuilderServiceByType(nodeType).View().Delete(
						ctx,
						&obs.ViewPrimaryKey{
							Id:        previous.Data.Id,
							ProjectId: resourceEnvId,
							EnvId:     cast.ToString(environmentId),
						},
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while deleting view", logger.Error(err))
						continue
					}
				case pb.ResourceType_POSTGRESQL:
					_, err = services.GoObjectBuilderService().View().Delete(
						ctx,
						&nb.ViewPrimaryKey{
							Id:        previous.Data.Id,
							ProjectId: resourceEnvId,
							EnvId:     cast.ToString(environmentId),
						},
					)
					if err != nil {
						logReq.Response = err.Error()
						h.log.Error("!!!MigrationUp--->Error while deleting view", logger.Error(err))
						continue
					}
				}

				//ids = append(ids, v.Id)
			}
		} else if actionSource == "LAYOUT" {
			defer func() {
				go h.versionHistory(logReq)
			}()

			var (
				previous    DataUpdateLayoutWrapper
				current     DataUpdateLayoutWrapper
				currentPsql DataUpdateLayoutWrapperPsql
			)

			err := json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Request)), &current)
			if err != nil {
				continue
			}

			err = json.Unmarshal([]byte(cast.ToString(v.Request)), &currentPsql)
			if err != nil {
				continue
			}

			previous.Data.ProjectId = resourceEnvId
			previous.Data.EnvId = cast.ToString(environmentId)
			current.Data.ProjectId = resourceEnvId
			current.Data.EnvId = cast.ToString(environmentId)
			currentPsql.Data.ProjectId = resourceEnvId
			currentPsql.Data.EnvId = cast.ToString(environmentId)
			logReq.TableSlug = "Layout"

			switch actionType {
			case "UPDATE":

				switch resourceType {
				case pb.ResourceType_MONGODB:
					updateLayout, err := services.GetBuilderServiceByType(nodeType).Layout().Update(
						ctx,
						current.Data,
					)
					if err != nil {
						h.log.Error("!!!MigrationUp--->Error while updating layout", logger.Error(err))
						logReq.Response = err.Error()
						continue
					}

					logReq.Current = updateLayout
					logReq.Response = updateLayout
					logReq.Request = current.Data
				case pb.ResourceType_POSTGRESQL:
					updateLayout, err := services.GoObjectBuilderService().Layout().Update(
						ctx,
						currentPsql.Data,
					)
					if err != nil {
						h.log.Error("!!!MigrationUp--->Error while updating layout", logger.Error(err))
						logReq.Response = err.Error()
						continue
					}

					logReq.Current = updateLayout
					logReq.Response = updateLayout
					logReq.Request = currentPsql.Data
				}

				logReq.Previous = previous.Data

				//ids = append(ids, v.Id)
			case "DELETE":
				logReq.Previous = previous.Data

				switch resourceType {
				case pb.ResourceType_MONGODB:
					_, err = services.GetBuilderServiceByType(nodeType).Layout().RemoveLayout(
						ctx,
						&obs.LayoutPrimaryKey{
							Id:        previous.Data.Id,
							ProjectId: resourceEnvId,
							EnvId:     cast.ToString(environmentId),
						},
					)
					if err != nil {
						h.log.Error("!!!MigrationUp--->Error while deleting layout", logger.Error(err))
						logReq.Response = err.Error()
						continue
					}
				case pb.ResourceType_POSTGRESQL:
					_, err = services.GoObjectBuilderService().Layout().RemoveLayout(
						ctx,
						&nb.LayoutPrimaryKey{
							Id:        previous.Data.Id,
							ProjectId: resourceEnvId,
							EnvId:     cast.ToString(environmentId),
						},
					)
					if err != nil {
						h.log.Error("!!!MigrationUp--->Error while deleting layout", logger.Error(err))
						logReq.Response = err.Error()
						continue
					}
				}
			}
		}
	}

	return nil
}

func (h *HandlerV2) MigrateDownByVersion(c *gin.Context, services services.ServiceManagerI, lists *obs.ListVersionHistory, environmentId, nodeType, userId string, resourceType pb.ResourceType) error {
	var (
		err error
	)

	listData, err := json.Marshal(lists.Histories)
	if err != nil {
		h.log.Error("!!!MigrateDownByVersions--->Error while marshalling list data", logger.Error(err))
		return err
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 300*time.Second)
	defer cancel()

	req := models.MigrateUpRequest{}

	err = json.Unmarshal(listData, &req.Data)
	if err != nil {
		h.log.Error("!!!MigrationDownByVersion--->Error while unmarshalling list data", logger.Error(err))
		return err
	}

	migrateRequest := req.Data

	for i := len(migrateRequest) - 1; i >= 0; i-- {
		v := migrateRequest[i]
		var (
			actionSource  = v.ActionSource
			actionType    = strings.Split(v.ActionType, " ")[0]
			nodeType      = nodeType
			resourceEnvId = environmentId
		)

		if actionSource == "TABLE" {
			var (
				current            DataTableWrapper
				currentPsql        DataTableWrapperPsql
				previous           DataUpdateTableWrapper
				previousPsql       DataUpdateTableWrapperPsql
				createPrevious     DataTableWrapper
				createPreviousPsql DataTableWrapperPsql
			)

			if cast.ToString(v.Current) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Current)), &current)
				if err != nil {
					continue
				}
				current.Data.ProjectId = resourceEnvId
				current.Data.EnvId = environmentId

				err = json.Unmarshal([]byte(cast.ToString(v.Current)), &currentPsql)
				if err != nil {
					continue
				}

				currentPsql.Data.ProjectId = resourceEnvId
				currentPsql.Data.EnvId = environmentId
			}

			if cast.ToString(v.Previous) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
				if err != nil {
					continue
				}
				previous.Data.ProjectId = resourceEnvId
				previous.Data.EnvId = environmentId

				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previousPsql)
				if err != nil {
					continue
				}

				previousPsql.Data.ProjectId = resourceEnvId
				previousPsql.Data.EnvId = environmentId
			}

			if cast.ToString(v.Previous) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &createPrevious)
				if err != nil {
					continue
				}
				createPrevious.Data.ProjectId = resourceEnvId
				createPrevious.Data.EnvId = environmentId

				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &createPreviousPsql)
				if err != nil {
					continue
				}

				createPreviousPsql.Data.ProjectId = resourceEnvId
				createPreviousPsql.Data.EnvId = environmentId
			}

			switch actionType {
			case "CREATE":
				switch resourceType {
				case pb.ResourceType_MONGODB:
					_, err := services.GetBuilderServiceByType(nodeType).Table().Delete(
						ctx,
						&obs.TablePrimaryKey{
							Id:        current.Data.Id,
							ProjectId: resourceEnvId,
							EnvId:     cast.ToString(environmentId),
						},
					)
					if err != nil {
						continue
					}

					_, _ = services.GetBuilderServiceByType(nodeType).View().Delete(
						ctx,
						&obs.ViewPrimaryKey{
							TableSlug: current.Data.Slug,
							ProjectId: resourceEnvId,
							EnvId:     cast.ToString(environmentId),
						},
					)
					// //ids = append(ids, v.Id)
				case pb.ResourceType_POSTGRESQL:
					_, err := services.GoObjectBuilderService().Table().Delete(
						ctx,
						&nb.TablePrimaryKey{
							Id:        currentPsql.Data.Id,
							ProjectId: resourceEnvId,
							EnvId:     cast.ToString(environmentId),
						},
					)
					if err != nil {
						continue
					}

					_, _ = services.GoObjectBuilderService().View().Delete(
						ctx,
						&nb.ViewPrimaryKey{
							TableSlug: currentPsql.Data.Slug,
							ProjectId: resourceEnvId,
							EnvId:     cast.ToString(environmentId),
						},
					)
					//ids = append(ids, v.Id)
				}
			case "UPDATE":
				switch resourceType {
				case pb.ResourceType_MONGODB:
					previous.Data.CommitType = "TABLE"
					previous.Data.Name = fmt.Sprintf("Auto Created Commit Create table - %s", time.Now().Format(time.RFC1123))
					_, err := services.GetBuilderServiceByType(nodeType).Table().Update(
						ctx,
						previous.Data,
					)
					if err != nil {
						continue
					}

					//ids = append(ids, v.Id)
				case pb.ResourceType_POSTGRESQL:
					previousPsql.Data.CommitType = "TABLE"
					previousPsql.Data.Name = fmt.Sprintf("Auto Created Commit Create table - %s", time.Now().Format(time.RFC1123))
					_, err := services.GoObjectBuilderService().Table().Update(
						ctx,
						previousPsql.Data,
					)
					if err != nil {
						continue
					}

					//ids = append(ids, v.Id)
				}
			case "DELETE":
				switch resourceType {
				case pb.ResourceType_MONGODB:
					createPrevious.Data.CommitType = "TABLE"
					createPrevious.Data.Name = fmt.Sprintf("Auto Created Commit Create table - %s", time.Now().Format(time.RFC1123))
					_, err := services.GetBuilderServiceByType(nodeType).Table().Create(
						ctx,
						createPrevious.Data,
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				case pb.ResourceType_POSTGRESQL:
					createPreviousPsql.Data.CommitType = "TABLE"
					createPreviousPsql.Data.Name = fmt.Sprintf("Auto Created Commit Create table - %s", time.Now().Format(time.RFC1123))
					_, err := services.GoObjectBuilderService().Table().Create(
						ctx,
						createPreviousPsql.Data,
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				}
			}
		} else if actionSource == "FIELD" {
			var (
				current            DataCreateFieldWrapper
				currentPsql        DataCreateFieldWrapperPsql
				previous           DataFieldWrapper
				previousPsql       DataFieldWrapperPsql
				createPrevious     DataCreateFieldWrapper
				createPreviousPsql DataCreateFieldWrapperPsql
			)

			if cast.ToString(v.Current) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Current)), &current)
				if err != nil {
					continue
				}
				current.Data.ProjectId = resourceEnvId
				current.Data.EnvId = environmentId

				err = json.Unmarshal([]byte(cast.ToString(v.Current)), &currentPsql)
				if err != nil {
					continue
				}
				currentPsql.Data.ProjectId = resourceEnvId
				currentPsql.Data.EnvId = environmentId
			}

			if cast.ToString(v.Previous) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
				if err != nil {
					continue
				}
				previous.Data.ProjectId = resourceEnvId
				previous.Data.EnvId = environmentId

				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previousPsql)
				if err != nil {
					continue
				}
				previousPsql.Data.ProjectId = resourceEnvId
				previousPsql.Data.EnvId = environmentId
			}

			if cast.ToString(v.Previous) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &createPrevious)
				if err != nil {
					continue
				}
				createPrevious.Data.ProjectId = resourceEnvId
				createPrevious.Data.EnvId = environmentId

				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &createPreviousPsql)
				if err != nil {
					continue
				}
				createPreviousPsql.Data.ProjectId = resourceEnvId
				createPreviousPsql.Data.EnvId = environmentId
			}

			switch actionType {
			case "CREATE":
				switch resourceType {
				case pb.ResourceType_MONGODB:
					_, err := services.GetBuilderServiceByType(nodeType).Field().Delete(
						ctx,
						&obs.FieldPrimaryKey{
							ProjectId: current.Data.ProjectId,
							EnvId:     current.Data.EnvId,
							Id:        current.Data.Id,
						},
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				case pb.ResourceType_POSTGRESQL:
					_, err := services.GoObjectBuilderService().Field().Delete(
						ctx,
						&nb.FieldPrimaryKey{
							ProjectId: currentPsql.Data.ProjectId,
							EnvId:     currentPsql.Data.EnvId,
							Id:        currentPsql.Data.Id,
						},
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				}
			case "UPDATE":
				switch resourceType {
				case pb.ResourceType_MONGODB:
					_, err := services.GetBuilderServiceByType(nodeType).Field().Update(
						ctx,
						previous.Data,
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				case pb.ResourceType_POSTGRESQL:
					_, err := services.GoObjectBuilderService().Field().Update(
						ctx,
						previousPsql.Data,
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				}
			case "DELETE":
				switch resourceType {
				case pb.ResourceType_MONGODB:
					_, err := services.GetBuilderServiceByType(nodeType).Field().Create(
						ctx,
						createPrevious.Data,
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				case pb.ResourceType_POSTGRESQL:
					_, err := services.GoObjectBuilderService().Field().Create(
						ctx,
						createPreviousPsql.Data,
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				}
			}
		} else if actionSource == "RELATION" {
			var (
				previous     DataRelationWrapper
				previousPsql DataRelationWrapperPsql
			)

			if cast.ToString(v.Previous) != "" {
				err := json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
				if err != nil {
					continue
				}

				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previousPsql)
				if err != nil {
					continue
				}
			}

			switch actionType {
			case "CREATE":
				switch resourceType {
				case pb.ResourceType_MONGODB:
					var (
						current DataCreateRelationWrapper
					)

					if cast.ToString(v.Current) != "" {
						err := json.Unmarshal([]byte(cast.ToString(v.Current)), &current)
						if err != nil {
							continue
						}
					}

					_, err := services.GetBuilderServiceByType(nodeType).Relation().Delete(
						ctx,
						&obs.RelationPrimaryKey{
							Id:        current.Data.Id,
							ProjectId: resourceEnvId,
							EnvId:     cast.ToString(environmentId),
						},
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				case pb.ResourceType_POSTGRESQL:
					var (
						currentPsql DataCreateRelationWrapperPsql
					)

					if cast.ToString(v.Current) != "" {
						err := json.Unmarshal([]byte(cast.ToString(v.Current)), &currentPsql)
						if err != nil {
							continue
						}
					}

					_, err := services.GoObjectBuilderService().Relation().Delete(
						ctx,
						&nb.RelationPrimaryKey{
							Id:        currentPsql.Data.Id,
							ProjectId: resourceEnvId,
							EnvId:     cast.ToString(environmentId),
						},
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				}
			case "UPDATE":
				switch resourceType {
				case pb.ResourceType_MONGODB:
					var (
						updateRelation = &obs.UpdateRelationRequest{
							Id:                     previous.Data.Id,
							TableFrom:              previous.Data.TableFrom.Slug,
							TableTo:                previous.Data.TableTo.Slug,
							Type:                   previous.Data.Type,
							AutoFilters:            previous.Data.AutoFilters,
							Summaries:              previous.Data.Summaries,
							Editable:               previous.Data.Editable,
							IsEditable:             previous.Data.IsEditable,
							Title:                  previous.Data.Title,
							Columns:                previous.Data.Columns,
							QuickFilters:           previous.Data.QuickFilters,
							GroupFields:            previous.Data.GroupFields,
							RelationTableSlug:      previous.Data.RelationTableSlug,
							ViewType:               previous.Data.ViewType,
							DynamicTables:          previous.Data.DynamicTables,
							RelationFieldSlug:      previous.Data.RelationFieldSlug,
							DefaultValues:          previous.Data.DefaultValues,
							IsUserIdDefault:        previous.Data.IsUserIdDefault,
							Cascadings:             previous.Data.Cascadings,
							ObjectIdFromJwt:        previous.Data.ObjectIdFromJwt,
							CascadingTreeTableSlug: previous.Data.CascadingTreeFieldSlug,
							CascadingTreeFieldSlug: previous.Data.CascadingTreeFieldSlug,
							ActionRelations:        previous.Data.ActionRelations,
							DefaultLimit:           previous.Data.DefaultLimit,
							MultipleInsert:         previous.Data.MultipleInsert,
							UpdatedFields:          previous.Data.UpdatedFields,
							MultipleInsertField:    previous.Data.MultipleInsertField,
							ProjectId:              resourceEnvId,
							Creatable:              previous.Data.Creatable,
							DefaultEditable:        previous.Data.DefaultEditable,
							FunctionPath:           previous.Data.FunctionPath,
							RelationButtons:        previous.Data.RelationButtons,
							Attributes:             previous.Data.Attributes,
							EnvId:                  environmentId,
						}
						viewFields = []string{}
					)

					for _, v := range previous.Data.ViewFields {
						viewFields = append(viewFields, v.Id)
					}

					updateRelation.ViewFields = viewFields

					_, err := services.GetBuilderServiceByType(nodeType).Relation().Update(
						ctx,
						updateRelation,
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				case pb.ResourceType_POSTGRESQL:
					var (
						updateRelationPsql = &nb.UpdateRelationRequest{
							Id:                     previousPsql.Data.Id,
							TableFrom:              previousPsql.Data.TableFrom.Slug,
							TableTo:                previousPsql.Data.TableTo.Slug,
							Type:                   previousPsql.Data.Type,
							AutoFilters:            previousPsql.Data.AutoFilters,
							Summaries:              previousPsql.Data.Summaries,
							Editable:               previousPsql.Data.Editable,
							IsEditable:             previousPsql.Data.IsEditable,
							Title:                  previousPsql.Data.Title,
							Columns:                previousPsql.Data.Columns,
							QuickFilters:           previousPsql.Data.QuickFilters,
							GroupFields:            previousPsql.Data.GroupFields,
							RelationTableSlug:      previousPsql.Data.RelationTableSlug,
							ViewType:               previousPsql.Data.ViewType,
							DynamicTables:          previousPsql.Data.DynamicTables,
							RelationFieldSlug:      previousPsql.Data.RelationFieldSlug,
							DefaultValues:          previousPsql.Data.DefaultValues,
							IsUserIdDefault:        previousPsql.Data.IsUserIdDefault,
							Cascadings:             previousPsql.Data.Cascadings,
							ObjectIdFromJwt:        previousPsql.Data.ObjectIdFromJwt,
							CascadingTreeTableSlug: previousPsql.Data.CascadingTreeFieldSlug,
							CascadingTreeFieldSlug: previousPsql.Data.CascadingTreeFieldSlug,
							ActionRelations:        previousPsql.Data.ActionRelations,
							DefaultLimit:           previousPsql.Data.DefaultLimit,
							MultipleInsert:         previousPsql.Data.MultipleInsert,
							UpdatedFields:          previousPsql.Data.UpdatedFields,
							MultipleInsertField:    previousPsql.Data.MultipleInsertField,
							ProjectId:              resourceEnvId,
							Creatable:              previousPsql.Data.Creatable,
							DefaultEditable:        previousPsql.Data.DefaultEditable,
							FunctionPath:           previousPsql.Data.FunctionPath,
							RelationButtons:        previousPsql.Data.RelationButtons,
							Attributes:             previousPsql.Data.Attributes,
							EnvId:                  environmentId,
						}
						viewFields = []string{}
					)

					for _, v := range previousPsql.Data.ViewFields {
						viewFields = append(viewFields, v.Id)
					}

					updateRelationPsql.ViewFields = viewFields

					_, err := services.GoObjectBuilderService().Relation().Update(
						ctx,
						updateRelationPsql,
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				}
			case "DELETE":
				switch resourceType {
				case pb.ResourceType_MONGODB:
					var (
						createRelation = &obs.CreateRelationRequest{
							Id:                     previous.Data.Id,
							TableFrom:              previous.Data.TableFrom.Slug,
							TableTo:                previous.Data.TableTo.Slug,
							Type:                   previous.Data.Type,
							AutoFilters:            previous.Data.AutoFilters,
							Summaries:              previous.Data.Summaries,
							Editable:               previous.Data.Editable,
							IsEditable:             previous.Data.IsEditable,
							Title:                  previous.Data.Title,
							Columns:                previous.Data.Columns,
							QuickFilters:           previous.Data.QuickFilters,
							GroupFields:            previous.Data.GroupFields,
							RelationTableSlug:      previous.Data.RelationTableSlug,
							ViewType:               previous.Data.ViewType,
							DynamicTables:          previous.Data.DynamicTables,
							RelationFieldSlug:      previous.Data.RelationFieldSlug,
							DefaultValues:          previous.Data.DefaultValues,
							IsUserIdDefault:        previous.Data.IsUserIdDefault,
							Cascadings:             previous.Data.Cascadings,
							ObjectIdFromJwt:        previous.Data.ObjectIdFromJwt,
							CascadingTreeTableSlug: previous.Data.CascadingTreeFieldSlug,
							CascadingTreeFieldSlug: previous.Data.CascadingTreeFieldSlug,
							ActionRelations:        previous.Data.ActionRelations,
							DefaultLimit:           previous.Data.DefaultLimit,
							MultipleInsert:         previous.Data.MultipleInsert,
							UpdatedFields:          previous.Data.UpdatedFields,
							MultipleInsertField:    previous.Data.MultipleInsertField,
							ProjectId:              resourceEnvId,
							Creatable:              previous.Data.Creatable,
							DefaultEditable:        previous.Data.DefaultEditable,
							FunctionPath:           previous.Data.FunctionPath,
							RelationButtons:        previous.Data.RelationButtons,
							Attributes:             previous.Data.Attributes,
							EnvId:                  environmentId,
						}
						viewFields = []string{}
					)
					for _, v := range previous.Data.ViewFields {
						viewFields = append(viewFields, v.Id)
					}
					createRelation.ViewFields = viewFields

					_, err := services.GetBuilderServiceByType(nodeType).Relation().Create(
						ctx,
						createRelation,
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				case pb.ResourceType_POSTGRESQL:
					var (
						createRelationPsql = &nb.CreateRelationRequest{
							Id:                     previousPsql.Data.Id,
							TableFrom:              previousPsql.Data.TableFrom.Slug,
							TableTo:                previousPsql.Data.TableTo.Slug,
							Type:                   previousPsql.Data.Type,
							AutoFilters:            previousPsql.Data.AutoFilters,
							Summaries:              previousPsql.Data.Summaries,
							Editable:               previousPsql.Data.Editable,
							IsEditable:             previousPsql.Data.IsEditable,
							Title:                  previousPsql.Data.Title,
							Columns:                previousPsql.Data.Columns,
							QuickFilters:           previousPsql.Data.QuickFilters,
							GroupFields:            previousPsql.Data.GroupFields,
							RelationTableSlug:      previousPsql.Data.RelationTableSlug,
							ViewType:               previousPsql.Data.ViewType,
							DynamicTables:          previousPsql.Data.DynamicTables,
							RelationFieldSlug:      previousPsql.Data.RelationFieldSlug,
							DefaultValues:          previousPsql.Data.DefaultValues,
							IsUserIdDefault:        previousPsql.Data.IsUserIdDefault,
							Cascadings:             previousPsql.Data.Cascadings,
							ObjectIdFromJwt:        previousPsql.Data.ObjectIdFromJwt,
							CascadingTreeTableSlug: previousPsql.Data.CascadingTreeFieldSlug,
							CascadingTreeFieldSlug: previousPsql.Data.CascadingTreeFieldSlug,
							ActionRelations:        previousPsql.Data.ActionRelations,
							DefaultLimit:           previousPsql.Data.DefaultLimit,
							MultipleInsert:         previousPsql.Data.MultipleInsert,
							UpdatedFields:          previousPsql.Data.UpdatedFields,
							MultipleInsertField:    previousPsql.Data.MultipleInsertField,
							ProjectId:              resourceEnvId,
							Creatable:              previousPsql.Data.Creatable,
							DefaultEditable:        previousPsql.Data.DefaultEditable,
							FunctionPath:           previousPsql.Data.FunctionPath,
							RelationButtons:        previousPsql.Data.RelationButtons,
							Attributes:             previousPsql.Data.Attributes,
							EnvId:                  environmentId,
						}
						viewFields = []string{}
					)
					for _, v := range previousPsql.Data.ViewFields {
						viewFields = append(viewFields, v.Id)
					}
					createRelationPsql.ViewFields = viewFields

					_, err := services.GoObjectBuilderService().Relation().Create(
						ctx,
						createRelationPsql,
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				}
			}
		} else if actionSource == "MENU" {

			var (
				current           DataUpdateMenuWrapper
				currentPsql       DataUpdateMenuWrapperPsql
				previous          DataUpdateMenuWrapper
				previousPsql      DataUpdateMenuWrapperPsql
				createRequest     DataCreateMenuWrapper
				createRequestPsql DataCreateMenuWrapperPsql
				response          DataUpdateMenuWrapper
				responsePsql      DataUpdateMenuWrapperPsql
			)

			if cast.ToString(v.Current) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Current)), &current)
				if err != nil {
					continue
				}
				current.Data.ProjectId = resourceEnvId
				current.Data.EnvId = environmentId

				err = json.Unmarshal([]byte(cast.ToString(v.Current)), &currentPsql)
				if err != nil {
					continue
				}
				currentPsql.Data.ProjectId = resourceEnvId
				currentPsql.Data.EnvId = environmentId
			}

			if cast.ToString(v.Previous) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
				if err != nil {
					continue
				}
				previous.Data.ProjectId = resourceEnvId
				previous.Data.EnvId = environmentId

				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previousPsql)
				if err != nil {
					continue
				}
				previousPsql.Data.ProjectId = resourceEnvId
				previousPsql.Data.EnvId = environmentId
			}

			if cast.ToString(v.Previous) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &createRequest)
				if err != nil {
					continue
				}
				createRequest.Data.ProjectId = resourceEnvId
				createRequest.Data.EnvId = environmentId

				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &createRequestPsql)
				if err != nil {
					continue
				}
				createRequestPsql.Data.ProjectId = resourceEnvId
				createRequestPsql.Data.EnvId = environmentId
			}

			if cast.ToString(v.Response) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Response)), &response)
				if err != nil {
					continue
				}
				response.Data.ProjectId = resourceEnvId
				response.Data.EnvId = environmentId

				err = json.Unmarshal([]byte(cast.ToString(v.Response)), &responsePsql)
				if err != nil {
					continue
				}
				responsePsql.Data.ProjectId = resourceEnvId
				responsePsql.Data.EnvId = environmentId
			}

			switch actionType {
			case "CREATE":
				switch resourceType {
				case pb.ResourceType_MONGODB:
					_, err = services.GetBuilderServiceByType(nodeType).Menu().Delete(
						ctx,
						&obs.MenuPrimaryKey{
							Id:        current.Data.Id,
							ProjectId: resourceEnvId,
							EnvId:     cast.ToString(environmentId),
						},
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				case pb.ResourceType_POSTGRESQL:
					_, err = services.GoObjectBuilderService().Menu().Delete(
						ctx,
						&nb.MenuPrimaryKey{
							Id:        currentPsql.Data.Id,
							ProjectId: resourceEnvId,
							EnvId:     cast.ToString(environmentId),
						},
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				}
			case "UPDATE":
				switch resourceType {
				case pb.ResourceType_MONGODB:
					_, err := services.GetBuilderServiceByType(nodeType).Menu().Update(
						ctx,
						previous.Data,
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				case pb.ResourceType_POSTGRESQL:
					_, err := services.GoObjectBuilderService().Menu().Update(
						ctx,
						previousPsql.Data,
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				}
			case "DELETE":
				switch resourceType {
				case pb.ResourceType_MONGODB:
					_, err := services.GetBuilderServiceByType(nodeType).Menu().Create(
						ctx,
						createRequest.Data,
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				case pb.ResourceType_POSTGRESQL:
					_, err := services.GoObjectBuilderService().Menu().Create(
						ctx,
						createRequestPsql.Data,
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				}
			}
		} else if actionSource == "VIEW" {

			var (
				current            DataCreateViewWrapper
				currentPsql        DataCreateViewWrapperPsql
				previous           DataUpdateViewWrapper
				previousPsql       DataUpdateViewWrapperPsql
				createPrevious     DataCreateViewWrapper
				createPreviousPsql DataCreateViewWrapperPsql
			)

			if cast.ToString(v.Current) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Current)), &current)
				if err != nil {
					continue
				}
				current.Data.ProjectId = resourceEnvId
				current.Data.EnvId = environmentId

				err = json.Unmarshal([]byte(cast.ToString(v.Current)), &currentPsql)
				if err != nil {
					continue
				}
				currentPsql.Data.ProjectId = resourceEnvId
				currentPsql.Data.EnvId = environmentId
			}

			if cast.ToString(v.Current) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
				if err != nil {
					continue
				}
				previous.Data.ProjectId = resourceEnvId
				previous.Data.EnvId = environmentId

				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previousPsql)
				if err != nil {
					continue
				}
				previousPsql.Data.ProjectId = resourceEnvId
				previousPsql.Data.EnvId = environmentId
			}

			if cast.ToString(v.Previous) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &createPrevious)
				if err != nil {
					continue
				}
				createPrevious.Data.ProjectId = resourceEnvId
				createPrevious.Data.EnvId = environmentId

				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &createPreviousPsql)
				if err != nil {
					continue
				}
				createPreviousPsql.Data.ProjectId = resourceEnvId
				createPreviousPsql.Data.EnvId = environmentId
			}

			switch actionType {
			case "CREATE":
				switch resourceType {
				case pb.ResourceType_MONGODB:
					_, err := services.GetBuilderServiceByType(nodeType).View().Delete(
						ctx,
						&obs.ViewPrimaryKey{
							Id:        current.Data.Id,
							ProjectId: resourceEnvId,
							EnvId:     environmentId,
						},
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				case pb.ResourceType_POSTGRESQL:
					_, err := services.GoObjectBuilderService().View().Delete(
						ctx,
						&nb.ViewPrimaryKey{
							Id:        currentPsql.Data.Id,
							ProjectId: resourceEnvId,
							EnvId:     environmentId,
						},
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				}
			case "UPDATE":
				switch resourceType {
				case pb.ResourceType_MONGODB:
					_, err := services.GetBuilderServiceByType(nodeType).View().Update(
						ctx,
						previous.Data,
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				case pb.ResourceType_POSTGRESQL:
					_, err := services.GoObjectBuilderService().View().Update(
						ctx,
						previousPsql.Data,
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				}
			case "DELETE":
				switch resourceType {
				case pb.ResourceType_MONGODB:
					_, err := services.GetBuilderServiceByType(nodeType).View().Create(
						ctx,
						createPrevious.Data,
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				case pb.ResourceType_POSTGRESQL:
					_, err := services.GoObjectBuilderService().View().Create(
						ctx,
						createPreviousPsql.Data,
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				}
			}
		} else if actionSource == "LAYOUT" {
			var (
				previous     DataUpdateLayoutWrapper
				previousPsql DataUpdateLayoutWrapperPsql
			)

			if cast.ToString(v.Previous) != "" {
				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previous)
				if err != nil {
					continue
				}
				previous.Data.ProjectId = resourceEnvId
				previous.Data.EnvId = environmentId

				err = json.Unmarshal([]byte(cast.ToString(v.Previous)), &previousPsql)
				if err != nil {
					continue
				}
				previousPsql.Data.ProjectId = resourceEnvId
				previousPsql.Data.EnvId = environmentId
			}

			switch actionType {
			case "DELETE", "UPDATE":
				switch resourceType {
				case pb.ResourceType_MONGODB:
					_, err := services.GetBuilderServiceByType(nodeType).Layout().Update(
						ctx,
						previous.Data,
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				case pb.ResourceType_POSTGRESQL:
					_, err := services.GoObjectBuilderService().Layout().Update(
						ctx,
						previousPsql.Data,
					)
					if err != nil {
						continue
					}
					//ids = append(ids, v.Id)
				}
			}
		}
	}

	// resp.Ids = ids

	return nil
}
