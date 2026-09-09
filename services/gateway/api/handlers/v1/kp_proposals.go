package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

// kpAgentTimeout bounds the synchronous KP generation call. It must stay under
// the ingress proxy timeout in front of the gateway (see deploy notes) and
// comfortably above a typical HTML-only generation.
const kpAgentTimeout = 120 * time.Second

// kpProposalRequest is what the Professio frontend sends. The endpoint is a thin
// pass-through: only prompt/locale/themeTokens reach the agent. Brand/web-research
// behavior is intentionally left at the agent's defaults.
type kpProposalRequest struct {
	Prompt      string          `json:"prompt"`
	Locale      string          `json:"locale"`
	ThemeTokens json.RawMessage `json:"themeTokens,omitempty"`
	DealID      string          `json:"dealId,omitempty"`
}

type kpAgentRequest struct {
	Prompt                 string          `json:"prompt"`
	Locale                 string          `json:"locale,omitempty"`
	ThemeTokens            json.RawMessage `json:"themeTokens,omitempty"`
	PrototypePublicBaseURL string          `json:"prototypePublicBaseUrl,omitempty"`
}

type kpAgentError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type kpAgentResponse struct {
	Ok              bool              `json:"ok"`
	RequestID       string            `json:"requestId"`
	Title           string            `json:"title"`
	HTML            string            `json:"html"`
	PageCount       int               `json:"pageCount"`
	QAStatus        string            `json:"qaStatus"`
	DownloadURL     string            `json:"downloadUrl"`
	HTMLDownloadURL string            `json:"htmlDownloadUrl"`
	PrototypeURL    string            `json:"prototypeUrl"`
	Prototype       *kpAgentPrototype `json:"prototype"`
	Error           *kpAgentError     `json:"error"`
}

type kpAgentPrototype struct {
	URL             string `json:"url"`
	QAStatus        string `json:"qaStatus,omitempty"`
	ScreenCount     int    `json:"screenCount,omitempty"`
	RendererVersion string `json:"rendererVersion,omitempty"`
}

// kpProposalCacheEntry is the tenant/metadata record stored in centralRedis under
// config.KpProposalCachePrefix+requestId, written once by GenerateKpProposal and
// read by every other KP artifact endpoint (GetKpProposal, GetKpProposalHTML,
// DownloadKpProposalPDF) to verify the caller's project/environment owns requestId
// and to know which artifacts actually exist, across gateway replicas.
type kpProposalCacheEntry struct {
	ProjectID     string            `json:"projectId"`
	EnvironmentID string            `json:"environmentId"`
	Title         string            `json:"title"`
	PageCount     int               `json:"pageCount"`
	QAStatus      string            `json:"qaStatus"`
	HasHTML       bool              `json:"hasHtml"`
	HasPDF        bool              `json:"hasPdf"`
	Prototype     *kpAgentPrototype `json:"prototype,omitempty"`
}

