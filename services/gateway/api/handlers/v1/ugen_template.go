package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	as "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	pbo "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"
	servicepkg "github.com/Ucode-io/ucode-opensource/services/gateway/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

// ─── CRUD ────────────────────────────────────────────────────────────────────

type ugenTemplateResponse struct {
	Id                     string   `json:"id"`
	Name                   string   `json:"name"`
	Description            string   `json:"description"`
	Photo                  string   `json:"photo"`
	McpProjectId           string   `json:"mcp_project_id"`
	OrderNumber            int32    `json:"order_number"`
	CreatedAt              string   `json:"created_at"`
	UpdatedAt              string   `json:"updated_at"`
	DeletedAt              int64    `json:"deleted_at"`
	PreviewUrl             string   `json:"preview_url"`
	SourceResourceEnvId    string   `json:"source_resource_env_id"`
	SourceProjectId        string   `json:"source_project_id"`
	SourceEnvironmentId    string   `json:"source_environment_id"`
	SourceNodeType         string   `json:"source_node_type"`
	SourceMcpResourceEnvId string   `json:"source_mcp_resource_env_id"`
	SourceFunctionId       string   `json:"source_function_id"`
	SourceRepoId           string   `json:"source_repo_id"`
	Images                 []string `json:"images"`
	LikeCount              int32    `json:"like_count"`
	DislikeCount           int32    `json:"dislike_count"`
	CurrentUserReaction    string   `json:"current_user_reaction"`
	Price                  float64  `json:"price"`
	CurrencyId             string   `json:"currency_id"`
	PerUserPrice           float64  `json:"per_user_price"`
}

type ugenTemplateListResponse struct {
	Templates []*ugenTemplateResponse `json:"templates"`
	Count     int32                   `json:"count"`
}

