package v1

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
)

// ---------------------------------- FacebookAuth --------------------------------------------------

func (h *HandlerV1) FacebookConnect(c *gin.Context) {
	var (
		projectID     any
		environmentID any

		ok bool

		userID string
	)

	projectID, ok = c.Get("project_id")
	if !ok || !util.IsValidUUID(projectID.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentID, ok = c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentID.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "error getting environment id | not valid")
		return
	}

	if h.baseConf.FacebookAppID == "" || h.baseConf.FacebookRedirectURI == "" {
		h.HandleResponse(c, status_http.BadRequest, "facebook integration is not configured")
		return
	}

	if value, ok := c.Get("user_id"); ok {
		userID, _ = value.(string)
	}

	state, err := generateOAuthState()
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	body, err := json.Marshal(
		models.FacebookOAuthState{
			ProjectId:     projectID.(string),
			EnvironmentId: environmentID.(string),
			UserId:        userID,
			RedirectURL:   resolveFacebookRedirectURL(c),
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	err = h.centralRedis.Set(c.Request.Context(), config.FacebookOAuthStatePrefix+state, body, config.FacebookOAuthStateTTL).Err()
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	var (
		query = url.Values{
			"client_id":     {h.baseConf.FacebookAppID},
			"redirect_uri":  {h.baseConf.FacebookRedirectURI},
			"state":         {state},
			"response_type": {"code"},
			"scope":         {config.FacebookOAuthScopes},
		}
		authURL = fmt.Sprintf("%s/%s/dialog/oauth?%s",
			h.baseConf.FacebookAuthBaseURL,
			h.baseConf.FacebookGraphAPIVersion,
			query.Encode(),
		)
	)

	h.HandleResponse(c, status_http.OK, map[string]any{"auth_url": authURL})
}

func (h *HandlerV1) FacebookCallback(c *gin.Context) {
	var (
		code       = c.Query("code")
		stateToken = c.Query("state")
		fbErr      = strings.TrimSpace(c.Query("error"))
	)

	// Resolve the origin first so failures land back where the user started.
	state, stateErr := h.getAndDeleteFacebookState(c.Request.Context(), stateToken)

	if fbErr != "" {
		detail := c.Query("error_description")
		if detail == "" {
			detail = c.Query("error_reason")
		}
		h.redirectFacebookOAuth(c, facebookOAuthOutcome{reason: "facebook_" + fbErr, detail: detail, state: state})
		return
	}

	if stateErr != nil {
		h.redirectFacebookOAuth(c, facebookOAuthOutcome{reason: "invalid_state", detail: stateErr.Error()})
		return
	}

	if code == "" {
		h.redirectFacebookOAuth(c, facebookOAuthOutcome{reason: "missing_code", state: state})
		return
	}

	shortLived, err := h.exchangeFacebookCode(c.Request.Context(), code)
	if err != nil {
		h.redirectFacebookOAuth(c, facebookOAuthOutcome{reason: "code_exchange_failed", detail: err.Error(), state: state})
		return
	}

	longLived, err := h.exchangeFacebookLongLivedToken(c.Request.Context(), shortLived)
	if err != nil {
		h.redirectFacebookOAuth(c, facebookOAuthOutcome{reason: "long_lived_exchange_failed", detail: err.Error(), state: state})
		return
	}

	user, err := h.facebookFetchUser(c.Request.Context(), longLived)
	if err != nil {
		h.log.Warn("facebook connect: fetch user profile failed: " + err.Error())
		user = models.FacebookUser{Name: state.UserId}
	}

	if _, err := h.companyServices.IntegrationResource().UpsertIntegrationResource(
		c.Request.Context(), &pb.CreateIntegrationResourceRequest{
			Token:         longLived,
			ProjectId:     state.ProjectId,
			EnvironmentId: state.EnvironmentId,
			Username:      user.Name,
			Name:          config.FacebookIntegrationName,
			Type:          pb.ResourceType_META_LEADS,
		},
	); err != nil {
		h.redirectFacebookOAuth(c, facebookOAuthOutcome{reason: "token_store_failed", detail: err.Error(), state: state})
		return
	}

	// After OAuth, subscribe every page the user manages and pull its forms so
	// no manual page/form selection is needed. Best-effort: failures are logged
	// but never block the success redirect (the user token is already stored).
	h.facebookAutoConnectAllPages(c.Request.Context(), state, longLived)

	// Validate and warm the read-only Business/Marketing API path used by the
	// Meta Ads dashboard. This also records real test calls for App Review.
	h.facebookWarmMarketingAPI(c.Request.Context(), longLived)
	h.facebookQueueAppReviewCalls(c.Request.Context(), longLived)

	h.redirectFacebookOAuth(c, facebookOAuthOutcome{success: true, state: state})
}

// facebookAutoConnectAllPages subscribes every page the connected user manages
// to the leadgen webhook and pre-syncs each page's forms into lead_forms, so a
// single OAuth captures all current (and, via page-level subscription, future)
// forms with zero manual selection.
func (h *HandlerV1) facebookAutoConnectAllPages(ctx context.Context, state models.FacebookOAuthState, userToken string) {
	pages, err := h.facebookListPages(ctx, userToken)
	if err != nil {
		h.log.Warn("facebook auto-connect: list pages failed: " + err.Error())
		return
	}

	for _, page := range pages {
		if _, err := h.facebookConnectPage(ctx, state, page, ""); err != nil {
			h.log.Warn("facebook auto-connect: connect page " + page.ID + " failed: " + err.Error())
			continue
		}
		h.facebookSyncPageForms(ctx, state, page)
	}
}

// FacebookStatus reports whether the connected Meta user token is still active so
// the frontend can prompt a reconnect before the user manages pages or forms.
func (h *HandlerV1) FacebookStatus(c *gin.Context) {
	state, ok := h.authContext(c)
	if !ok {
		return
	}

	userToken, err := h.getFacebookUserToken(c.Request.Context(), state)
	if err != nil {
		h.HandleResponse(c, status_http.OK, models.FacebookConnectionStatus{})
		return
	}

	debug, err := h.facebookDebugToken(c.Request.Context(), userToken)
	if err != nil {
		h.HandleResponse(c, status_http.OK, models.FacebookConnectionStatus{Connected: true, Reason: err.Error()})
		return
	}

	status := models.FacebookConnectionStatus{
		Connected: true,
		Active:    debug.IsValid,
		ExpiresAt: debug.ExpiresAt,
	}
	if !debug.IsValid {
		status.Reason = "token is no longer valid, reconnect required"
		if debug.Error != nil {
			status.Reason = debug.Error.Message
		}
	}

	h.HandleResponse(c, status_http.OK, status)
}

// ---------------------------------- FacebookPages --------------------------------------------------

func (h *HandlerV1) FacebookPages(c *gin.Context) {
	state, ok := h.authContext(c)
	if !ok {
		return
	}

	userToken, err := h.getFacebookUserToken(c.Request.Context(), state)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, "facebook is not connected, run connect first")
		return
	}

	pages, err := h.facebookListPages(c.Request.Context(), userToken)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, gin.H{"pages": pages})
}

