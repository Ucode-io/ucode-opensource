package handlers

import (
	"context"
	"log"
	nethttp "net/http"
	"net/url"
	"strings"
	"time"

	status_http "github.com/Ucode-io/ucode-opensource/services/auth/api/http"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	pba "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"

	"github.com/gin-gonic/gin"
)

// UgenRegister godoc
// @ID create_company
// @Router /v3/ugen-register [POST]
// @Summary Register User
// @Description Register User
// @Tags Ugen
// @Accept json
// @Produce json
// @Param company body auth_service.RegisterCompanyRequest true "RegisterCompanyRequestBody"
// @Success 201 {object} http.Response{data=auth_service.CompanyPrimaryKey} "Company data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) UgenRegister(c *gin.Context) {
	var company pba.RegisterCompanyRequest

	if err := c.ShouldBindJSON(&company); err != nil {
		h.handleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	company.IsUgen = true
	company.FareId = config.UGEN_FREE_FARE_ID

	resp, err := h.services.CompanyService().Register(
		c.Request.Context(), &company,
	)
	if err != nil {
		h.handleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, status_http.Created, resp)
}

func (h *Handler) UgenLogin(c *gin.Context) {
	var login pba.UgenLoginReq

	if err := c.ShouldBindJSON(&login); err != nil {
		h.handleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	login.ClientIp = c.ClientIP()
	login.UserAgent = c.Request.UserAgent()

	response, err := h.services.SessionService().UgenLogin(c.Request.Context(), &login)
	if err != nil {
		h.handleError(c, status_http.BadRequest, err)
		return
	}

	h.handleResponse(c, status_http.Created, response)
}

func (h *Handler) UgenGoogleAuth(c *gin.Context) {
	oauthCfg := h.googleOAuthConfig()
	state, nonce, err := helper.NewGoogleOAuthState(h.cfg.SecretKey, 5*time.Minute)
	if err != nil {
		h.redirectUgenAuthCallback(c, "error", "state_failed")
		return
	}

	redirectURL, err := helper.GoogleAuthRedirectURL(oauthCfg, state)
	if err != nil {
		h.redirectUgenAuthCallback(c, "error", "config_missing")
		return
	}

	h.setStateCookie(c, helper.GoogleOAuthStateCookie, nonce, int((5 * time.Minute).Seconds()))
	c.Redirect(nethttp.StatusFound, redirectURL)
}

func (h *Handler) UgenGoogleCallback(c *gin.Context) {
	if errText := c.Query("error"); errText != "" {
		h.clearCookie(c, helper.GoogleOAuthStateCookie)
		h.redirectUgenAuthCallback(c, "error", "google_denied")
		return
	}

	stateCookie, err := c.Cookie(helper.GoogleOAuthStateCookie)
	if err != nil {
		h.redirectUgenAuthCallback(c, "error", "state_missing")
		return
	}
	h.clearCookie(c, helper.GoogleOAuthStateCookie)

	if err = helper.ValidateGoogleOAuthState(c.Query("state"), stateCookie, h.cfg.SecretKey); err != nil {
		h.redirectUgenAuthCallback(c, "error", "state_invalid")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	httpClient := &nethttp.Client{Timeout: 15 * time.Second}
	tokenResp, err := helper.ExchangeGoogleCode(ctx, httpClient, h.googleOAuthConfig(), c.Query("code"))
	if err != nil {
		h.redirectUgenAuthCallback(c, "error", "token_exchange_failed")
		return
	}

	profile, err := helper.FetchGoogleProfile(ctx, httpClient, tokenResp.AccessToken)
	if err != nil {
		h.redirectUgenAuthCallback(c, "error", "profile_failed")
		return
	}

	loginResp, err := h.services.SessionService().UgenGoogleLogin(
		c.Request.Context(),
		&pba.UgenGoogleLoginReq{
			GoogleId:  profile.ID,
			Email:     profile.Email,
			Name:      profile.Name,
			Picture:   profile.Picture,
			ClientIp:  c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		},
	)
	if err != nil {
		log.Println("ERROR GOOGLE SIGN IN:", err.Error())
		h.redirectUgenAuthCallback(c, "error", "login_failed")
		return
	}

	token := loginResp.GetResponse().GetToken()
	if token == nil || token.GetAccessToken() == "" || token.GetRefreshToken() == "" {
		h.redirectUgenAuthCallback(c, "error", "token_missing")
		return
	}

	h.setSessionCookie(c, helper.GoogleAccessCookie, token.GetAccessToken(), int(config.AccessTokenExpiresInTime.Seconds()))
	h.setSessionCookie(c, helper.GoogleRefreshCookie, token.GetRefreshToken(), int(config.RefreshTokenExpiresInTime.Seconds()))
	h.redirectUgenAuthCallback(c, "success", "")
}

func (h *Handler) UgenAuthSession(c *gin.Context) {
	accessToken, err := c.Cookie(helper.GoogleAccessCookie)
	if err != nil {
		h.handleResponse(c, status_http.Unauthorized, "access token cookie is required")
		return
	}
	refreshToken, err := c.Cookie(helper.GoogleRefreshCookie)
	if err != nil {
		h.handleResponse(c, status_http.Unauthorized, "refresh token cookie is required")
		return
	}

	response, err := h.services.SessionService().UgenAuthSession(
		c.Request.Context(),
		&pba.UgenAuthSessionReq{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
		},
	)
	if err != nil {
		h.handleError(c, status_http.Unauthorized, err)
		return
	}

	h.handleResponse(c, status_http.OK, response)
}

func (h *Handler) googleOAuthConfig() helper.GoogleOAuthConfig {
	return helper.GoogleOAuthConfig{
		ClientID:     h.cfg.GoogleClientID,
		ClientSecret: h.cfg.GoogleClientSecret,
		CallbackURL:  h.cfg.GoogleCallbackURL,
		Secret:       h.cfg.SecretKey,
	}
}

func (h *Handler) redirectUgenAuthCallback(c *gin.Context, status, message string) {
	frontendURL := strings.TrimRight(h.cfg.FrontendURL, "/")
	if frontendURL == "" {
		frontendURL = "/"
	}

	values := url.Values{}
	values.Set("provider", "google")
	values.Set("status", status)
	if message != "" {
		values.Set("message", message)
	}

	if frontendURL == "/" {
		c.Redirect(nethttp.StatusFound, "/auth-callback?"+values.Encode())
		return
	}
	c.Redirect(nethttp.StatusFound, frontendURL+"/auth-callback?"+values.Encode())
}

// setSessionCookie writes a cross-site session cookie (SameSite=None requires Secure).
// Used for access/refresh tokens that the frontend reads via cross-origin XHR with credentials.
func (h *Handler) setSessionCookie(c *gin.Context, name, value string, maxAge int) {
	c.SetSameSite(nethttp.SameSiteNoneMode)
	c.SetCookie(name, value, maxAge, "/", h.cfg.CookieDomain, true, true)
}

// setStateCookie writes a same-site cookie used during OAuth top-level redirects.
func (h *Handler) setStateCookie(c *gin.Context, name, value string, maxAge int) {
	c.SetSameSite(nethttp.SameSiteLaxMode)
	c.SetCookie(name, value, maxAge, "/", h.cfg.CookieDomain, h.secureCookies(), true)
}

func (h *Handler) clearCookie(c *gin.Context, name string) {
	h.setStateCookie(c, name, "", -1)
}

func (h *Handler) secureCookies() bool {
	return h.cfg.Environment != config.DebugMode && h.cfg.Environment != config.TestMode
}
