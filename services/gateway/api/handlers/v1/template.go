package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/structpb"
)

type CreateTemplateReq struct {
	Tables []*TableResponse `json:"tables"`
	Menu   *nb.MenuTree     `json:"menus"`
}

type ExecuteTemplate struct {
	Id     string       `json:"id"`
	Tables []*Temp      `json:"tables"`
	Menu   *nb.MenuTree `json:"menus"`
}

type Temp struct {
	Id           string                      `json:"table_id"`
	Slug         string                      `json:"slug"`
	WithRows     bool                        `json:"with_rows"`
	Info         *nb.CreateTableRequest      `json:"info"`
	Fields       []*nb.CreateFieldRequest    `json:"fields"`
	Relations    []*nb.CreateRelationRequest `json:"relations"`
	Views        []*nb.CreateViewRequest     `json:"views"`
	Layouts      []*nb.LayoutRequest         `json:"layouts"`
	CustomEvents []*nb.CustomEvent           `json:"custom_events"`
	Functions    []*nb.CreateFunctionRequest `json:"functions"`
	Rows         []map[string]any            `json:"rows"`
}

type TempTable struct {
	Id           string                      `json:"id"`
	Slug         string                      `json:"slug"`
	WithRows     bool                        `json:"with_rows"`
	Info         *nb.CreateTableRequest      `json:"info"`
	Fields       []*nb.CreateFieldRequest    `json:"fields"`
	Relations    []*nb.CreateRelationRequest `json:"relations"`
	Views        []*nb.CreateViewRequest     `json:"views"`
	Layouts      []*nb.LayoutRequest         `json:"layouts"`
	CustomEvents []*nb.CustomEvent           `json:"custom_events"`
	Functions    []*nb.CreateFunctionRequest `json:"functions"`
	Rows         []map[string]any            `json:"rows"`
}

type TableResponse struct {
	Id           string `json:"id"`
	Slug         string `json:"slug"`
	Info         any    `json:"info"`
	Fields       any    `json:"fields"`
	Relations    any    `json:"relations"`
	Views        any    `json:"views"`
	Layouts      any    `json:"layouts"`
	CustomEvents any    `json:"custom_events"`
	Functions    any    `json:"functions"`
	Rows         []any  `json:"rows"`
}

type CreateTemplate struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Photo       string           `json:"photo"`
	MenuId      string           `json:"menu_id"`
	Tables      []map[string]any `json:"tables"`
	Functions   any              `json:"functions"`
	Microfronts any              `json:"microfronts"`
}