func (h *HandlerV1) FacebookPageForms(c *gin.Context) {
	state, ok := h.authContext(c)
	if !ok {
		return
	}

	pageID := strings.TrimSpace(c.Param("page_id"))
	if pageID == "" {
		h.HandleResponse(c, status_http.InvalidArgument, "page_id is required")
		return
	}

	pageToken, err := h.getFacebookPageToken(c.Request.Context(), state, pageID)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	forms, err := h.facebookListForms(c.Request.Context(), pageID, pageToken)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, gin.H{"forms": forms})
}

func (h *HandlerV1) FacebookFormQuestions(c *gin.Context) {
	state, ok := h.authContext(c)
	if !ok {
		return
	}

	formID := strings.TrimSpace(c.Param("form_id"))
	if formID == "" {
		h.HandleResponse(c, status_http.InvalidArgument, "form_id is required")
		return
	}

	pageID := strings.TrimSpace(c.Query("page_id"))
	if pageID == "" {
		h.HandleResponse(c, status_http.InvalidArgument, "page_id query param is required")
		return
	}

	pageToken, err := h.getFacebookPageToken(c.Request.Context(), state, pageID)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	form, err := h.facebookFormQuestions(c.Request.Context(), formID, pageToken)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, form)
}

// ---------------------------------------- HELPER----------------------------------------------------

