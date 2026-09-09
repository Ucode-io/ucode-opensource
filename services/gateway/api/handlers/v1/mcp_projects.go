package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	pbo "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *HandlerV1) GetMcpProjects(c *gin.Context) {
	var (
		projectTitle   = c.Query("title")
		limit          = cast.ToUint32(c.Query("limit"))
		offset         = cast.ToUint32(c.Query("offset"))
		orderBy        = c.Query("order_by")
		orderDirection = c.Query("order_direction")
		ids            = c.QueryArray("ids")
		projectId      any
		environmentId  any
		ok             bool
	)

	if limit == 0 {
		limit = 10
	}

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

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if resource.ResourceType != pb.ResourceType_POSTGRESQL {
		h.HandleResponse(c, status_http.InvalidArgument, "resource type not supported")
		return
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	response, err := services.GoObjectBuilderService().McpProject().GetAllMcpProject(
		c.Request.Context(),
		&pbo.GetMcpProjectListReq{
			ResourceEnvId:  resource.ResourceEnvironmentId,
			Limit:          limit,
			Offset:         offset,
			Title:          projectTitle,
			OrderBy:        orderBy,
			OrderDirection: orderDirection,
			Ids:            ids,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

func (h *HandlerV1) GetMcpProjectFiles(c *gin.Context) {
	var (
		mcpProjectId  = c.Param("mcp_project_id")
		projectId     any
		environmentId any
		ok            bool
	)

	if !util.IsValidUUID(mcpProjectId) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

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

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if resource.ResourceType != pb.ResourceType_POSTGRESQL {
		h.HandleResponse(c, status_http.InvalidArgument, "resource type not supported")
		return
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	response, err := services.GoObjectBuilderService().McpProject().GetMcpProjectFiles(
		c.Request.Context(),
		&pbo.McpProjectId{
			ResourceEnvId: resource.ResourceEnvironmentId,
			Id:            mcpProjectId,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

func (h *HandlerV1) SaveMcpProject(c *gin.Context) {
	var (
		mcpProjectId  = c.Param("mcp_project_id")
		request       pbo.McpProject
		projectId     any
		environmentId any
		ok            bool
	)

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	if !util.IsValidUUID(mcpProjectId) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

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

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if resource.ResourceType != pb.ResourceType_POSTGRESQL {
		h.HandleResponse(c, status_http.InvalidArgument, "resource type not supported")
		return
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	request.ResourceEnvId = resource.ResourceEnvironmentId
	request.Id = mcpProjectId

	response, err := services.GoObjectBuilderService().McpProject().UpdateMcpProject(c.Request.Context(), &request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

func (h *HandlerV1) DeleteMcpProject(c *gin.Context) {
	var (
		mcpProjectId  = c.Param("mcp_project_id")
		projectId     any
		environmentId any
		ok            bool
	)

	if !util.IsValidUUID(mcpProjectId) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

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

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if resource.ResourceType != pb.ResourceType_POSTGRESQL {
		h.HandleResponse(c, status_http.InvalidArgument, "resource type not supported")
		return
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	response, err := services.GoObjectBuilderService().McpProject().DeleteMcpProject(
		c.Request.Context(), &pbo.McpProjectId{
			ResourceEnvId: resource.ResourceEnvironmentId,
			Id:            mcpProjectId,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	// Удаляем short link из company_service и Redis.
	// Используем ucode_project_id из удалённого проекта (если вернулся).
	if ucodeProjectId := response.GetUcodeProjectId(); ucodeProjectId != "" {
		go func() {
			ctx := context.Background()
			if _, delErr := h.companyServices.MfeShortLink().DeleteByProjectId(
				ctx, &pb.MfeShortLinkProjectReq{ProjectId: ucodeProjectId},
			); delErr != nil {
				h.log.Error("delete mfe short link", logger.String("project_id", ucodeProjectId), logger.Error(delErr))
			}
		}()
	}

	h.HandleResponse(c, status_http.OK, response)
}

func (h *HandlerV1) PublishMcpProjectFront(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

func (h *HandlerV1) GetMfeShortLink(c *gin.Context) {
	functionId := c.Param("function_id")
	if !util.IsValidUUID(functionId) {
		h.HandleResponse(c, status_http.InvalidArgument, "function_id must be a valid UUID")
		return
	}

	link, err := h.companyServices.MfeShortLink().GetByFunctionId(
		c.Request.Context(),
		&pb.MfeShortLinkFunctionReq{FunctionId: functionId},
	)
	if err != nil {
		h.HandleResponse(c, status_http.NotFound, "short link not found")
		return
	}

	h.HandleResponse(
		c, status_http.OK,
		map[string]string{
			"short_url":      mfeShortURL(h.baseConf.ShortURLBase, link.GetSlug()),
			"slug":           link.GetSlug(),
			"url":            link.GetUrl(),
			"function_id":    link.GetFunctionId(),
			"mcp_project_id": link.GetMcpProjectId(),
		},
	)
}

// CreateMfeShortLink creates a short link for an existing microfrontend.
// Idempotent: if a short link for this function_id already exists, returns it without creating a new one.
func (h *HandlerV1) CreateMfeShortLink(c *gin.Context) {
	var req models.CreateMfeShortLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	if !util.IsValidUUID(req.FunctionId) {
		h.HandleResponse(c, status_http.InvalidArgument, "function_id must be a valid UUID")
		return
	}
	if !util.IsValidUUID(req.McpProjectId) {
		h.HandleResponse(c, status_http.InvalidArgument, "mcp_project_id must be a valid UUID")
		return
	}
	if req.Url == "" {
		h.HandleResponse(c, status_http.InvalidArgument, "url is required")
		return
	}

	ctx := c.Request.Context()

	// Idempotency: return existing link if already created for this function.
	if existing, err := h.companyServices.MfeShortLink().GetByFunctionId(
		ctx, &pb.MfeShortLinkFunctionReq{FunctionId: req.FunctionId},
	); err == nil && existing.GetSlug() != "" {
		h.HandleResponse(c, status_http.OK, map[string]string{
			"short_url":      mfeShortURL(h.baseConf.ShortURLBase, existing.GetSlug()),
			"slug":           existing.GetSlug(),
			"url":            existing.GetUrl(),
			"function_id":    existing.GetFunctionId(),
			"mcp_project_id": existing.GetMcpProjectId(),
		})
		return
	}

	var link *pb.MfeShortLink
	for range 5 {
		slug, err := generateSlug()
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, "failed to generate slug")
			return
		}
		link, err = h.companyServices.MfeShortLink().Create(ctx, &pb.MfeShortLink{
			Slug:         slug,
			Url:          req.Url,
			ProjectId:    req.ProjectId,
			McpProjectId: req.McpProjectId,
			FunctionId:   req.FunctionId,
		})
		if err == nil {
			break
		}
		if status.Code(err) != codes.AlreadyExists {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		link = nil
	}
	if link == nil {
		h.HandleResponse(c, status_http.GRPCError, "failed to generate unique slug after 5 attempts")
		return
	}

	if h.centralRedis != nil {
		go func() {
			_ = h.centralRedis.Set(context.Background(), mfeShortLinkRedisPrefix+link.GetSlug(), link.GetUrl(), mfeShortLinkRedisTTL).Err()
		}()
	}

	h.HandleResponse(c, status_http.OK, map[string]string{
		"short_url":      mfeShortURL(h.baseConf.ShortURLBase, link.GetSlug()),
		"slug":           link.GetSlug(),
		"url":            link.GetUrl(),
		"function_id":    link.GetFunctionId(),
		"mcp_project_id": link.GetMcpProjectId(),
	})
}

func (h *HandlerV1) ManualSaveMcpProject(c *gin.Context) {
	var (
		mcpProjectId  = c.Param("mcp_project_id")
		request       models.ManualSaveMcpProjectRequest
		projectId     any
		environmentId any
		ok            bool
	)

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	if !util.IsValidUUID(mcpProjectId) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

	if len(request.Files) == 0 {
		h.HandleResponse(c, status_http.InvalidArgument, "files are required")
		return
	}

	if request.RepoID == 0 {
		h.HandleResponse(c, status_http.InvalidArgument, "repo_id is required")
		return
	}

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

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if resource.ResourceType != pb.ResourceType_POSTGRESQL {
		h.HandleResponse(c, status_http.InvalidArgument, "resource type not supported")
		return
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	// Fetch current project to get the full file list.
	current, err := services.GoObjectBuilderService().McpProject().GetMcpProjectFiles(
		c.Request.Context(),
		&pbo.McpProjectId{
			ResourceEnvId: resource.ResourceEnvironmentId,
			Id:            mcpProjectId,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	// Build a lookup of manually edited files.
	editedFiles := make(map[string]string, len(request.Files))
	for _, f := range request.Files {
		editedFiles[f.FilePath] = f.Content
	}

	// Merge: replace content for edited files (clear their file_graph), keep the rest intact.
	mergedFiles := make([]*pbo.McpProjectFiles, 0, len(current.ProjectFiles))
	for _, f := range current.ProjectFiles {
		if newContent, edited := editedFiles[f.Path]; edited {
			mergedFiles = append(mergedFiles, &pbo.McpProjectFiles{
				Path:      f.Path,
				Content:   newContent,
				FileGraph: nil, // cleared — stale AST after manual edit
			})
			delete(editedFiles, f.Path)
		} else {
			mergedFiles = append(mergedFiles, f)
		}
	}
	// Add any new files that didn't exist in the project yet.
	for path, content := range editedFiles {
		mergedFiles = append(mergedFiles, &pbo.McpProjectFiles{
			Path:    path,
			Content: content,
		})
	}

	// Persist merged file list to DB, keeping title/description/env unchanged.
	updated, err := services.GoObjectBuilderService().McpProject().UpdateMcpProject(
		c.Request.Context(),
		&pbo.McpProject{
			Id:            mcpProjectId,
			ResourceEnvId: resource.ResourceEnvironmentId,
			Title:         current.Title,
			Description:   current.Description,
			ProjectFiles:  mergedFiles,
			ProjectEnv:    current.ProjectEnv,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	// Push only the edited files to the u-gen branch via function service.
	commitMsg := request.CommitMessage
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("manual edit: %d file(s) updated", len(request.Files))
	}

	type pushReq struct {
		RepoID        int                       `json:"repo_id"`
		Files         []models.GitlabFileChange `json:"files"`
		CommitMessage string                    `json:"commit_message"`
	}

	pushBody, err := json.Marshal(pushReq{
		RepoID:        request.RepoID,
		Files:         request.Files,
		CommitMessage: commitMsg,
	})
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, "failed to build push request: "+err.Error())
		return
	}

	pushURL := h.baseConf.GoFunctionServiceHost + h.baseConf.GoFunctionServiceHTTPPort +
		"/v2/functions/micro-frontend/push-changes"

	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPut, pushURL, bytes.NewReader(pushBody))
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, "failed to build http request: "+err.Error())
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", c.GetHeader("Authorization"))
	if apiKey := c.GetHeader("X-API-KEY"); apiKey != "" {
		httpReq.Header.Set("X-API-KEY", apiKey)
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, "push-changes call failed: "+err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 400 {
		respBytes, _ := io.ReadAll(httpResp.Body)
		h.HandleResponse(c, status_http.InternalServerError,
			fmt.Sprintf("push-changes returned %d: %s", httpResp.StatusCode, string(respBytes)))
		return
	}

	// Save a full snapshot of the project after a successful push.
	microfrontendId := current.FunctionId
	if microfrontendId == "" {
		microfrontendId = request.MicrofrontendID
	}

	if microfrontendId != "" {
		snapshotFiles := make([]models.GitlabFileChange, 0, len(mergedFiles))
		for _, f := range mergedFiles {
			if !snapshotExcluded(f.Path) {
				snapshotFiles = append(snapshotFiles, models.GitlabFileChange{
					FilePath: f.Path,
					Content:  f.Content,
				})
			}
		}

		go func() {
			filesJSON, err := json.Marshal(snapshotFiles)
			if err != nil {
				log.Printf("[VERSION] manual save: failed to marshal files: %v", err)
				return
			}
			_, err = services.GoObjectBuilderService().MicrofrontendVersions().CreateVersion(
				context.Background(),
				&pbo.CreateMicrofrontendVersionRequest{
					ResourceEnvId:   resource.ResourceEnvironmentId,
					MicrofrontendId: microfrontendId,
					CommitMessage:   commitMsg,
					Files:           string(filesJSON),
				},
			)
			if err != nil {
				log.Printf("[VERSION] manual save: failed to create version: %v", err)
			}
		}()
	}

	h.HandleResponse(c, status_http.OK, updated)
}