// CreateTemplate godoc
// @Security ApiKeyAuth
// @ID create_template
// @Router /v1/template [POST]
// @Summary Create template
// @Description Create template
// @Tags Template
// @Accept json
// @Produce json
// @Param template body CreateTemplateReq true "CreateTemplateReq"
// @Success 201 {object} status_http.Response{data=object} "Template data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) CreateTemplate(c *gin.Context) {
	var (
		template      CreateTemplate
		resourceType  pb.ResourceType
		nodeType      string
		limit, offset int = 100, 0
		menuResp          = &nb.MenuTree{}
		rowsData          = map[string]any{}
	)

	listRequest := map[string]any{
		"limit":  limit,
		"offset": offset,
	}

	structData, err := helper.ConvertMapToStruct(listRequest)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 300*time.Second)
	defer cancel()

	if err := c.ShouldBindJSON(&template); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "error getting environment id | not valid")
		return
	}

	tokenInfo, err := h.GetAuthInfo(c)
	if err != nil {
		h.HandleResponse(c, status_http.Forbidden, err.Error())
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		ctx,
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_TEMPLATE_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(ctx, projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	tableSlugs := map[string]bool{
		"role":         true,
		"client_type":  true,
		"person":       true,
		"sms_template": true,
	}

	tables := make([]*TableResponse, 0)

	resourceType = resource.ResourceType
	nodeType = resource.NodeType

	requestTables, err := convert[[]map[string]any, []TempTable](template.Tables)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(nodeType).Table().GetAll(
			ctx, &obs.GetAllTablesRequest{
				Limit:     int32(limit),
				Offset:    int32(offset),
				ProjectId: resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		menuResp, err = services.GoObjectBuilderService().Menu().GetMenuTree(
			ctx, &nb.MenuPrimaryKey{
				ProjectId: resource.ResourceEnvironmentId,
				Id:        template.MenuId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		for _, table := range requestTables {
			tableResp, err := services.GoObjectBuilderService().Table().GetByID(
				ctx, &nb.TablePrimaryKey{
					Id:        table.Id,
					ProjectId: resource.ResourceEnvironmentId,
				},
			)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}

			if tableSlugs[tableResp.Slug] {
				continue
			}

			fieldResp, err := services.GoObjectBuilderService().Field().GetAll(
				ctx, &nb.GetAllFieldsRequest{
					Limit:     int32(limit),
					Offset:    int32(offset),
					TableId:   table.Id,
					TableSlug: tableResp.Slug,
					ProjectId: resource.ResourceEnvironmentId,
				},
			)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}

			relationResp, err := services.GoObjectBuilderService().Relation().GetRelationsByTableFrom(
				ctx, &nb.GetRelationsByTableFromRequest{
					TableFrom: tableResp.Slug,
					ProjectId: resource.ResourceEnvironmentId,
				},
			)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}

			layoutResp, err := services.GoObjectBuilderService().Layout().GetLayoutByTableID(
				ctx, &nb.GetLayoutByTableIDRequest{
					TableId:   table.Id,
					ProjectId: resource.ResourceEnvironmentId,
				},
			)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}

			viewResp, err := services.GoObjectBuilderService().View().GetList(
				ctx, &nb.GetAllViewsRequest{
					TableSlug: tableResp.Slug,
					ProjectId: resource.ResourceEnvironmentId,
					RoleId:    tokenInfo.RoleId,
					MenuId:    tableResp.Slug,
				},
			)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}

			customEventResp, err := services.GoObjectBuilderService().CustomEvent().GetList(
				ctx, &nb.GetCustomEventsListRequest{
					TableSlug: tableResp.Slug,
					RoleId:    tokenInfo.RoleId,
					ProjectId: resource.ResourceEnvironmentId,
				},
			)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}

			tbl, err := convert[*nb.Table, any](tableResp)

			fields, err := convert[[]*nb.Field, any](fieldResp.Fields)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
			relations, err := convert[[]*nb.CreateRelationRequest, any](relationResp.Relations)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
			views, err := convert[[]*nb.View, any](viewResp.Views)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
			layouts, err := convert[[]*nb.LayoutResponse, any](layoutResp.Layouts)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
			customeevents, err := convert[[]*nb.CustomEvent, any](customEventResp.CustomEvents)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}

			if table.WithRows {
				rows, err := services.GoObjectBuilderService().ObjectBuilder().GetList2(
					c.Request.Context(), &nb.CommonMessage{
						TableSlug: tableResp.Slug,
						Data:      structData,
						ProjectId: resource.ResourceEnvironmentId,
					},
				)
				if err != nil {
					h.HandleResponse(c, status_http.GRPCError, err.Error())
					return
				}

				rowsData, err = convert[*structpb.Struct, map[string]any](rows.Data)
				if err != nil {
					h.HandleResponse(c, status_http.GRPCError, err.Error())
					return
				}
			}

			tables = append(tables, &TableResponse{
				Id:           table.Id,
				Slug:         tableResp.Slug,
				Info:         tbl,
				Fields:       fields,
				Relations:    relations,
				Views:        views,
				Layouts:      layouts,
				CustomEvents: customeevents,
				Rows:         cast.ToSlice(rowsData["response"]),
			})
		}
	}

	tablesReq := CreateTemplateReq{
		Tables: tables,
		Menu:   menuResp,
	}

	tbls, err := convert[CreateTemplateReq, *structpb.Struct](tablesReq)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	functions, err := convert[map[string]any, *structpb.Struct](map[string]any{
		"functions": template.Functions,
	})
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	microfronts, err := convert[map[string]any, *structpb.Struct](map[string]any{
		"microfronts": template.Microfronts,
	})
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	res, err := h.companyServices.Template().Create(ctx, &pb.CreateTemplateMetadataReq{
		Name:        template.Name,
		Description: template.Description,
		Photo:       template.Photo,
		Tables:      tbls,
		Functions:   functions,
		Microfronts: microfronts,
	})

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, res)
}

func convert[T any, U any](in T) (U, error) {
	var out U

	data, err := json.Marshal(in)
	if err != nil {
		return out, err
	}

	err = json.Unmarshal(data, &out)
	return out, err
}

// GetSingleTemplate godoc
// @Security ApiKeyAuth
// @ID get_single_template
// @Router /v1/template/{template-id} [GET]
// @Summary Get single template
// @Description Get single template
// @Tags Template
// @Accept json
// @Produce json
// @Param template-id path string true "template-id"
// @Success 200 {object} status_http.Response{data=object} "TemplateBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetSingleTemplate(c *gin.Context) {
	templateId := c.Param("template-id")

	if !util.IsValidUUID(templateId) {
		h.HandleResponse(c, status_http.InvalidArgument, "folder id is an invalid uuid")
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "environment id is an invalid uuid")
		return
	}

	resp, err := h.companyServices.Template().GetById(c.Request.Context(), &pb.GetTemplateMetadataByIdReq{
		Id: templateId,
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	anny, err := convert[*pb.TemplateMetadata, map[string]any](resp)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, anny)
}