func (h *HandlerV1) getFacebookUserToken(ctx context.Context, state models.FacebookOAuthState) (string, error) {
	list, err := h.companyServices.IntegrationResource().GetIntegrationResourceList(
		ctx, &pb.GetListIntegrationResourceRequest{
			ProjectId:     state.ProjectId,
			EnvironmentId: state.EnvironmentId,
			Type:          pb.ResourceType_META_LEADS,
		},
	)
	if err != nil {
		return "", err
	}

	for _, integration := range list.GetIntegrationResources() {
		if token := strings.TrimSpace(integration.GetToken()); token != "" {
			return token, nil
		}
	}
	return "", errors.New("facebook is not connected, run connect first")
}

func (h *HandlerV1) getAndDeleteFacebookState(ctx context.Context, state string) (models.FacebookOAuthState, error) {
	var (
		payload models.FacebookOAuthState
		key     = config.FacebookOAuthStatePrefix + state
	)

	body, err := h.centralRedis.Get(ctx, key).Bytes()
	if err != nil {
		return payload, err
	}

	_ = h.centralRedis.Del(ctx, key).Err()

	if err = json.Unmarshal(body, &payload); err != nil {
		return payload, err
	}

	if payload.ProjectId == "" || payload.EnvironmentId == "" {
		return payload, errors.New("oauth state payload is incomplete")
	}

	return payload, nil
}

func (h *HandlerV1) getFacebookPageToken(ctx context.Context, state models.FacebookOAuthState, pageID string) (string, error) {
	userToken, err := h.getFacebookUserToken(ctx, state)
	if err != nil {
		return "", errors.New("facebook is not connected, run connect first")
	}

	pages, err := h.facebookListPages(ctx, userToken)
	if err != nil {
		return "", err
	}
	for _, page := range pages {
		if page.ID == pageID {
			return page.AccessToken, nil
		}
	}
	return "", fmt.Errorf("page %s is not managed by the connected account", pageID)
}

func generateOAuthState() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate oauth state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// facebookOAuthOutcome carries the callback result to the redirect: reason names
// the stage that failed, detail the underlying error, state the originating app.
type facebookOAuthOutcome struct {
	success bool
	reason  string
	detail  string
	state   models.FacebookOAuthState
}

// redirectFacebookOAuth returns the user to the exact page they started from with
// the result in the query: facebook=success, or facebook=error with reason (which
// stage failed) and detail (the underlying message) so the frontend can handle it.
func (h *HandlerV1) redirectFacebookOAuth(c *gin.Context, out facebookOAuthOutcome) {
	base := out.state.RedirectURL
	if base == "" {
		base = "/"
	}

	u, err := url.Parse(base)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, base)
		return
	}

	q := u.Query()
	if out.success {
		q.Set("facebook", "success")
	} else {
		q.Set("facebook", "error")
		if out.reason != "" {
			q.Set("reason", out.reason)
		}
		if out.detail != "" {
			q.Set("detail", out.detail)
		}
	}
	u.RawQuery = q.Encode()

	c.Redirect(http.StatusTemporaryRedirect, u.String())
}

func resolveFacebookRedirectURL(c *gin.Context) string {
	if raw := strings.TrimSpace(c.Query("redirect_uri")); raw != "" {
		return raw
	}
	return strings.TrimSpace(c.GetHeader("Referer"))
}