type ugenTemplateReactionResponse struct {
	Id           string `json:"id"`
	TemplateId   string `json:"template_id"`
	UserId       string `json:"user_id"`
	ReactionType string `json:"reaction_type"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	DeletedAt    int64  `json:"deleted_at"`
}

type ugenTemplateReactionListResponse struct {
	Reactions []*ugenTemplateReactionResponse `json:"reactions"`
	Count     int32                           `json:"count"`
}

type setUgenTemplateReactionRequest struct {
	ReactionType string `json:"reaction_type" binding:"required"`
}

func (h *HandlerV1) CreateUgenTemplate(c *gin.Context) {
	var req pb.CreateUgenTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	if err := h.enrichCreateUgenTemplateSource(c, &req); err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	resp, err := h.companyServices.UgenTemplate().Create(c.Request.Context(), &req)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	h.HandleResponse(c, status_http.OK, newUgenTemplateResponse(resp))
}

func (h *HandlerV1) enrichCreateUgenTemplateSource(c *gin.Context, req *pb.CreateUgenTemplateReq) error {
	ctx := c.Request.Context()

	if req.GetMcpProjectId() == "" {
		return fmt.Errorf("mcp_project_id is required")
	}
	if req.GetSourceFunctionId() == "" {
		return fmt.Errorf("source_function_id is required")
	}

	// The mcp_project ROW lives in the head/current project's builder DB, so read
	// it from the request context. Everything about the source is then derived
	// from that row — we do NOT trust client-sent resource-env ids, because the
	// data env (where the source project's tables live) is its own CHILD builder
	// env, not the head env (which only has the default tables).
	headProjectID, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(headProjectID.(string)) {
		return config.ErrProjectIdValid
	}
	headEnvID, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(headEnvID.(string)) {
		return config.ErrEnvironmentIdValid
	}

	headResource, err := h.companyServices.ServiceResource().GetSingle(
		ctx,
		&pb.GetSingleServiceResourceReq{
			ProjectId:     headProjectID.(string),
			EnvironmentId: headEnvID.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		return fmt.Errorf("get head builder resource: %w", err)
	}
	mcpResourceEnvID := headResource.GetResourceEnvironmentId()
	if mcpResourceEnvID == "" || headResource.GetNodeType() == "" {
		return fmt.Errorf("head builder resource is incomplete")
	}

	headService, err := h.GetProjectSrvc(ctx, headProjectID.(string), headResource.GetNodeType())
	if err != nil {
		return fmt.Errorf("get head project service: %w", err)
	}

	sourceMcp, err := headService.GoObjectBuilderService().McpProject().GetMcpProjectFiles(
		ctx,
		&pbo.McpProjectId{
			ResourceEnvId: mcpResourceEnvID,
			Id:            req.GetMcpProjectId(),
			WithoutFiles:  true,
		},
	)
	if err != nil {
		return fmt.Errorf("get source mcp project: %w", err)
	}
	if !util.IsValidUUID(sourceMcp.GetUcodeProjectId()) || !util.IsValidUUID(sourceMcp.GetEnvironmentId()) {
		return fmt.Errorf("source mcp project has no child ucode_project_id/environment_id")
	}

	// Resolve the source project's DATA env — its own child builder resource env,
	// where its tables/rows/functions live.
	dataResource, err := h.companyServices.ServiceResource().GetSingle(
		ctx,
		&pb.GetSingleServiceResourceReq{
			ProjectId:     sourceMcp.GetUcodeProjectId(),
			EnvironmentId: sourceMcp.GetEnvironmentId(),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		return fmt.Errorf("get source data builder resource: %w", err)
	}
	if dataResource.GetResourceEnvironmentId() == "" || dataResource.GetNodeType() == "" {
		return fmt.Errorf("source data builder resource is incomplete")
	}

	req.SourceResourceEnvId = dataResource.GetResourceEnvironmentId()
	req.SourceMcpResourceEnvId = mcpResourceEnvID
	req.SourceProjectId = sourceMcp.GetUcodeProjectId()
	req.SourceEnvironmentId = sourceMcp.GetEnvironmentId()
	req.SourceNodeType = dataResource.GetNodeType()

	sourceDataService, err := h.GetProjectSrvc(ctx, req.GetSourceProjectId(), req.GetSourceNodeType())
	if err != nil {
		return fmt.Errorf("get source data project service: %w", err)
	}

	sourceFunction, err := sourceDataService.GoObjectBuilderService().Function().GetSingle(
		ctx,
		&pbo.FunctionPrimaryKey{
			Id:        req.GetSourceFunctionId(),
			ProjectId: req.GetSourceResourceEnvId(),
		},
	)
	if err != nil {
		return fmt.Errorf("get source function: %w", err)
	}

	if req.GetSourceRepoId() == "" {
		req.SourceRepoId = sourceFunction.GetRepoId()
	} else if sourceFunction.GetRepoId() != "" && req.GetSourceRepoId() != sourceFunction.GetRepoId() {
		return fmt.Errorf("source_repo_id does not match source_function_id repo_id")
	}
	if req.GetSourceRepoId() == "" {
		return fmt.Errorf("source_repo_id is required")
	}

	if req.GetPreviewUrl() == "" && sourceFunction.GetUrl() != "" {
		req.PreviewUrl = normalizeUgenTemplatePreviewURL(sourceFunction.GetUrl())
	}

	return nil
}

func normalizeUgenTemplatePreviewURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}
	return "https://" + strings.TrimPrefix(rawURL, "//")
}

func (h *HandlerV1) GetUgenTemplateById(c *gin.Context) {
	id := c.Param("id")
	if !util.IsValidUUID(id) {
		h.HandleResponse(c, status_http.InvalidArgument, "invalid id")
		return
	}

	resp, err := h.companyServices.UgenTemplate().GetById(
		c.Request.Context(),
		&pb.GetUgenTemplateByIdReq{
			Id:     id,
			UserId: getOptionalUgenTemplateUserID(c),
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	h.HandleResponse(c, status_http.OK, newUgenTemplateResponse(resp))
}

func (h *HandlerV1) GetUgenTemplateList(c *gin.Context) {
	limit := cast.ToInt32(c.Query("limit"))
	offset := cast.ToInt32(c.Query("offset"))
	if limit == 0 {
		limit = 10
	}

	resp, err := h.companyServices.UgenTemplate().List(
		c.Request.Context(),
		&pb.GetUgenTemplateListReq{
			Limit:  limit,
			Offset: offset,
			UserId: getOptionalUgenTemplateUserID(c),
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	h.HandleResponse(c, status_http.OK, newUgenTemplateListResponse(resp))
}

func (h *HandlerV1) UpdateUgenTemplate(c *gin.Context) {
	var req pb.UpdateUgenTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	req.Id = c.Param("id")
	if !util.IsValidUUID(req.Id) {
		h.HandleResponse(c, status_http.InvalidArgument, "invalid id")
		return
	}

	resp, err := h.companyServices.UgenTemplate().Update(c.Request.Context(), &req)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	h.HandleResponse(c, status_http.OK, newUgenTemplateResponse(resp))
}

type setUgenTemplatePriceRequest struct {
	Price        float64 `json:"price"`
	CurrencyId   string  `json:"currency_id"`
	PerUserPrice float64 `json:"per_user_price"`
}

// SetUgenTemplatePrice is the admin-only price control. Templates are created for
// free and users cannot set a price via create/update — only this endpoint
// (called from the admin panel) assigns or updates a template's price and
// currency. Price 0 makes the template free again.
func (h *HandlerV1) SetUgenTemplatePrice(c *gin.Context) {
	id := c.Param("id")
	if !util.IsValidUUID(id) {
		h.HandleResponse(c, status_http.InvalidArgument, "invalid id")
		return
	}

	var req setUgenTemplatePriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	if req.Price < 0 || req.PerUserPrice < 0 {
		h.HandleResponse(c, status_http.BadRequest, "price cannot be negative")
		return
	}

	resp, err := h.companyServices.UgenTemplate().SetPrice(
		c.Request.Context(),
		&pb.SetUgenTemplatePriceReq{
			Id:           id,
			Price:        req.Price,
			CurrencyId:   req.CurrencyId,
			PerUserPrice: req.PerUserPrice,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	h.HandleResponse(c, status_http.OK, newUgenTemplateResponse(resp))
}

func (h *HandlerV1) DeleteUgenTemplate(c *gin.Context) {
	id := c.Param("id")
	if !util.IsValidUUID(id) {
		h.HandleResponse(c, status_http.InvalidArgument, "invalid id")
		return
	}

	_, err := h.companyServices.UgenTemplate().Delete(
		c.Request.Context(),
		&pb.DeleteUgenTemplateReq{Id: id},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	h.HandleResponse(c, status_http.OK, gin.H{"message": "deleted"})
}

func (h *HandlerV1) SetUgenTemplateReaction(c *gin.Context) {
	templateID := c.Param("id")
	if !util.IsValidUUID(templateID) {
		h.HandleResponse(c, status_http.InvalidArgument, "invalid id")
		return
	}

	var req setUgenTemplateReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	reactionType, err := ugenTemplateReactionTypeFromRequest(req.ReactionType)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	userID, err := h.getUgenTemplateUserID(c)
	if err != nil {
		h.HandleResponse(c, status_http.Unauthorized, err.Error())
		return
	}

	resp, err := h.companyServices.UgenTemplate().SetReaction(
		c.Request.Context(),
		&pb.SetUgenTemplateReactionReq{
			TemplateId:   templateID,
			UserId:       userID,
			ReactionType: reactionType,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	h.HandleResponse(c, status_http.OK, newUgenTemplateReactionResponse(resp))
}

func (h *HandlerV1) DeleteUgenTemplateReaction(c *gin.Context) {
	templateID := c.Param("id")
	if !util.IsValidUUID(templateID) {
		h.HandleResponse(c, status_http.InvalidArgument, "invalid id")
		return
	}

	userID, err := h.getUgenTemplateUserID(c)
	if err != nil {
		h.HandleResponse(c, status_http.Unauthorized, err.Error())
		return
	}

	_, err = h.companyServices.UgenTemplate().DeleteReaction(
		c.Request.Context(),
		&pb.DeleteUgenTemplateReactionReq{
			TemplateId: templateID,
			UserId:     userID,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	h.HandleResponse(c, status_http.OK, gin.H{"message": "deleted"})
}

func (h *HandlerV1) GetUgenTemplateReactionList(c *gin.Context) {
	templateID := c.Param("id")
	if !util.IsValidUUID(templateID) {
		h.HandleResponse(c, status_http.InvalidArgument, "invalid id")
		return
	}

	reactionType, err := ugenTemplateOptionalReactionTypeFromRequest(c.Query("reaction_type"))
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	limit := cast.ToInt32(c.Query("limit"))
	offset := cast.ToInt32(c.Query("offset"))
	if limit == 0 {
		limit = 10
	}

	resp, err := h.companyServices.UgenTemplate().ListReactions(
		c.Request.Context(),
		&pb.GetUgenTemplateReactionListReq{
			TemplateId:   templateID,
			ReactionType: reactionType,
			Limit:        limit,
			Offset:       offset,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	h.HandleResponse(c, status_http.OK, newUgenTemplateReactionListResponse(resp))
}

func newUgenTemplateListResponse(resp *pb.GetUgenTemplateListResponse) *ugenTemplateListResponse {
	if resp == nil {
		return &ugenTemplateListResponse{}
	}

	templates := make([]*ugenTemplateResponse, 0, len(resp.GetTemplates()))
	for _, template := range resp.GetTemplates() {
		templates = append(templates, newUgenTemplateResponse(template))
	}

	return &ugenTemplateListResponse{
		Templates: templates,
		Count:     resp.GetCount(),
	}
}

func newUgenTemplateResponse(t *pb.UgenTemplate) *ugenTemplateResponse {
	if t == nil {
		return nil
	}

	return &ugenTemplateResponse{
		Id:                     t.GetId(),
		Name:                   t.GetName(),
		Description:            t.GetDescription(),
		Photo:                  t.GetPhoto(),
		McpProjectId:           t.GetMcpProjectId(),
		OrderNumber:            t.GetOrderNumber(),
		CreatedAt:              t.GetCreatedAt(),
		UpdatedAt:              t.GetUpdatedAt(),
		DeletedAt:              t.GetDeletedAt(),
		PreviewUrl:             t.GetPreviewUrl(),
		SourceResourceEnvId:    t.GetSourceResourceEnvId(),
		SourceProjectId:        t.GetSourceProjectId(),
		SourceEnvironmentId:    t.GetSourceEnvironmentId(),
		SourceNodeType:         t.GetSourceNodeType(),
		SourceMcpResourceEnvId: t.GetSourceMcpResourceEnvId(),
		SourceFunctionId:       t.GetSourceFunctionId(),
		SourceRepoId:           t.GetSourceRepoId(),
		Images:                 t.GetImages(),
		LikeCount:              t.GetLikeCount(),
		DislikeCount:           t.GetDislikeCount(),
		CurrentUserReaction:    ugenTemplateReactionTypeToResponse(t.GetCurrentUserReaction()),
		Price:                  t.GetPrice(),
		CurrencyId:             t.GetCurrencyId(),
		PerUserPrice:           t.GetPerUserPrice(),
	}
}

func newUgenTemplateReactionListResponse(resp *pb.GetUgenTemplateReactionListResponse) *ugenTemplateReactionListResponse {
	if resp == nil {
		return &ugenTemplateReactionListResponse{}
	}

	reactions := make([]*ugenTemplateReactionResponse, 0, len(resp.GetReactions()))
	for _, reaction := range resp.GetReactions() {
		reactions = append(reactions, newUgenTemplateReactionResponse(reaction))
	}

	return &ugenTemplateReactionListResponse{
		Reactions: reactions,
		Count:     resp.GetCount(),
	}
}

func newUgenTemplateReactionResponse(reaction *pb.UgenTemplateReaction) *ugenTemplateReactionResponse {
	if reaction == nil {
		return nil
	}
	return &ugenTemplateReactionResponse{
		Id:           reaction.GetId(),
		TemplateId:   reaction.GetTemplateId(),
		UserId:       reaction.GetUserId(),
		ReactionType: ugenTemplateReactionTypeToResponse(reaction.GetReactionType()),
		CreatedAt:    reaction.GetCreatedAt(),
		UpdatedAt:    reaction.GetUpdatedAt(),
		DeletedAt:    reaction.GetDeletedAt(),
	}
}

func (h *HandlerV1) getUgenTemplateUserID(c *gin.Context) (string, error) {
	authInfo, err := h.adminAuthInfo(c)
	if err != nil {
		return "", err
	}
	userID := authInfo.GetUserIdAuth()
	if userID == "" {
		userID = authInfo.GetUserId()
	}
	if userID == "" {
		return "", fmt.Errorf("user_id is required")
	}
	return userID, nil
}

func getOptionalUgenTemplateUserID(c *gin.Context) string {
	data, ok := c.Get("Auth_Admin")
	if !ok {
		return ""
	}
	authInfo, ok := data.(*as.HasAccessSuperAdminRes)
	if !ok {
		return ""
	}
	if authInfo.GetUserIdAuth() != "" {
		return authInfo.GetUserIdAuth()
	}
	return authInfo.GetUserId()
}

func ugenTemplateOptionalReactionTypeFromRequest(reactionType string) (pb.UgenTemplateReactionType, error) {
	if strings.TrimSpace(reactionType) == "" {
		return pb.UgenTemplateReactionType_UGEN_TEMPLATE_REACTION_TYPE_UNSPECIFIED, nil
	}
	return ugenTemplateReactionTypeFromRequest(reactionType)
}

func ugenTemplateReactionTypeFromRequest(reactionType string) (pb.UgenTemplateReactionType, error) {
	switch strings.ToLower(strings.TrimSpace(reactionType)) {
	case "like", "ugen_template_reaction_type_like":
		return pb.UgenTemplateReactionType_UGEN_TEMPLATE_REACTION_TYPE_LIKE, nil
	case "dislike", "ugen_template_reaction_type_dislike":
		return pb.UgenTemplateReactionType_UGEN_TEMPLATE_REACTION_TYPE_DISLIKE, nil
	default:
		return pb.UgenTemplateReactionType_UGEN_TEMPLATE_REACTION_TYPE_UNSPECIFIED, fmt.Errorf("reaction_type must be like or dislike")
	}
}

func ugenTemplateReactionTypeToResponse(reactionType pb.UgenTemplateReactionType) string {
	switch reactionType {
	case pb.UgenTemplateReactionType_UGEN_TEMPLATE_REACTION_TYPE_LIKE:
		return "like"
	case pb.UgenTemplateReactionType_UGEN_TEMPLATE_REACTION_TYPE_DISLIKE:
		return "dislike"
	default:
		return ""
	}
}

// ─── Create project from template ────────────────────────────────────────────

type CreateProjectFromTemplateReq struct {
	TemplateId  string `json:"template_id" binding:"required"`
	ProjectName string `json:"project_name"`

	IdempotencyKey string `json:"idempotency_key"`
}

// CreateProjectFromTemplate provisions a new isolated ucode project from a

func (h *HandlerV1) CreateProjectFromTemplate(c *gin.Context) {
	var req CreateProjectFromTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	var (
		projectId     any
		environmentId any
		ok            bool
		ctx           = context.Background()
	)

	projectId, ok = c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

	environmentId, ok = c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrEnvironmentIdValid)
		return
	}

	authInfo, err := h.adminAuthInfo(c)
	if err != nil {
		h.HandleResponse(c, status_http.Unauthorized, "unauthorized")
		return
	}

	tmpl, err := h.companyServices.UgenTemplate().GetById(
		ctx, &pb.GetUgenTemplateByIdReq{Id: req.TemplateId},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("get template: %v", err))
		return
	}

	projectName := req.ProjectName
	if projectName == "" {
		projectName = tmpl.GetName()
	}

	headProject, err := h.companyServices.Project().GetById(
		ctx, &pb.GetProjectByIdRequest{ProjectId: projectId.(string)},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("get head project: %v", err))
		return
	}

	headResource, err := h.companyServices.ServiceResource().GetSingle(
		ctx,
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("get head resource: %v", err))
		return
	}

	// The new project's MCP project and chat live in the head project's builder
	// resource env, exactly like manually created u-gen projects.
	mainResourceEnvID := headResource.GetResourceEnvironmentId()
	mainService, err := h.GetProjectSrvc(ctx, projectId.(string), headResource.GetNodeType())
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("get head project service: %v", err))
		return
	}

	// Source project the template was captured from. Everything below reads
	// straight from the template's stored source fields — no fallbacks.
	sourceService, err := h.GetProjectSrvc(ctx, tmpl.GetSourceProjectId(), tmpl.GetSourceNodeType())
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("get source project service: %v", err))
		return
	}

	// MCP project metadata (env vars / visibility / type) is a nice-to-have. The
	// template's mcp row and its table data can live in different DBs, so try the
	// mcp env first, then the data env; if the row is in neither, don't fail the
	// clone — proceed with empty metadata (proto getters are nil-safe). Its
	// project_files are ignored here regardless.
	sourceMcp, mcpErr := sourceService.GoObjectBuilderService().McpProject().GetMcpProjectFiles(
		ctx,
		&pbo.McpProjectId{
			ResourceEnvId: tmpl.GetSourceMcpResourceEnvId(),
			Id:            tmpl.GetMcpProjectId(),
			WithoutFiles:  true,
		},
	)
	if mcpErr != nil {
		log.Printf("[ugen-template] source mcp metadata not in mcp env %s: %v", tmpl.GetSourceMcpResourceEnvId(), mcpErr)
		sourceMcp, mcpErr = sourceService.GoObjectBuilderService().McpProject().GetMcpProjectFiles(
			ctx,
			&pbo.McpProjectId{
				ResourceEnvId: tmpl.GetSourceResourceEnvId(),
				Id:            tmpl.GetMcpProjectId(),
				WithoutFiles:  true,
			},
		)
		if mcpErr != nil {
			log.Printf("[ugen-template] source mcp metadata not in data env %s either; proceeding with empty metadata: %v", tmpl.GetSourceResourceEnvId(), mcpErr)
			sourceMcp = nil
		}
	}

	// Microfrontend files come straight from the microfrontend repo, resolved via
	// the MCP resource env (source_mcp_resource_env_id) — not from project_files.
	sourceMcpFiles, err := h.getTemplateMicrofrontendFiles(ctx, sourceService, tmpl, tmpl.GetSourceMcpResourceEnvId(), c.GetHeader("Authorization"))
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}
	if len(sourceMcpFiles) == 0 {
		h.HandleResponse(c, status_http.InvalidArgument, "template source has no microfrontend files")
		return
	}

	// Charge the head project's balance for a paid template before provisioning
	// anything. Free templates (price 0) skip billing entirely. The charge is
	// atomic and overdraft-checked in the billing service; on any later
	// provisioning failure we issue a compensating refund below so the user never
	// pays for a project that was not fully created.
	var (
		chargeTxID    string
		chargedAmount float64
	)

	if tmpl.GetPrice() > 0 {
		idempotencyKey := req.IdempotencyKey
		if idempotencyKey == "" {
			idempotencyKey = uuid.NewString()
		}
		creatorID := authInfo.GetUserIdAuth()
		if creatorID == "" {
			creatorID = authInfo.GetUserId()
		}
		charge, chargeErr := h.companyServices.Billing().ChargeProjectBalance(ctx, &pb.ChargeProjectBalanceRequest{
			ProjectId:       projectId.(string),
			Amount:          tmpl.GetPrice(),
			CurrencyId:      tmpl.GetCurrencyId(),
			CreatorId:       creatorID,
			Comment:         "ugen template: " + tmpl.GetName(),
			TransactionType: "template_purchase",
			ExternalId:      idempotencyKey,
		})
		if chargeErr != nil {
			h.respondBillingError(c, chargeErr)
			return
		}
		chargeTxID = charge.GetTransactionId()
		chargedAmount = charge.GetChargedAmount()
	}

	// authToken and headProjectID are captured now because the two slow steps
	// (full schema/data copy and the GitLab fork+publish) run in a background
	// goroutine after this handler returns. The gin.Context — and its headers —
	// must not be read once the request is done.
	authToken := c.GetHeader("Authorization")
	headProjectID := projectId.(string)

	// provisionSync runs only the fast, identity-establishing steps: create the
	// new project, environment, resource, API key, MCP row and chat. It stops
	// before copyUgenTemplateData and publishTemplateMicrofrontend, which together
	// take minutes (GitLab fork+import alone is 30-90s) and previously blew past
	// Cloudflare's ~100s edge timeout (HTTP 524). Those run in the background below.
	provisionSync := func() (*templateProvisionResult, error) {
		targetProject, err := h.companyServices.Project().Create(
			ctx, &pb.CreateProjectRequest{
				Title:        sanitizeProjectNameForBackend(projectName),
				CompanyId:    headProject.GetCompanyId(),
				K8SNamespace: headProject.GetK8SNamespace(),
				// Carry the template's per-user price onto the new project so that
				// adding users later charges the head balance (in the template's
				// currency) instead of enforcing the fare user limit. 0 = free.
				PerUserPrice:      tmpl.GetPerUserPrice(),
				PerUserCurrencyId: tmpl.GetCurrencyId(),
			},
		)
		if err != nil {
			return nil, fmt.Errorf("create target project: %v", err)
		}

		targetEnv, err := h.companyServices.Environment().CreateV2(
			ctx, &pb.CreateEnvironmentRequest{
				CompanyId:    headProject.GetCompanyId(),
				ProjectId:    targetProject.GetProjectId(),
				UserId:       authInfo.GetUserIdAuth(),
				ClientTypeId: authInfo.GetClientTypeId(),
				RoleId:       authInfo.GetRoleId(),
				Name:         "Production",
				DisplayColor: "#00FF00",
				Description:  "Production Environment",
			},
		)
		if err != nil {
			return nil, fmt.Errorf("create target env: %v", err)
		}

		targetResource, err := h.companyServices.ServiceResource().GetSingle(
			ctx,
			&pb.GetSingleServiceResourceReq{
				ProjectId:     targetProject.GetProjectId(),
				EnvironmentId: targetEnv.GetId(),
				ServiceType:   pb.ServiceType_BUILDER_SERVICE,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("get target resource: %v", err)
		}

		targetService, err := h.GetProjectSrvc(ctx, targetProject.GetProjectId(), targetResource.GetNodeType())
		if err != nil {
			return nil, fmt.Errorf("get target project service: %v", err)
		}

		apiKeys, err := h.authService.ApiKey().GetList(
			ctx, &as.GetListReq{
				EnvironmentId: targetEnv.GetId(),
				ProjectId:     targetProject.GetProjectId(),
				Limit:         1,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("get api keys: %v", err)
		}
		var apiKey string
		if len(apiKeys.GetData()) > 0 {
			apiKey = apiKeys.GetData()[0].GetAppId()
		}

		// Created "provisioning" — the background job flips it to "ready" once the
		// schema/data copy and microfrontend publish finish (or "error" on failure).
		newMcpProject, err := mainService.GoObjectBuilderService().McpProject().CreateMcpProject(
			ctx, &pbo.CreateMcpProjectReqeust{
				ResourceEnvId:  mainResourceEnvID,
				Title:          projectName,
				Description:    "Created from template: " + tmpl.GetName(),
				ProjectFiles:   sourceMcpFiles,
				ProjectEnv:     sourceMcp.GetProjectEnv(),
				UcodeProjectId: targetProject.GetProjectId(),
				ApiKey:         apiKey,
				EnvironmentId:  targetEnv.GetId(),
				Status:         "provisioning",
				AppVisibility:  sourceMcp.GetAppVisibility(),
				ProjectType:    sourceMcp.GetProjectType(),
			},
		)
		if err != nil {
			return nil, fmt.Errorf("create mcp project: %v", err)
		}

		chat, err := mainService.GoObjectBuilderService().AiChat().CreateChat(
			ctx,
			&pbo.CreateChatRequest{
				ResourceEnvId: mainResourceEnvID,
				ProjectId:     newMcpProject.GetId(),
				Title:         projectName,
				Description:   "Created from template: " + tmpl.GetName(),
				Type:          "ugen",
			},
		)
		if err != nil {
			return nil, fmt.Errorf("create template project chat: %v", err)
		}

		// Point the cloned frontend at THIS project's backend, not the template
		// source. .env / .env.* are protected files copied verbatim, so they carry
		// the source's VITE_X_API_KEY and project/env ids — the cloned app would
		// otherwise read and write the SOURCE's data. Rewrite them with the new
		// project's identity and persist the corrected .env into the stored files.
		rewrittenFiles := rewriteTemplateEnvFiles(sourceMcpFiles, apiKey, targetProject.GetProjectId(), newMcpProject.GetId(), targetEnv.GetId())
		if envFiles := filterTemplateEnvFiles(rewrittenFiles); len(envFiles) > 0 {
			if _, err = mainService.GoObjectBuilderService().McpProject().UpdateMcpProject(ctx, &pbo.McpProject{
				ResourceEnvId: mainResourceEnvID,
				Id:            newMcpProject.GetId(),
				ProjectFiles:  envFiles,
			}); err != nil {
				return nil, fmt.Errorf("rewrite mcp env files: %v", err)
			}
		}

		targetProjectData := &models.ProjectData{
			McpProjectId:   newMcpProject.GetId(),
			UcodeProjectId: targetProject.GetProjectId(),
			ApiKey:         apiKey,
			EnvironmentId:  targetEnv.GetId(),
			ResourceEnvId:  targetResource.GetResourceEnvironmentId(),
			NodeType:       targetResource.GetNodeType(),
			ResourceType:   int32(targetResource.GetResourceType()),
		}

		return &templateProvisionResult{
			targetService: targetService,
			data:          targetProjectData,
			mcpID:         newMcpProject.GetId(),
			publishFiles:  rewrittenFiles,
			response: gin.H{
				"project_id":           targetProject.GetProjectId(),
				"ucode_project_id":     targetProject.GetProjectId(),
				"environment_id":       targetEnv.GetId(),
				"mcp_project_id":       newMcpProject.GetId(),
				"chat_id":              chat.GetId(),
				"api_key":              apiKey,
				"resource_env_id":      targetProjectData.ResourceEnvId,
				"main_resource_env_id": mainResourceEnvID,
				// Provisioning still running in the background; the microfrontend
				// refs are filled in on the MCP row once publish completes. Poll
				// GET /v1/mcp_project/:id and watch `status` flip ready/error.
				"status":                     "provisioning",
				"template_preview_url":       tmpl.GetPreviewUrl(),
				"source_mcp_project_id":      tmpl.GetMcpProjectId(),
				"source_resource_env_id":     tmpl.GetSourceResourceEnvId(),
				"source_mcp_resource_env_id": tmpl.GetSourceMcpResourceEnvId(),
				"source_repo_id":             tmpl.GetSourceRepoId(),
				"source_function_id":         tmpl.GetSourceFunctionId(),
			},
		}, nil
	}

	prov, err := provisionSync()
	if err != nil {
		// The fast steps failed before we ever returned to the client. Compensate
		// the charge (if any) so the user never pays for a project that was not
		// created. Best-effort — a failed refund is logged for reconciliation.
		if chargeTxID != "" {
			if _, refundErr := h.companyServices.Billing().RefundProjectBalance(ctx, &pb.RefundProjectBalanceRequest{
				ProjectId:     headProjectID,
				TransactionId: chargeTxID,
				Comment:       "refund: template provisioning failed",
			}); refundErr != nil {
				log.Printf("[ugen-template] REFUND FAILED project=%s charge_tx=%s: %v", headProjectID, chargeTxID, refundErr)
			}
		}
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	// Background: the two slow steps. On success mark the MCP row ready with its
	// microfrontend refs; on failure mark it errored and refund. ctx is
	// context.Background() (declared above), so client disconnect never cancels it.
	go func(res *templateProvisionResult) {
		bgErr := func() error {
			if err := h.copyUgenTemplateData(ctx, sourceService, res.targetService, tmpl.GetSourceResourceEnvId(), res.data.ResourceEnvId, res.data.UcodeProjectId); err != nil {
				return fmt.Errorf("copy template data: %w", err)
			}

			published, err := h.publishTemplateMicrofrontend(ctx, projectName, res.publishFiles, res.data, mainResourceEnvID, authToken)
			if err != nil {
				return fmt.Errorf("publish template microfrontend: %w", err)
			}

			if _, err := mainService.GoObjectBuilderService().McpProject().UpdateMcpProject(ctx, &pbo.McpProject{
				ResourceEnvId:       mainResourceEnvID,
				Id:                  res.mcpID,
				MicrofrontendId:     published.Data.ID,
				MicrofrontendRepoId: published.Data.RepoId,
				MicrofrontendBranch: published.Data.Branch,
				MicrofrontendUrl:    published.Data.Url,
				Status:              "ready",
			}); err != nil {
				return fmt.Errorf("save template microfrontend refs: %w", err)
			}
			return nil
		}()

		if bgErr != nil {
			log.Printf("[ugen-template] async provisioning FAILED mcp=%s project=%s: %v", res.mcpID, res.data.UcodeProjectId, bgErr)
			if _, err := mainService.GoObjectBuilderService().McpProject().UpdateMcpProject(ctx, &pbo.McpProject{
				ResourceEnvId: mainResourceEnvID,
				Id:            res.mcpID,
				Status:        "error",
			}); err != nil {
				log.Printf("[ugen-template] mark mcp errored FAILED mcp=%s: %v", res.mcpID, err)
			}
			if chargeTxID != "" {
				if _, refundErr := h.companyServices.Billing().RefundProjectBalance(ctx, &pb.RefundProjectBalanceRequest{
					ProjectId:     headProjectID,
					TransactionId: chargeTxID,
					Comment:       "refund: template provisioning failed",
				}); refundErr != nil {
					log.Printf("[ugen-template] REFUND FAILED project=%s charge_tx=%s: %v", headProjectID, chargeTxID, refundErr)
				}
			}
		}
	}(prov)

	if chargeTxID != "" {
		prov.response["charged_amount"] = chargedAmount
		prov.response["charge_transaction_id"] = chargeTxID
	}
	h.HandleResponse(c, status_http.OK, prov.response)
}

// templateProvisionResult carries the fast-path outcome of CreateProjectFromTemplate
// from the synchronous provisioning closure to the background copy/publish job.
type templateProvisionResult struct {
	targetService servicepkg.ServiceManagerI
	data          *models.ProjectData
	mcpID         string
	// publishFiles is sourceMcpFiles with .env rewritten for the new project.
	publishFiles []*pbo.McpProjectFiles
	response     gin.H
}

// templateEnvRewriteKeys maps the .env variables that must carry the new
// project's identity so the cloned frontend talks to its own backend instead of
// the template source. Base URLs (VITE_*_BASE_URL) are platform-wide and kept.
var templateEnvRewriteKeys = []string{
	"VITE_X_API_KEY",
	"VITE_UCODE_PROJECT_ID",
	"VITE_PROJECT_ID",
	"VITE_ENVIRONMENT_ID",
}

// isTemplateEnvFile reports whether a file path is a dotenv file (.env, .env.*).
func isTemplateEnvFile(path string) bool {
	base := path
	if idx := strings.LastIndexByte(path, '/'); idx >= 0 {
		base = path[idx+1:]
	}
	return base == ".env" || strings.HasPrefix(base, ".env.")
}

// rewriteTemplateEnvFiles returns files with every dotenv file's project-identity
// variables (VITE_X_API_KEY / VITE_UCODE_PROJECT_ID / VITE_PROJECT_ID /
// VITE_ENVIRONMENT_ID) replaced by the new project's values. Non-env files and
// unrelated lines pass through untouched. Only existing keys are replaced; a
// missing key is not appended.
func rewriteTemplateEnvFiles(files []*pbo.McpProjectFiles, apiKey, ucodeProjectID, mcpProjectID, environmentID string) []*pbo.McpProjectFiles {
	replacements := map[string]string{
		"VITE_X_API_KEY":        apiKey,
		"VITE_UCODE_PROJECT_ID": ucodeProjectID,
		"VITE_PROJECT_ID":       mcpProjectID,
		"VITE_ENVIRONMENT_ID":   environmentID,
	}
	out := make([]*pbo.McpProjectFiles, 0, len(files))
	for _, f := range files {
		if f == nil || !isTemplateEnvFile(f.GetPath()) {
			out = append(out, f)
			continue
		}
		lines := strings.Split(f.GetContent(), "\n")
		for i, line := range lines {
			trimmed := strings.TrimLeft(line, " \t")
			for _, key := range templateEnvRewriteKeys {
				val := replacements[key]
				if val == "" {
					continue
				}
				if strings.HasPrefix(trimmed, key+"=") {
					lines[i] = key + "=" + val
					break
				}
			}
		}
		out = append(out, &pbo.McpProjectFiles{
			Path:    f.GetPath(),
			Content: strings.Join(lines, "\n"),
		})
	}
	return out
}

// filterTemplateEnvFiles returns only the dotenv files from a file list.
func filterTemplateEnvFiles(files []*pbo.McpProjectFiles) []*pbo.McpProjectFiles {
	out := make([]*pbo.McpProjectFiles, 0, 4)
	for _, f := range files {
		if f != nil && isTemplateEnvFile(f.GetPath()) {
			out = append(out, f)
		}
	}
	return out
}

// respondBillingError maps the billing service's gRPC status codes to the HTTP
// codes the UI expects: insufficient funds -> 402, missing project/template ->
// 404, bad input -> 400, everything else -> generic gRPC error.
func (h *HandlerV1) respondBillingError(c *gin.Context, err error) {
	st, ok := status.FromError(err)
	if !ok {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	switch st.Code() {
	case codes.NotFound:
		h.HandleResponse(c, status_http.NotFound, st.Message())
	case codes.FailedPrecondition:
		h.HandleResponse(c, status_http.PaymentRequired, st.Message())
	case codes.InvalidArgument:
		h.HandleResponse(c, status_http.InvalidArgument, st.Message())
	default:
		h.HandleResponse(c, status_http.GRPCError, err.Error())
	}
}

type templateMicrofrontendFilesResponse struct {
	Data struct {
		Files []struct {
			Path     string `json:"path"`
			FilePath string `json:"file_path"`
			Content  string `json:"content"`
		} `json:"files"`
	} `json:"data"`
}

func (h *HandlerV1) getTemplateMicrofrontendFiles(ctx context.Context, sourceService servicepkg.ServiceManagerI, tmpl *pb.UgenTemplate, sourceMcpResourceEnvID, authToken string) ([]*pbo.McpProjectFiles, error) {
	repoID := tmpl.GetSourceRepoId()
	if repoID == "" && tmpl.GetSourceFunctionId() != "" {
		function, err := sourceService.GoObjectBuilderService().Function().GetSingle(ctx, &pbo.FunctionPrimaryKey{
			Id:        tmpl.GetSourceFunctionId(),
			ProjectId: sourceMcpResourceEnvID,
		})
		if err != nil {
			return nil, fmt.Errorf("get template source function: %w", err)
		}
		repoID = function.GetRepoId()
	}
	if repoID == "" {
		return nil, fmt.Errorf("template source has no project_files and no source_repo_id")
	}

	filesURL := h.baseConf.GoFunctionServiceHost + h.baseConf.GoFunctionServiceHTTPPort +
		"/v2/functions/micro-frontend/files?repo_id=" + url.QueryEscape(repoID)

	filesReq, err := http.NewRequestWithContext(ctx, http.MethodGet, filesURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build template files request: %w", err)
	}
	filesReq.Header.Set("Authorization", authToken)

	httpClient := &http.Client{Timeout: 2 * time.Minute}
	filesResp, err := httpClient.Do(filesReq)
	if err != nil {
		return nil, fmt.Errorf("fetch template u-gen files: %w", err)
	}
	defer filesResp.Body.Close()

	filesRespBytes, err := io.ReadAll(filesResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read template u-gen files response: %w", err)
	}
	if filesResp.StatusCode >= 400 {
		return nil, fmt.Errorf("fetch template u-gen files returned %d: %s", filesResp.StatusCode, string(filesRespBytes))
	}

	var result templateMicrofrontendFilesResponse
	if err = json.Unmarshal(filesRespBytes, &result); err != nil {
		return nil, fmt.Errorf("parse template u-gen files response: %w", err)
	}

	files := make([]*pbo.McpProjectFiles, 0, len(result.Data.Files))
	for _, file := range result.Data.Files {
		filePath := file.Path
		if filePath == "" {
			filePath = file.FilePath
		}
		if filePath == "" {
			continue
		}
		files = append(files, &pbo.McpProjectFiles{
			Path: filePath,
			// Strip NUL/control bytes here: these files are mirrored into the new
			// project's mcp project_files (Postgres text), which rejects 0x00 with
			// SQLSTATE 22021. Same sanitizer as the microfrontend publish path.
			Content: sanitizeFileContent(file.Content),
		})
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("template source repo %s has no microfrontend files", repoID)
	}

	log.Printf("[ugen-template] loaded %d source files from repo_id=%s", len(files), repoID)
	return files, nil
}

func (h *HandlerV1) copyUgenTemplateData(ctx context.Context, sourceService, targetService servicepkg.ServiceManagerI, sourceResourceEnvID, targetResourceEnvID, targetUcodeProjectID string) error {
	tablesResp, err := sourceService.GoObjectBuilderService().Table().GetAll(ctx, &pbo.GetAllTablesRequest{
		ProjectId: sourceResourceEnvID,
		Limit:     1000,
		Offset:    0,
	})
	if err != nil {
		return fmt.Errorf("get source tables: %w", err)
	}

	for _, table := range tablesResp.GetTables() {
		if skipUgenTemplateTable(table.GetSlug()) {
			continue
		}
		createTable, err := convert[*pbo.Table, *pbo.CreateTableRequest](table)
		if err != nil {
			return fmt.Errorf("convert table %s: %w", table.GetSlug(), err)
		}
		createTable.ProjectId = targetResourceEnvID
		createTable.EnvId = targetResourceEnvID
		// UcodeProjectId is the company-service project id of the NEW project; the
		// object-builder needs it to resolve fare/table-limit info. Without it (or
		// with the source project's id copied by convert) CreateTable fails with
		// "error getting project info".
		createTable.UcodeProjectId = targetUcodeProjectID
		if createTable.Id == "" {
			createTable.Id = table.GetId()
		}
		if _, err = targetService.GoObjectBuilderService().Table().Create(ctx, createTable); err != nil {
			return fmt.Errorf("create table %s: %w", table.GetSlug(), err)
		}
	}

	if err = h.copyTemplateMenus(ctx, sourceService, targetService, sourceResourceEnvID, targetResourceEnvID); err != nil {
		return err
	}

	// Phase 1 — schema: copy fields + relations for EVERY table before any
	// layout is copied. A table's detail-page layout has relation-tabs that FK
	// to relation(id), and those relations frequently belong to a different
	// table (e.g. the "companies" detail page shows related "deals", whose
	// relation has table_from="deals"). Copying layouts interleaved per table
	// therefore hit view_relation_permission_relation_id_fkey / tab_relation_id_fkey
	// whenever the referenced relation had not been created yet. Relation IDs are
	// preserved on copy (Relation.Create keeps the supplied id), so we record the
	// ones we actually create and use that set to prune dangling layout tabs below.
	createdRelationIDs := make(map[string]bool)
	for _, table := range tablesResp.GetTables() {
		if skipUgenTemplateTable(table.GetSlug()) {
			continue
		}
		if err = h.copyTemplateTableSchema(ctx, sourceService, targetService, sourceResourceEnvID, targetResourceEnvID, table, createdRelationIDs); err != nil {
			return err
		}
	}

	// Phase 2 — presentation: copy layouts / views / custom events now that every
	// relation exists. Layout relation-tabs whose relation was skipped (e.g. a
	// login table's relation to role/client_type) are dropped so the copy cannot
	// reference a relation that does not exist in the target.
	for _, table := range tablesResp.GetTables() {
		if skipUgenTemplateTable(table.GetSlug()) {
			continue
		}
		if err = h.copyTemplateTableViews(ctx, sourceService, targetService, sourceResourceEnvID, targetResourceEnvID, table, createdRelationIDs); err != nil {
			return err
		}
	}

	// Rows: copy the template's sample data. Every table's rows are copied — both
	// business/custom tables AND the default ucode tables (role, client_type,
	// person, sms_template) — so a clone comes with the template's sample data.
	// Login tables are skipped inside copyTemplateRows because their rows carry
	// auth references (user_id_auth/client_type_id/role_id) that would violate the
	// target's foreign keys.
	pending := make([]*pbo.Table, 0, len(tablesResp.GetTables()))
	for _, table := range tablesResp.GetTables() {
		pending = append(pending, table)
	}
	// Best-effort + FK-tolerant: template tables reference each other and GetAll
	// returns them unordered, so a child row can precede its parent. FK failures
	// are retried until a pass makes no progress; any other error is logged and
	// skipped so one bad row never fails an otherwise-good clone.
	for len(pending) > 0 {
		var failed []*pbo.Table
		for _, table := range pending {
			if rowErr := h.copyTemplateRows(ctx, sourceService, targetService, sourceResourceEnvID, targetResourceEnvID, table); rowErr != nil {
				if isForeignKeyViolation(rowErr) {
					failed = append(failed, table)
					continue
				}
				log.Printf("[ugen-template] skip rows %s: %v", table.GetSlug(), rowErr)
			}
		}
		if len(failed) == len(pending) {
			// Only unresolved FK failures remain and none advanced this pass.
			for _, table := range failed {
				log.Printf("[ugen-template] rows unresolved (FK) %s", table.GetSlug())
			}
			break
		}
		pending = failed
	}

	return nil
}

// copyTemplateTableSchema copies a table's fields and relations. It records the
// id of every relation it creates into createdRelationIDs so the later layout
// pass can drop relation-tabs whose relation was skipped or never copied.
func (h *HandlerV1) copyTemplateTableSchema(ctx context.Context, sourceService, targetService servicepkg.ServiceManagerI, sourceResourceEnvID, targetResourceEnvID string, table *pbo.Table, createdRelationIDs map[string]bool) error {
	fieldsResp, err := sourceService.GoObjectBuilderService().Field().GetAll(ctx, &pbo.GetAllFieldsRequest{
		Limit:     1000,
		Offset:    0,
		TableId:   table.GetId(),
		TableSlug: table.GetSlug(),
		ProjectId: sourceResourceEnvID,
	})
	if err != nil {
		return fmt.Errorf("get fields for %s: %w", table.GetSlug(), err)
	}
	for _, field := range fieldsResp.GetFields() {
		if skipUgenTemplateField(field.GetSlug(), field.GetType(), table.GetIsLoginTable()) {
			continue
		}
		createField, err := convert[*pbo.Field, *pbo.CreateFieldRequest](field)
		if err != nil {
			return fmt.Errorf("convert field %s.%s: %w", table.GetSlug(), field.GetSlug(), err)
		}
		createField.ProjectId = targetResourceEnvID
		createField.EnvId = targetResourceEnvID
		createField.TableId = table.GetId()
		if _, err = targetService.GoObjectBuilderService().Field().Create(ctx, createField); err != nil {
			return fmt.Errorf("create field %s.%s: %w", table.GetSlug(), field.GetSlug(), err)
		}
	}

	relationsResp, err := sourceService.GoObjectBuilderService().Relation().GetRelationsByTableFrom(ctx, &pbo.GetRelationsByTableFromRequest{
		TableFrom: table.GetSlug(),
		ProjectId: sourceResourceEnvID,
	})
	if err != nil {
		return fmt.Errorf("get relations for %s: %w", table.GetSlug(), err)
	}
	// A relation's attributes (label, format: RELATION, icon, table_to, ...) live on
	// the LOOKUP field row, not on the relation row — the "relation" table has no
	// attributes column, so GetRelationsByTableFrom can never return them and the
	// clone would recreate every relation with attributes = {} (LOOKUP fields then
	// show "FROM deals TO companies" with no table_to for the frontend). Rebuild the
	// relation_id -> attributes index from the source fields (already loaded above)
	// and reattach it before Create.
	sourceRelationAttrs := make(map[string]*structpb.Struct, len(fieldsResp.GetFields()))
	for _, field := range fieldsResp.GetFields() {
		if field.GetRelationId() != "" && field.GetAttributes() != nil {
			sourceRelationAttrs[field.GetRelationId()] = field.GetAttributes()
		}
	}
	for _, relation := range relationsResp.GetRelations() {
		if skipUgenTemplateRelation(table, relation) {
			continue
		}
		relation.ProjectId = targetResourceEnvID
		relation.EnvId = targetResourceEnvID
		relation.RelationFieldId = uuid.NewString()
		relation.RelationToFieldId = uuid.NewString()
		if attrs, ok := sourceRelationAttrs[relation.GetId()]; ok {
			relation.Attributes = attrs
		}
		if relation.Attributes == nil {
			relation.Attributes, _ = helper.ConvertMapToStruct(map[string]any{})
		}
		if _, err = targetService.GoObjectBuilderService().Relation().Create(ctx, relation); err != nil {
			return fmt.Errorf("create relation %s -> %s: %w", relation.GetTableFrom(), relation.GetTableTo(), err)
		}
		if relation.GetId() != "" {
			createdRelationIDs[relation.GetId()] = true
		}
	}

	return nil
}

// copyTemplateTableViews copies a table's layouts, views and custom events. It
// must run only after every table's relations exist (see copyTemplateTableSchema)
// because layout relation-tabs and their view_relation_permission rows FK to
// relation(id). createdRelationIDs is the set of relations actually created; any
// layout relation-tab outside it is pruned so the copy cannot violate
// tab_relation_id_fkey / view_relation_permission_relation_id_fkey.
func (h *HandlerV1) copyTemplateTableViews(ctx context.Context, sourceService, targetService servicepkg.ServiceManagerI, sourceResourceEnvID, targetResourceEnvID string, table *pbo.Table, createdRelationIDs map[string]bool) error {
	layoutsResp, err := sourceService.GoObjectBuilderService().Layout().GetLayoutByTableID(ctx, &pbo.GetLayoutByTableIDRequest{
		TableId:   table.GetId(),
		ProjectId: sourceResourceEnvID,
	})
	if err != nil {
		return fmt.Errorf("get layouts for %s: %w", table.GetSlug(), err)
	}
	for _, layout := range layoutsResp.GetLayouts() {
		layoutReq, err := convert[*pbo.LayoutResponse, *pbo.LayoutRequest](layout)
		if err != nil {
			return fmt.Errorf("convert layout %s: %w", layout.GetId(), err)
		}
		layoutReq.ProjectId = targetResourceEnvID
		layoutReq.EnvId = targetResourceEnvID
		layoutReq.TableId = table.GetId()
		layoutReq.WithoutResponse = true
		layoutReq.Tabs = pruneDanglingRelationTabs(layoutReq.GetTabs(), createdRelationIDs)
		if _, err = targetService.GoObjectBuilderService().Layout().Update(ctx, layoutReq); err != nil {
			return fmt.Errorf("create layout %s: %w", layout.GetId(), err)
		}
	}

	// Views belong to a table via their table_slug. View.GetList filters by
	// req.MenuId (a non-UUID value is matched against table_slug), NOT req.TableSlug.
	// The previous call set only TableSlug, so GetList fell through to
	// `WHERE table_slug = ''` and returned the project's orphan (empty table_slug)
	// views — then re-created every one of them once per table. With a source that
	// carried ~1200 orphan views that exploded the target `view` table ~27x (≈32k
	// inert rows the UI never renders). Scope the fetch to this table via MenuId and
	// still guard client-side so an orphan or cross-table view is never copied.
	viewsResp, err := sourceService.GoObjectBuilderService().View().GetList(ctx, &pbo.GetAllViewsRequest{
		MenuId:    table.GetSlug(),
		TableSlug: table.GetSlug(),
		ProjectId: sourceResourceEnvID,
	})
	if err != nil {
		return fmt.Errorf("get views for %s: %w", table.GetSlug(), err)
	}
	// Skip views the target already has: Table().Create seeds default TABLE/SECTION
	// views for every table, so re-copying would duplicate them and the count would
	// creep up every clone generation. Dedupe on (type, name) within the table.
	viewSig := func(v *pbo.View) string { return v.GetType() + "\x1f" + v.GetName() }
	seenViews := make(map[string]bool)
	if existingViews, exErr := targetService.GoObjectBuilderService().View().GetList(ctx, &pbo.GetAllViewsRequest{
		MenuId:    table.GetSlug(),
		TableSlug: table.GetSlug(),
		ProjectId: targetResourceEnvID,
	}); exErr != nil {
		log.Printf("[ugen-template] read target views %s: %v", table.GetSlug(), exErr)
	} else {
		for _, v := range existingViews.GetViews() {
			seenViews[viewSig(v)] = true
		}
	}
	// Views are presentation-only, and a template's source view data can be partly
	// broken (missing relation/menu refs). Copy best-effort so one bad view never
	// aborts an otherwise-good clone.
	for _, view := range viewsResp.GetViews() {
		if view.GetTableSlug() != table.GetSlug() {
			continue
		}
		if seenViews[viewSig(view)] {
			continue
		}
		seenViews[viewSig(view)] = true
		viewReq, err := convert[*pbo.View, *pbo.CreateViewRequest](view)
		if err != nil {
			log.Printf("[ugen-template] skip view %s (%s): %v", view.GetId(), table.GetSlug(), err)
			continue
		}
		viewReq.ProjectId = targetResourceEnvID
		viewReq.EnvId = targetResourceEnvID
		viewReq.Id = ""
		if _, err = targetService.GoObjectBuilderService().View().Create(ctx, viewReq); err != nil {
			log.Printf("[ugen-template] skip view %s (%s): %v", view.GetId(), table.GetSlug(), err)
			continue
		}
	}

	eventsResp, err := sourceService.GoObjectBuilderService().CustomEvent().GetList(ctx, &pbo.GetCustomEventsListRequest{
		TableSlug: table.GetSlug(),
		ProjectId: sourceResourceEnvID,
	})
	if err != nil {
		return fmt.Errorf("get custom events for %s: %w", table.GetSlug(), err)
	}
	for _, event := range eventsResp.GetCustomEvents() {
		_, err = targetService.GoObjectBuilderService().CustomEvent().Create(ctx, &pbo.CreateCustomEventRequest{
			TableSlug:  table.GetSlug(),
			Icon:       event.GetIcon(),
			EventPath:  event.GetEventPath(),
			Label:      event.GetLabel(),
			Url:        event.GetUrl(),
			Disable:    event.GetDisable(),
			ProjectId:  targetResourceEnvID,
			Method:     event.GetMethod(),
			ActionType: event.GetActionType(),
			Attributes: event.GetAttributes(),
			EnvId:      targetResourceEnvID,
			Path:       event.GetPath(),
		})
		if err != nil {
			return fmt.Errorf("create custom event %s: %w", event.GetLabel(), err)
		}
	}

	return nil
}

// pruneDanglingRelationTabs removes a layout's relation-tabs whose relation was
// not copied into the target (skipped relations such as a login table's
// role/client_type links, or relations that live only on skipped tables). Both
// tab.relation_id and view_relation_permission.relation_id FK to relation(id),
// so a dangling reference would abort the whole Layout.Update with SQLSTATE
// 23503. Non-relation tabs are kept but have any stray relation reference
// cleared so they still render their sections.
func pruneDanglingRelationTabs(tabs []*pbo.TabRequest, createdRelationIDs map[string]bool) []*pbo.TabRequest {
	if len(tabs) == 0 {
		return tabs
	}
	kept := make([]*pbo.TabRequest, 0, len(tabs))
	for _, tab := range tabs {
		relID := tab.GetRelationId()
		if relID != "" && !createdRelationIDs[relID] {
			if tab.GetType() == "relation" {
				continue
			}
			tab.RelationId = ""
		}
		kept = append(kept, tab)
	}
	return kept
}

// isForeignKeyViolation reports whether err is a PostgreSQL foreign-key violation
// (SQLSTATE 23503), as surfaced through the object-builder gRPC error string.
func isForeignKeyViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23503") || strings.Contains(msg, "foreign key constraint")
}

// copyTemplateRows copies every row of a source table into the target so a clone
// includes the template's sample data. Login tables are skipped — their rows
// carry auth references that would violate the target's foreign keys.
func (h *HandlerV1) copyTemplateRows(ctx context.Context, sourceService, targetService servicepkg.ServiceManagerI, sourceResourceEnvID, targetResourceEnvID string, table *pbo.Table) error {
	if table.GetIsLoginTable() {
		return nil
	}

	tableSlug := table.GetSlug()
	listData, err := helper.ConvertMapToStruct(map[string]any{
		"limit":  1000,
		"offset": 0,
	})
	if err != nil {
		return err
	}
	rows, err := sourceService.GoObjectBuilderService().ObjectBuilder().GetList2(ctx, &pbo.CommonMessage{
		TableSlug: tableSlug,
		Data:      listData,
		ProjectId: sourceResourceEnvID,
	})
	if err != nil {
		return fmt.Errorf("get rows for %s: %w", tableSlug, err)
	}
	rowsMap, err := convertAnyStruct(rows.Data)
	if err != nil {
		return fmt.Errorf("convert rows for %s: %w", tableSlug, err)
	}
	rowList := cast.ToSlice(rowsMap["response"])
	if len(rowList) == 0 {
		return nil
	}
	objects := make([]map[string]any, 0, len(rowList))
	for _, row := range rowList {
		if rowMap, ok := row.(map[string]any); ok {
			objects = append(objects, rowMap)
		}
	}
	if len(objects) == 0 {
		return nil
	}
	fields := make([]string, 0, len(objects[0]))
	for field := range objects[0] {
		fields = append(fields, field)
	}
	upsertData, err := helper.ConvertMapToStruct(map[string]any{
		"objects":    objects,
		"field_slug": "guid",
		"fields":     fields,
	})
	if err != nil {
		return err
	}
	_, err = targetService.GoObjectBuilderService().Items().UpsertMany(ctx, &pbo.CommonMessage{
		TableSlug: tableSlug,
		Data:      upsertData,
		ProjectId: targetResourceEnvID,
	})
	if err != nil {
		return fmt.Errorf("upsert rows for %s: %w", tableSlug, err)
	}
	return nil
}

func convertAnyStruct(s *structpb.Struct) (map[string]any, error) {
	if s == nil {
		return map[string]any{}, nil
	}
	return convert[*structpb.Struct, map[string]any](s)
}

func (h *HandlerV1) copyTemplateMenus(ctx context.Context, sourceService, targetService servicepkg.ServiceManagerI, sourceResourceEnvID, targetResourceEnvID string) error {
	const rootMenuID = "c57eedc3-a954-4262-a0af-376c65b5a284"

	keptTargetMenuIDs, err := h.clearTargetTemplateMenus(ctx, targetService, targetResourceEnvID, rootMenuID)
	if err != nil {
		return err
	}

	tree, err := sourceService.GoObjectBuilderService().Menu().GetMenuTree(ctx, &pbo.MenuPrimaryKey{
		Id:        rootMenuID,
		ProjectId: sourceResourceEnvID,
	})
	if err != nil {
		return fmt.Errorf("get menu tree: %w", err)
	}

	for _, child := range tree.GetChildren() {
		if err = h.copyTemplateMenuTree(ctx, targetService, child, targetResourceEnvID, rootMenuID, keptTargetMenuIDs); err != nil {
			return err
		}
	}
	return nil
}

func (h *HandlerV1) clearTargetTemplateMenus(ctx context.Context, targetService servicepkg.ServiceManagerI, targetResourceEnvID, rootMenuID string) (map[string]bool, error) {
	tree, err := targetService.GoObjectBuilderService().Menu().GetMenuTree(ctx, &pbo.MenuPrimaryKey{
		Id:        rootMenuID,
		ProjectId: targetResourceEnvID,
	})
	if err != nil {
		return nil, fmt.Errorf("get target menu tree: %w", err)
	}

	keptMenuIDs := map[string]bool{
		rootMenuID: true,
	}
	for _, child := range tree.GetChildren() {
		if err = h.clearTargetTemplateMenuTree(ctx, targetService, child, targetResourceEnvID, keptMenuIDs); err != nil {
			return nil, err
		}
	}
	return keptMenuIDs, nil
}

func (h *HandlerV1) clearTargetTemplateMenuTree(ctx context.Context, targetService servicepkg.ServiceManagerI, menu *pbo.MenuTree, targetResourceEnvID string, keptMenuIDs map[string]bool) error {
	for _, child := range menu.GetChildren() {
		if err := h.clearTargetTemplateMenuTree(ctx, targetService, child, targetResourceEnvID, keptMenuIDs); err != nil {
			return err
		}
	}

	if isProtectedUgenTemplateMenu(menu.GetId()) {
		keptMenuIDs[menu.GetId()] = true
		return nil
	}

	if _, err := targetService.GoObjectBuilderService().Menu().Delete(ctx, &pbo.MenuPrimaryKey{
		Id:        menu.GetId(),
		ProjectId: targetResourceEnvID,
		EnvId:     targetResourceEnvID,
	}); err != nil {
		return fmt.Errorf("delete target menu %s: %w", menu.GetLabel(), err)
	}
	return nil
}

func (h *HandlerV1) copyTemplateMenuTree(ctx context.Context, targetService servicepkg.ServiceManagerI, menu *pbo.MenuTree, targetResourceEnvID, parentID string, keptTargetMenuIDs map[string]bool) error {
	if keptTargetMenuIDs[menu.GetId()] {
		for _, child := range menu.GetChildren() {
			if err := h.copyTemplateMenuTree(ctx, targetService, child, targetResourceEnvID, menu.GetId(), keptTargetMenuIDs); err != nil {
				return err
			}
		}
		return nil
	}

	created, err := targetService.GoObjectBuilderService().Menu().Create(ctx, &pbo.CreateMenuRequest{
		Label:           menu.GetLabel(),
		Icon:            menu.GetIcon(),
		Type:            menu.GetType(),
		ProjectId:       targetResourceEnvID,
		ParentId:        parentID,
		MicrofrontendId: "",
		WebpageId:       menu.GetWebpageId(),
		Attributes:      menu.GetAttributes(),
		WikiId:          menu.GetWikiId(),
		IsVisible:       menu.GetIsVisible(),
		EnvId:           targetResourceEnvID,
		TableId:         menu.GetTableId(),
		LayoutId:        menu.GetLayoutId(),
		Id:              menu.GetId(),
	})
	if err != nil {
		return fmt.Errorf("create menu %s: %w", menu.GetLabel(), err)
	}

	for _, child := range menu.GetChildren() {
		if err = h.copyTemplateMenuTree(ctx, targetService, child, targetResourceEnvID, created.GetId(), keptTargetMenuIDs); err != nil {
			return err
		}
	}
	return nil
}

func isProtectedUgenTemplateMenu(id string) bool {
	return protectedUgenTemplateMenuIDs[id]
}

var protectedUgenTemplateMenuIDs = map[string]bool{
	"c57eedc3-a954-4262-a0af-376c65b5a284": true, // root
	"c57eedc3-a954-4262-a0af-376c65b5a282": true, // content
	"c57eedc3-a954-4262-a0af-376c65b5a280": true, // settings
	"c57eedc3-a954-4262-a0af-376c65b5a278": true, // analytics
	"c57eedc3-a954-4262-a0af-376c65b5a276": true, // pivot
	"c57eedc3-a954-4262-a0af-376c65b5a274": true, // report setting
	"7c26b15e-2360-4f17-8539-449c8829003f": true, // saved pivot
	"e96b654a-1692-43ed-89a8-de4d2357d891": true, // history pivot
	"a8de4296-c8c3-48d6-bef0-ee17057733d6": true, // user and permission
	"d1b3b349-4200-4ba9-8d06-70299795d5e6": true, // data
	"f7d1fa7d-b857-4a24-a18c-402345f65df8": true, // code
	"f313614f-f018-4ddc-a0ce-10a1f5716401": true, // resource
	"db4ffda3-7696-4f56-9f1f-be128d82ae68": true, // api
	"3b74ee68-26e3-48c8-bc95-257ca7d6aa5c": true, // profile setting
	"8a6f913a-e3d4-4b73-9fc0-c942f343d0b9": true, // files
	"744d63e6-0ab7-4f16-a588-d9129cf959d1": true, // wiki
	"9e988322-cffd-484c-9ed6-460d8701551b": true, // users
}

func (h *HandlerV1) publishTemplateMicrofrontend(ctx context.Context, projectName string, files []*pbo.McpProjectFiles, target *models.ProjectData, mainResourceEnvID, authToken string) (models.PublishAiMicroFrontendResponse, error) {
	pubFiles := make([]models.GitlabFileChange, 0, len(files))
	for _, file := range files {
		if file.GetPath() == "" {
			continue
		}
		pubFiles = append(pubFiles, models.GitlabFileChange{
			FilePath: file.GetPath(),
			Content:  sanitizeFileContent(file.GetContent()),
		})
	}
	if len(pubFiles) == 0 {
		return models.PublishAiMicroFrontendResponse{}, fmt.Errorf("no microfrontend files to publish")
	}

	publishBody := models.PublishAiMicroFrontendRequest{
		ProjectId:        target.UcodeProjectId,
		EnvironmentId:    target.EnvironmentId,
		Name:             slugify(projectName),
		Path:             uniqueMFEPath(),
		FrameworkType:    "REACT",
		Files:            pubFiles,
		McpProjectId:     target.McpProjectId,
		McpResourceEnvId: mainResourceEnvID,
	}

	pubBytes, err := json.Marshal(publishBody)
	if err != nil {
		return models.PublishAiMicroFrontendResponse{}, fmt.Errorf("marshal publish request: %w", err)
	}

	pubURL := h.baseConf.GoFunctionServiceHost + h.baseConf.GoFunctionServiceHTTPPort +
		"/v2/functions/micro-frontend/publish-ai"

	pubReq, err := http.NewRequestWithContext(ctx, http.MethodPost, pubURL, bytes.NewReader(pubBytes))
	if err != nil {
		return models.PublishAiMicroFrontendResponse{}, fmt.Errorf("build publish request: %w", err)
	}
	pubReq.Header.Set("Content-Type", "application/json")
	pubReq.Header.Set("Authorization", authToken)

	httpClient := &http.Client{Timeout: 2 * time.Minute}
	pubResp, err := httpClient.Do(pubReq)
	if err != nil {
		return models.PublishAiMicroFrontendResponse{}, fmt.Errorf("publish-ai call: %w", err)
	}
	defer pubResp.Body.Close()

	pubRespBytes, err := io.ReadAll(pubResp.Body)
	if err != nil {
		return models.PublishAiMicroFrontendResponse{}, fmt.Errorf("read publish-ai response: %w", err)
	}
	if pubResp.StatusCode >= 400 {
		return models.PublishAiMicroFrontendResponse{}, fmt.Errorf("publish-ai returned %d: %s", pubResp.StatusCode, string(pubRespBytes))
	}

	var result models.PublishAiMicroFrontendResponse
	if err = json.Unmarshal(pubRespBytes, &result); err != nil {
		return models.PublishAiMicroFrontendResponse{}, fmt.Errorf("parse publish-ai response: %w", err)
	}
	log.Printf("[ugen-template] published %d files to project %s", len(pubFiles), target.UcodeProjectId)
	return result, nil
}

// isDefaultUcodeTable reports whether slug is one of ucode's default system
// tables that every project already provisions. Their SCHEMA is not re-created
// on template clone (skipUgenTemplateTable), but their ROWS are copied so the new
// project inherits the template's base config (roles, client types, SMS templates).
func isDefaultUcodeTable(slug string) bool {
	switch slug {
	case "role", "client_type", "person", "sms_template":
		return true
	default:
		return false
	}
}

// skipUgenTemplateTable reports whether a table's schema copy should be skipped —
// the default ucode tables already exist in the target project.
func skipUgenTemplateTable(slug string) bool {
	return isDefaultUcodeTable(slug)
}

func skipUgenTemplateField(slug, fieldType string, isLoginTable bool) bool {
	switch slug {
	case "guid", "created_at", "updated_at", "deleted_at", "folder_id", "user_id_auth":
		return true
	}
	if isLoginTable {
		switch slug {
		case "login", "password", "phone", "email", "tin", "last_activity", "client_type_id", "role_id":
			return true
		}
	}
	switch fieldType {
	case "LOOKUP", "LOOKUPS":
		return true
	default:
		return false
	}
}

func skipUgenTemplateRelation(table *pbo.Table, relation *pbo.CreateRelationRequest) bool {
	if !table.GetIsLoginTable() {
		return false
	}
	switch relation.GetTableTo() {
	case "client_type", "role":
		return true
	}
	switch relation.GetFieldFrom() {
	case "client_type_id", "role_id":
		return true
	default:
		return false
	}
}