// DeleteTemplate godoc
// @Security ApiKeyAuth
// @ID delete_template
// @Router /v1/template/{template-id} [DELETE]
// @Summary Delete template
// @Description Delete template
// @Tags Template
// @Accept json
// @Produce json
// @Param template-id path string true "template-id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) DeleteTemplate(c *gin.Context) {
	templateId := c.Param("template-id")

	if !util.IsValidUUID(templateId) {
		h.HandleResponse(c, status_http.InvalidArgument, "view id is an invalid uuid")
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "environment id is an invalid uuid")
		return
	}

	res, err := h.companyServices.Template().Delete(c.Request.Context(), &pb.DeleteTemplateMetadataReq{
		Id: templateId,
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.NoContent, res)
}

// GetListTemplate godoc
// @Security ApiKeyAuth
// @ID get_list_template
// @Router /v1/template [GET]
// @Summary Get List template
// @Description Get List template
// @Tags Template
// @Accept json
// @Produce json
// @Param folder-id query string true "folder-id"
// @Param limit query string false "limit"
// @Param offset query string false "offset"
// @Success 200 {object} status_http.Response{data=object} "FolderBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetListTemplate(c *gin.Context) {
	limit, err := h.getLimitParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "environment id is an invalid uuid")
		return
	}

	res, err := h.companyServices.Template().List(c.Request.Context(),
		&pb.GetTemplateMetadataListReq{
			Limit:  int32(limit),
			Offset: int32(offset),
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	anny, err := convert[*pb.GetTemplateMetadataListResponse, map[string]any](res)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, anny)
}