// GenerateKpProposal godoc
// @Security ApiKeyAuth
// @ID v1_generate_kp_proposal
// @Router /v1/kp-proposals [POST]
// @Summary Generate a commercial proposal (KP) as HTML
// @Description Synchronously generates a KP via the standalone kp-generator-agent and returns the HTML inline.
// @Tags KP
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer access token"
// @Param GenerateKpProposalRequest body kpProposalRequest true "GenerateKpProposalRequest"
// @Success 200 {object} status_http.Response "KP HTML"
// @Failure 400 {object} status_http.Response "Bad Request"
// @Failure 500 {object} status_http.Response "Server Error"
func (h *HandlerV1) GenerateKpProposal(c *gin.Context) {
	// AuthMiddleware has already validated the JWT and populated the context.
	// A valid, project-scoped token establishes project-level access.
	projectID, environmentID, ok := h.kpTenantFromContext(c)
	if !ok {
		return
	}

	if strings.TrimSpace(h.baseConf.KpAgentURL) == "" {
		h.HandleResponse(c, status_http.InternalServerError, kpError("KP_AGENT_UNAVAILABLE", "KP agent is not configured"))
		return
	}

	var req kpProposalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		h.HandleResponse(c, status_http.InvalidArgument, "prompt is required")
		return
	}

	agentResp, err := h.callKpAgent(c.Request.Context(), kpAgentRequest{
		Prompt:                 req.Prompt,
		Locale:                 mapKpLocale(req.Locale),
		ThemeTokens:            req.ThemeTokens,
		PrototypePublicBaseURL: kpPrototypePublicBaseURL(c),
	})
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, kpError("KP_AGENT_UNAVAILABLE", err.Error()))
		return
	}
	if !agentResp.Ok {
		code, message := "KP_AGENT_FAILED", "KP generation failed"
		if agentResp.Error != nil {
			if agentResp.Error.Code != "" {
				code = agentResp.Error.Code
			}
			if agentResp.Error.Message != "" {
				message = agentResp.Error.Message
			}
		}
		h.HandleResponse(c, status_http.InvalidArgument, kpError(code, message))
		return
	}
	if strings.TrimSpace(agentResp.HTML) == "" {
		h.HandleResponse(c, status_http.InternalServerError, kpError("KP_ARTIFACT_HTML_MISSING", "agent returned empty HTML"))
		return
	}
	prototypeURL := strings.TrimSpace(agentResp.PrototypeURL)
	if prototypeURL == "" && agentResp.Prototype != nil {
		prototypeURL = strings.TrimSpace(agentResp.Prototype.URL)
	}
	if prototypeURL == "" {
		h.HandleResponse(c, status_http.InternalServerError, kpError("KP_ARTIFACT_PROTOTYPE_MISSING", "agent returned no prototype URL"))
		return
	}
	prototype := gin.H{"url": prototypeURL}
	if agentResp.Prototype != nil {
		prototype["qaStatus"] = agentResp.Prototype.QAStatus
		prototype["screenCount"] = agentResp.Prototype.ScreenCount
		prototype["rendererVersion"] = agentResp.Prototype.RendererVersion
	}

	cachedPrototype := agentResp.Prototype
	if cachedPrototype == nil {
		cachedPrototype = &kpAgentPrototype{URL: prototypeURL}
	}

	var htmlURL, pdfURL string
	if h.centralRedis != nil {
		entry, err := json.Marshal(kpProposalCacheEntry{
			ProjectID:     projectID,
			EnvironmentID: environmentID,
			Title:         agentResp.Title,
			PageCount:     agentResp.PageCount,
			QAStatus:      agentResp.QAStatus,
			HasHTML:       agentResp.HTMLDownloadURL != "",
			HasPDF:        agentResp.DownloadURL != "",
			Prototype:     cachedPrototype,
		})
		if err != nil {
			h.log.Error("kp-proposal: failed to marshal proposal cache entry", logger.Error(err))
		} else if err := h.centralRedis.Set(c.Request.Context(), config.KpProposalCachePrefix+agentResp.RequestID, entry, config.KpProposalCacheTTL).Err(); err != nil {
			h.log.Error("kp-proposal: failed to cache proposal tenant/metadata mapping", logger.Error(err))
		} else {
			if agentResp.HTMLDownloadURL != "" {
				htmlURL = "/v1/kp-proposals/" + agentResp.RequestID + "/html"
			}
			if agentResp.DownloadURL != "" {
				pdfURL = "/v1/kp-proposals/" + agentResp.RequestID + "/pdf"
			}
		}
	}

	h.HandleResponse(c, status_http.OK, gin.H{
		"ok":           true,
		"status":       "completed",
		"requestId":    agentResp.RequestID,
		"title":        agentResp.Title,
		"html":         agentResp.HTML,
		"htmlUrl":      htmlURL,
		"pageCount":    agentResp.PageCount,
		"prototypeUrl": prototypeURL,
		"prototype":    prototype,
		"pdfUrl":       pdfURL,
	})
}

// kpTenantFromContext resolves the project/environment scope AuthMiddleware
// already validated onto the gin context, shared by every KP proposal handler
// that needs it (GenerateKpProposal, GetKpProposal, GetKpProposalHTML,
// DownloadKpProposalPDF). On failure it has already written the response.
func (h *HandlerV1) kpTenantFromContext(c *gin.Context) (projectID, environmentID string, ok bool) {
	rawProjectID, exists := c.Get("project_id")
	if !exists || !util.IsValidUUID(cast.ToString(rawProjectID)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return "", "", false
	}
	rawEnvironmentID, exists := c.Get("environment_id")
	if !exists || !util.IsValidUUID(cast.ToString(rawEnvironmentID)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrEnvironmentIdValid)
		return "", "", false
	}
	return cast.ToString(rawProjectID), cast.ToString(rawEnvironmentID), true
}

// loadKpProposalCacheEntry reads the requestId -> tenant/metadata record written
// by GenerateKpProposal. On a miss (unknown, expired, or corrupt entry) it has
// already written a 404 KP_ARTIFACT_NOT_FOUND response — callers only need to
// additionally check tenant ownership against the returned entry.
func (h *HandlerV1) loadKpProposalCacheEntry(c *gin.Context, requestID string) (kpProposalCacheEntry, bool) {
	var entry kpProposalCacheEntry
	notFound := func() (kpProposalCacheEntry, bool) {
		h.HandleResponse(c, status_http.NotFound, kpError("KP_ARTIFACT_NOT_FOUND", "KP artifact not found"))
		return entry, false
	}
	if h.centralRedis == nil {
		return notFound()
	}
	cached, err := h.centralRedis.Get(c.Request.Context(), config.KpProposalCachePrefix+requestID).Bytes()
	if err != nil {
		return notFound()
	}
	if err := json.Unmarshal(cached, &entry); err != nil {
		return notFound()
	}
	return entry, true
}

