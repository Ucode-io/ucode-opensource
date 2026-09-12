package api

import (
	"github.com/Ucode-io/ucode-opensource/services/auth/api/docs"
	"github.com/Ucode-io/ucode-opensource/services/auth/api/handlers"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"

	"github.com/gin-gonic/gin"
	"github.com/opentracing-contrib/go-gin/ginhttp"
	"github.com/opentracing/opentracing-go"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/ucode-io/ratelimiter"
)

// SetUpRouter godoc
// @description This is a api gateway
// @termsOfService https://u-code.io
func SetUpRouter(h handlers.Handler, cfg config.BaseConfig, tracer opentracing.Tracer, limiter ratelimiter.RateLimiterI) (r *gin.Engine) {
	r = gin.New()

	r.Use(gin.Logger(), gin.Recovery())
	r.Use(ginhttp.Middleware(tracer))

	docs.SwaggerInfo.Title = cfg.ServiceName
	docs.SwaggerInfo.Version = cfg.Version
	docs.SwaggerInfo.Schemes = []string{cfg.HTTPScheme}

	// @securityDefinitions.apikey ApiKeyAuth
	// @in header
	// @name Authorization
	// r.Use(customCORSMiddleware(cfg))
	r.Use(customCORSMiddleware())

	v2 := r.Group("/v2")
	v2.PUT("/refresh", h.V2RefreshToken)

	v2.Use(h.LoginMiddleware())
	{
		v2.GET("/connection", h.V2GetConnectionList)
		v2.POST("/multi-company/one-login", h.V2MultiCompanyOneLogin)
		v2.POST("/login", h.V2Login)
		v2.POST("/auth/logout", h.V2Logout)

	}

	v3 := r.Group("/v3")
	{
		v3.POST("/multicompany/default-login", h.V3MultiCompanyLogin)
	}
	v2.Use(h.AuthMiddleware())
	{
		// register
		v2.POST("/register", h.V2Register)

		v2.POST("/login/with-option", h.V2LoginWithOption)
		v2.POST("/send/message", h.SendMessage)
		v2.POST("/send-code-app", h.V2SendCodeApp)
		v2.POST("/forgot-password", h.ForgotPassword)
		v2.POST("/forgot-password-with-environment-email", h.ForgotPasswordWithEnvironmentEmail)
		v2.PUT("/reset-password", h.V2ResetPassword)
		v2.PUT("set-email/send-code", h.EmailEnter)
		v2.PUT("/expire-sessions", h.ExpireSessions)

		v2.GET("/e-imzo/challenge", h.GetChallenge)
		v2.POST("/e-imzo/verify/user", h.VerifyUser)

		v2.PUT("/refresh-superadmin", h.V2RefreshTokenSuperAdmin)
		v2.POST("/multi-company/login", h.V2MultiCompanyLogin)

		//connection
		v2.POST("/connection", h.V2CreateConnection)
		v2.GET("/connection/:connection_id", h.V2GetConnectionByID)
		v2.PUT("/connection", h.V2UpdateConnection)
		v2.DELETE("/connection/:connection_id", h.V2DeleteConnection)
		v2.GET("/get-connection-options/:connection_id/:user_id", h.GetConnectionOptions)

		// ROLE SERVICE
		v2.GET("/role/:role-id", h.V2GetRoleByID)
		v2.GET("/role", h.V2GetRolesList)
		v2.POST("/role", h.V2AddRole)
		v2.DELETE("/role/:role-id", h.V2RemoveRole)

		// CLIENT PLATFORM
		v2.GET("/client-platform", h.GetClientPlatformList)

		// admin, dev, hr, ceo
		v2.POST("/client-type", h.V2CreateClientType)
		v2.GET("/client-type", h.V2GetClientTypeList)
		v2.GET("/client-type/:client-type-id", h.V2GetClientTypeByID)
		v2.PUT("/client-type", h.V2UpdateClientType)
		v2.DELETE("/client-type/:client-type-id", h.V2DeleteClientType)

		// role-permission
		v2.GET("/role-permission/detailed/:project-id/:role-id", h.GetListWithRoleAppTablePermissions)
		v2.PUT("/role-permission/detailed", h.UpdateRoleAppTablePermissions)

		// menu-permission
		v2.GET("/menu-permission/detailed/:project-id/:role-id/:parent-id", h.GetListMenuPermissions)
		v2.PUT("/menu-permission/detailed", h.UpdateMenuPermissions)

		// user
		v2.POST("/user", h.V2CreateUser)
		v2.POST("/user/sync", h.SyncUser)
		v2.GET("/user", h.V2GetUserList)
		v2.GET("/user/:user-id", h.V2GetUserByID)
		v2.PUT("/user", h.V2UpdateUser)
		v2.DELETE("/user/:user-id", h.V2DeleteUser)
		v2.DELETE("/auth-table/:user-id", h.V2DeleteFromAuthTable)
		v2.PUT("/user/reset-password", h.V2UserResetPassword)
		v2.POST("/user/invite", h.AddUserToProject)
		v2.POST("/user/check", h.V2GetUserByLoginType)

		// api keys
		apiKeys := v2.Group("/api-key")
		{
			apiKeys.POST("/:project-id", h.CreateApiKey)
			apiKeys.PUT("/:project-id/:id", h.UpdateApiKey)
			apiKeys.GET("/:project-id/:id", h.GetApiKey)
			apiKeys.GET("/:project-id", h.GetListApiKeys)
			apiKeys.DELETE("/:project-id/:id", h.DeleteApiKeys)
			apiKeys.POST("/generate-token", h.GenerateApiKeyToken)
			apiKeys.POST("/refresh-token", h.RefreshApiKeyToken)
			apiKeys.GET("/:project-id/tokens", h.ListClientTokens)
		}

		//session
		session := v2.Group("/session")
		{
			session.GET("", h.GetSessionList)
			session.DELETE("/:id", h.DeleteSession)
			session.DELETE("", h.DeleteByParams)

			session.GET("/devices", h.GetSessionDevices)
			session.DELETE("/by-device", h.DeleteSessionsByDevice)
			session.DELETE("/except-current", h.DeleteSessionsExceptCurrent)
		}

		// environment
		v2.GET("/resource-environment", h.GetAllResourceEnvironments)
	}

	auth := v2.Group("/auth")
	{
		auth.POST("/register/:provider", h.V2RegisterProvider)
		auth.POST("/verify/:verify_id", h.V2VerifyOtp)
		auth.POST("/login/:provider", h.V2LoginProvider)
		auth.POST("/refresh", h.V2RefreshToken)
		auth.POST("/send-code", h.V2SendCode)
		auth.POST("/password/reset", h.V2UserResetPassword)
	}

	v2Sms := r.Group("/v2")
	v2Sms.Use(h.AuthMiddleware(), limiter.GinMiddleware())
	{
		v2Sms.POST("/send-code", h.V2SendCode)
	}

	//COMPANY
	company := r.Group("company")
	{
		company.POST("", h.RegisterCompany)
		company.PUT("", h.UpdateCompany)
		company.DELETE("/:company-id", h.RemoveCompany)
	}

	//PROJECT
	project := r.Group("project")
	{
		project.POST("", h.CreateProject)
		project.PUT("", h.UpdateProject)
		project.GET("/:project-id", h.GetProjectByID)
		project.DELETE("/:project-id", h.DeleteProject)
	}

	// Integration
	r.POST("/emqx", h.Emqx)
	r.POST("/custom", h.Custom)

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	return
}

func customCORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		} else {
			c.Header("Access-Control-Allow-Origin", "*")
		}

		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, Resource-Id, Environment-Id, X-API-KEY, Platform-Type")
		c.Header("Access-Control-Max-Age", "3600")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}