func (h *HandlerV1) ExecuteTemplate(c *gin.Context) {
	var (
		resourceType pb.ResourceType
		body         ExecuteTemplate
		template     ExecuteTemplate
	)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 300*time.Second)
	defer cancel()

	if err := c.ShouldBindJSON(&body); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "environment id is an invalid uuid")
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		ctx,
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_TEMPLATE_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	resourceType = resource.ResourceType

	services, err := h.GetProjectSrvc(ctx, projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	templateId := body.Id
	template = body
	if templateId != "" {
		res, err := h.companyServices.Template().GetById(c.Request.Context(),
			&pb.GetTemplateMetadataByIdReq{
				Id: templateId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		template, err = convert[*structpb.Struct, ExecuteTemplate](res.Tables)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}

	fieldSlugMap := map[string]bool{
		"guid":       true,
		"created_at": true,
		"updated_at": true,
		"deleted_at": true,
		"folder_id":  true,
	}

	fieldTypeMap := map[string]bool{
		"LOOKUP":  true,
		"LOOKUPS": true,
	}

	switch resourceType {
	case pb.ResourceType_MONGODB:
	case pb.ResourceType_POSTGRESQL:
		for _, table := range template.Tables {
			createtable := table.Info
			createtable.ProjectId = resource.ResourceEnvironmentId
			_, err := services.GoObjectBuilderService().Table().Create(ctx, table.Info)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
		}

		if template.Menu != nil {
			err = h.createMenusRecursively(ctx, services, template.Menu, resource.ResourceEnvironmentId, template.Menu.ParentId)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("Failed to create menus: %v", err))
				return
			}
		}

		for _, table := range template.Tables {
			for _, field := range table.Fields {
				if fieldSlugMap[field.Slug] {
					continue
				}

				if fieldTypeMap[field.Type] {
					continue
				}

				field.ProjectId = resource.ResourceEnvironmentId
				_, err := services.GoObjectBuilderService().Field().Create(ctx, field)
				if err != nil {
					h.HandleResponse(c, status_http.GRPCError, err.Error())
					return
				}
			}

			for _, relation := range table.Relations {
				relation.RelationFieldId = uuid.NewString()
				relation.RelationToFieldId = uuid.NewString()
				relation.Attributes, _ = convert[map[string]any, *structpb.Struct](map[string]any{})
				relation.ProjectId = resource.ResourceEnvironmentId
				_, err := services.GoObjectBuilderService().Relation().Create(ctx, relation)
				if err != nil {
					h.HandleResponse(c, status_http.GRPCError, err.Error())
					return
				}
			}

			for _, layout := range table.Layouts {
				layout.ProjectId = resource.ResourceEnvironmentId
				layout.WithoutResponse = true
				_, err := services.GoObjectBuilderService().Layout().Update(ctx, layout)
				if err != nil {
					h.HandleResponse(c, status_http.GRPCError, err.Error())
					return
				}
			}

			for _, view := range table.Views {
				view.ProjectId = resource.ResourceEnvironmentId

				if view.MenuId != "" {
					_, getMenuErr := services.GoObjectBuilderService().Menu().GetByID(
						ctx,
						&nb.MenuPrimaryKey{
							Id:        view.MenuId,
							ProjectId: resource.ResourceEnvironmentId,
						},
					)
					if getMenuErr != nil {
						menuLabel := table.Info.GetLabel()
						if menuLabel == "" {
							menuLabel = table.Slug
						}

						attribute := map[string]any{
							"label_en": menuLabel,
							"label_ru": menuLabel,
						}

						attributeStruct, err := convert[map[string]any, *structpb.Struct](attribute)
						if err != nil {
							h.HandleResponse(c, status_http.GRPCError, err.Error())
							return
						}

						createdMenu, createMenuErr := services.GoObjectBuilderService().Menu().Create(ctx,
							&nb.CreateMenuRequest{
								Label:      menuLabel,
								ProjectId:  resource.ResourceEnvironmentId,
								TableId:    table.Info.Id,
								Type:       "TABLE",
								Id:         view.MenuId,
								Attributes: attributeStruct,
								ParentId:   "c57eedc3-a954-4262-a0af-376c65b5a284",
							},
						)
						if createMenuErr != nil {
							h.HandleResponse(c, status_http.GRPCError, createMenuErr.Error())
							return
						}

						view.MenuId = createdMenu.Id
					}
				}
				_, err := services.GoObjectBuilderService().View().Create(ctx, view)
				if err != nil {
					h.HandleResponse(c, status_http.GRPCError, err.Error())
					return
				}
			}

			for _, customevent := range table.CustomEvents {
				// Create custom event
				customEventReq := &nb.CreateCustomEventRequest{
					TableSlug:  table.Info.Slug,
					Icon:       customevent.Icon,
					EventPath:  customevent.EventPath,
					Label:      customevent.Label,
					Url:        customevent.Url,
					Disable:    customevent.Disable,
					ProjectId:  resource.ResourceEnvironmentId,
					Method:     customevent.Method,
					ActionType: customevent.ActionType,
					Attributes: customevent.Attributes,
					EnvId:      resource.ResourceEnvironmentId,
					Path:       customevent.Path,
				}

				_, err = services.GoObjectBuilderService().CustomEvent().Create(ctx, customEventReq)
				if err != nil {
					h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("Failed to create custom event '%s': %v", customevent.Label, err))
					return
				}
			}

			result := models.CommonMessage{}

			if len(table.Rows) > 0 {
				var fields []string
				for field := range table.Rows[0] {
					fields = append(fields, field)
				}

				// Build the structure
				data := map[string]any{
					"objects":    table.Rows,
					"field_slug": "guid",
					"fields":     fields,
				}

				result.Data = data

				structData, err := helper.ConvertMapToStruct(result.Data)
				if err != nil {
					h.HandleResponse(c, status_http.InvalidArgument, err.Error())
					return
				}

				_, err = services.GoObjectBuilderService().Items().UpsertMany(ctx, &nb.CommonMessage{
					TableSlug: table.Slug,
					Data:      structData,
					ProjectId: resource.ResourceEnvironmentId,
				})
				if err != nil {
					h.HandleResponse(c, status_http.InvalidArgument, err.Error())
					return
				}
			}
		}
	}

	h.HandleResponse(c, status_http.OK, "OK")
}

// createMenusRecursively creates menus and their children recursively
func (h *HandlerV1) createMenusRecursively(ctx context.Context, services services.ServiceManagerI, menu *nb.MenuTree, projectId, parentId string) error {
	if menu == nil {
		return nil
	}

	// Prepare menu data for creation
	menuData := &nb.CreateMenuRequest{
		Id:              menu.Id,
		Label:           menu.Label,
		Icon:            menu.Icon,
		Type:            menu.Type,
		ProjectId:       projectId,
		ParentId:        parentId,
		MicrofrontendId: menu.MicrofrontendId,
		WebpageId:       menu.WebpageId,
		Attributes:      menu.Attributes,
		WikiId:          menu.WikiId,
		IsVisible:       menu.IsVisible,
		EnvId:           menu.EnvId,
		TableId:         menu.TableId,
		LayoutId:        menu.LayoutId,
	}

	// Create the menu
	createdMenu, err := services.GoObjectBuilderService().Menu().Create(ctx, menuData)
	if err != nil {
		return fmt.Errorf("failed to create menu '%s': %v", menu.Label, err)
	}

	// Recursively create children menus
	if len(menu.Children) > 0 {
		for _, child := range menu.Children {
			err = h.createMenusRecursively(ctx, services, child, projectId, createdMenu.Id)
			if err != nil {
				return fmt.Errorf("failed to create child menu for '%s': %v", menu.Label, err)
			}
		}
	}

	return nil
}