// GetKpProposal godoc
// @Security ApiKeyAuth
// @ID v1_get_kp_proposal
// @Router /v1/kp-proposals/{requestId} [GET]
// @Summary Get metadata for a previously generated KP
// @Description Durable, tenant-scoped metadata lookup backing the persistent /kp/{requestId} preview URL — survives refresh and works regardless of which gateway replica handled the original POST.
// @Tags KP
// @Param Authorization header string true "Bearer access token"
// @Param requestId path string true "KP request id"
// @Success 200 {object} status_http.Response "KP metadata"
// @Failure 400 {object} status_http.Response "Bad Request"
// @Failure 403 {object} status_http.Response "Forbidden"
// @Failure 404 {object} status_http.Response "Not Found"
func (h *HandlerV1) GetKpProposal(c *gin.Context) {
	projectID, environmentID, ok := h.kpTenantFromContext(c)
	if !ok {
		return
	}

	requestID := c.Param("requestId")
	if !isValidKpRequestID(requestID) {
		h.HandleResponse(c, status_http.InvalidArgument, "invalid requestId format")
		return
	}

	entry, ok := h.loadKpProposalCacheEntry(c, requestID)
	if !ok {
		return
	}
	if entry.ProjectID != projectID || entry.EnvironmentID != environmentID {
		h.HandleResponse(c, status_http.Forbidden, kpError("KP_ARTIFACT_FORBIDDEN", "requestId does not belong to the current project/environment"))
		return
	}

	data := gin.H{
		"ok":        true,
		"status":    "completed",
		"requestId": requestID,
		"title":     entry.Title,
		"pageCount": entry.PageCount,
		"qaStatus":  entry.QAStatus,
		"htmlUrl":   "",
		"pdfUrl":    "",
	}
	if entry.HasHTML {
		data["htmlUrl"] = "/v1/kp-proposals/" + requestID + "/html"
	}
	if entry.HasPDF {
		data["pdfUrl"] = "/v1/kp-proposals/" + requestID + "/pdf"
	}
	if entry.Prototype != nil {
		data["prototype"] = entry.Prototype
	}
	h.HandleResponse(c, status_http.OK, data)
}

// GetKpProposalHTML godoc
// @Security ApiKeyAuth
// @ID v1_get_kp_proposal_html
// @Router /v1/kp-proposals/{requestId}/html [GET]
// @Summary Get the raw HTML rendition of a previously generated KP
// @Description Proxies GET {KP_AGENT_URL}/v1/proposals/{requestId}/html, enforcing tenant ownership. Returns the HTML document directly, no JSON envelope.
// @Tags KP
// @Param Authorization header string true "Bearer access token"
// @Param requestId path string true "KP request id"
// @Success 200 {string} string "HTML document"
// @Failure 400 {object} status_http.Response "Bad Request"
// @Failure 403 {object} status_http.Response "Forbidden"
// @Failure 404 {object} status_http.Response "Not Found"
// @Failure 410 {object} status_http.Response "Gone"
// @Failure 502 {object} status_http.Response "Bad Gateway"
// @Failure 503 {object} status_http.Response "Service Unavailable"
func (h *HandlerV1) GetKpProposalHTML(c *gin.Context) {
	projectID, environmentID, ok := h.kpTenantFromContext(c)
	if !ok {
		return
	}

	requestID := c.Param("requestId")
	if !isValidKpRequestID(requestID) {
		h.HandleResponse(c, status_http.InvalidArgument, "invalid requestId format")
		return
	}

	entry, ok := h.loadKpProposalCacheEntry(c, requestID)
	if !ok {
		return
	}
	if entry.ProjectID != projectID || entry.EnvironmentID != environmentID {
		h.HandleResponse(c, status_http.Forbidden, kpError("KP_ARTIFACT_FORBIDDEN", "requestId does not belong to the current project/environment"))
		return
	}
	if !entry.HasHTML {
		h.HandleResponse(c, status_http.NotFound, kpError("KP_ARTIFACT_NOT_FOUND", "HTML artifact not found"))
		return
	}

	if strings.TrimSpace(h.baseConf.KpAgentURL) == "" {
		h.HandleResponse(c, status_http.ServiceUnavailable, kpError("KP_AGENT_UNAVAILABLE", "KP agent is not configured"))
		return
	}

	agentURL := strings.TrimRight(h.baseConf.KpAgentURL, "/") + "/v1/proposals/" + requestID + "/html"
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, agentURL, nil)
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}
	if h.baseConf.KpAgentAPIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+h.baseConf.KpAgentAPIKey)
	}

	client := &http.Client{Timeout: kpAgentArtifactTimeout}
	res, err := client.Do(httpReq)
	if err != nil {
		h.HandleResponse(c, status_http.BadGateway, kpError("KP_AGENT_UNAVAILABLE", err.Error()))
		return
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
		// handled below
	case http.StatusNotFound:
		h.HandleResponse(c, status_http.NotFound, kpError("KP_ARTIFACT_NOT_FOUND", "HTML artifact not found"))
		return
	case http.StatusGone:
		h.HandleResponse(c, status_http.Gone, kpError("KP_ARTIFACT_GONE", "HTML artifact is no longer available"))
		return
	case http.StatusUnauthorized, http.StatusForbidden:
		h.HandleResponse(c, status_http.BadGateway, kpError("KP_AGENT_AUTH_FAILED", "KP agent rejected the service credential"))
		return
	default:
		h.HandleResponse(c, status_http.BadGateway, kpError("KP_AGENT_UNAVAILABLE", "KP agent returned an unexpected status: "+res.Status))
		return
	}

	if !strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") {
		h.HandleResponse(c, status_http.BadGateway, kpError("KP_ARTIFACT_INVALID", "KP agent did not return HTML"))
		return
	}

	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.DataFromReader(http.StatusOK, res.ContentLength, "text/html; charset=utf-8", res.Body, nil)
}

// GetKpPrototype proxies a published prototype from the internal agent. The
// route is public so links embedded in downloaded PDFs work without an app JWT.
func (h *HandlerV1) GetKpPrototype(c *gin.Context) {
	publicID := strings.TrimSpace(c.Param("publicId"))
	if !isValidKpPrototypeID(publicID) {
		c.JSON(http.StatusNotFound, kpError("KP_PROTOTYPE_NOT_FOUND", "prototype not found"))
		return
	}
	if strings.TrimSpace(h.baseConf.KpAgentURL) == "" {
		c.JSON(http.StatusServiceUnavailable, kpError("KP_AGENT_UNAVAILABLE", "KP agent is not configured"))
		return
	}

	url := strings.TrimRight(h.baseConf.KpAgentURL, "/") + "/p/" + publicID + "/"
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, url, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, kpError("KP_PROTOTYPE_FAILED", err.Error()))
		return
	}
	if h.baseConf.KpAgentAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.baseConf.KpAgentAPIKey)
	}

	res, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, kpError("KP_AGENT_UNAVAILABLE", err.Error()))
		return
	}
	defer res.Body.Close()

	for _, name := range []string{
		"Cache-Control",
		"Content-Security-Policy",
		"Referrer-Policy",
		"X-Content-Type-Options",
		"X-Robots-Tag",
	} {
		if value := res.Header.Get(name); value != "" {
			c.Header(name, value)
		}
	}
	contentType := res.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "text/html; charset=utf-8"
	}
	c.DataFromReader(res.StatusCode, res.ContentLength, contentType, res.Body, nil)
}

func (h *HandlerV1) callKpAgent(ctx context.Context, body kpAgentRequest) (*kpAgentResponse, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(h.baseConf.KpAgentURL, "/") + "/v1/proposals"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if h.baseConf.KpAgentAPIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+h.baseConf.KpAgentAPIKey)
	}

	client := &http.Client{Timeout: kpAgentTimeout}
	res, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var parsed kpAgentResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("kp agent returned non-JSON (status %d): %s", res.StatusCode, truncate(string(raw), 300))
	}
	return &parsed, nil
}

// mapKpLocale maps the app locale to the agent's accepted locale set
// (uz-Latn, ru-RU, en); unknown/empty defaults to ru-RU.
func mapKpLocale(locale string) string {
	switch strings.ToLower(strings.TrimSpace(locale)) {
	case "uz", "uz-latn", "uz_latn":
		return "uz-Latn"
	case "en", "en-us":
		return "en"
	default:
		return "ru-RU"
	}
}

func kpPrototypePublicBaseURL(c *gin.Context) string {
	scheme := firstForwardedValue(c.GetHeader("X-Forwarded-Proto"))
	if scheme != "http" && scheme != "https" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := firstForwardedValue(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = c.Request.Host
	}
	return scheme + "://" + host + "/v1/kp-prototypes"
}

func firstForwardedValue(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.Split(value, ",")[0]))
}

func isValidKpPrototypeID(value string) bool {
	if len(value) < 8 || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func kpError(code, message string) gin.H {
	return gin.H{"ok": false, "error": gin.H{"code": code, "message": message}}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
