package v1

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"golang.org/x/sync/errgroup"
)

func parseStorageLimitToMB(val string) float64 {
	val = strings.TrimSpace(val)
	if strings.HasSuffix(val, "GB") {
		return cast.ToFloat64(strings.TrimSuffix(val, "GB")) * 1024
	}
	if strings.HasSuffix(val, "MB") {
		return cast.ToFloat64(strings.TrimSuffix(val, "MB"))
	}
	return 0
}

func (h *HandlerV1) GetAllPricingUsage(c *gin.Context) {
	projectId := cast.ToString(c.MustGet("project_id"))
	environmentId := cast.ToString(c.MustGet("environment_id"))
	h.log.Info(fmt.Sprintf("[pricing] GetAllPricingUsage: projectId=%s environmentId=%s", projectId, environmentId))

	limitsResp := &company_service.GetPricingLimitsResponse{}
	usageResp := &nb.GetResourceUsageResponse{}
	usersCountResp := &auth_service.GetProjectUsersCountResponse{}
	apiKeysResp := &auth_service.GetProjectApiKeysCountResponse{}
	tokenMetrics := &company_service.GetAiTokenUsageMetricsResponse{}
	apiMetrics := &company_service.GetApiCallMonitoringMetricsResponse{}
	ugenStatus := &company_service.GetProjectUgenStatusResponse{}

	g, gCtx := errgroup.WithContext(c.Request.Context())

	g.Go(func() error {
		resp, err := h.companyServices.Billing().GetPricingLimits(gCtx, &company_service.GetPricingLimitsRequest{ProjectId: projectId})
		if err != nil {
			h.log.Error(fmt.Sprintf("[GetAllPricingUsage] GetPricingLimits failed: %v", err))
			return nil
		}
		limitsResp = resp
		return nil
	})
	g.Go(func() error {
		service, resourceEnvId, err := h.getBuilderService(gCtx, projectId, environmentId)
		if err != nil {
			h.log.Error(fmt.Sprintf("[GetAllPricingUsage] getBuilderService failed: %v", err))
			return nil
		}
		resp, err := service.GoObjectBuilderService().ObjectBuilder().GetResourceUsage(gCtx, &nb.GetResourceUsageRequest{ProjectId: resourceEnvId})
		if err != nil {
			h.log.Error(fmt.Sprintf("[GetAllPricingUsage] GetResourceUsage failed: %v", err))
			return nil
		}
		usageResp = resp
		return nil
	})
	g.Go(func() error {
		resp, err := h.authService.User().GetProjectUsersCount(gCtx, &auth_service.GetProjectUsersCountRequest{ProjectId: projectId})
		if err != nil {
			h.log.Error(fmt.Sprintf("[GetAllPricingUsage] GetProjectUsersCount failed: %v", err))
			return nil
		}
		usersCountResp = resp
		return nil
	})
	g.Go(func() error {
		resp, err := h.authService.ApiKey().GetProjectApiKeysCount(gCtx, &auth_service.GetProjectApiKeysCountRequest{ProjectId: projectId})
		if err != nil {
			h.log.Error(fmt.Sprintf("[GetAllPricingUsage] GetProjectApiKeysCount failed: %v", err))
			return nil
		}
		apiKeysResp = resp
		return nil
	})
	g.Go(func() error {
		resp, err := h.companyServices.Billing().GetAiTokenUsageMetrics(gCtx, &company_service.GetAiTokenUsageMetricsRequest{ProjectId: projectId})
		if err != nil {
			h.log.Error(fmt.Sprintf("[GetAllPricingUsage] GetAiTokenUsageMetrics failed: %v", err))
			return nil
		}
		tokenMetrics = resp
		return nil
	})
	g.Go(func() error {
		resp, err := h.companyServices.Billing().GetApiCallMonitoringMetrics(gCtx, &company_service.GetApiCallMonitoringMetricsRequest{ProjectId: projectId})
		if err != nil {
			h.log.Error(fmt.Sprintf("[GetAllPricingUsage] GetApiCallMonitoringMetrics failed: %v", err))
			return nil
		}
		apiMetrics = resp
		return nil
	})
	g.Go(func() error {
		project, err := h.companyServices.Project().GetById(gCtx, &company_service.GetProjectByIdRequest{ProjectId: projectId})
		if err != nil {
			h.log.Error(fmt.Sprintf("[GetAllPricingUsage] GetProjectById failed: %v", err))
			return nil
		}
		resp, err := h.companyServices.Project().GetProjectUgenStatus(gCtx, &company_service.GetProjectUgenStatusRequest{
			ProjectId: projectId,
			CompanyId: project.GetCompanyId(),
		})
		if err != nil {
			h.log.Error(fmt.Sprintf("[GetAllPricingUsage] GetProjectUgenStatus failed: %v", err))
			return nil
		}
		ugenStatus = resp
		return nil
	})

	_ = g.Wait()

	response := models.AllPricingUsage{
		Functions:       models.PricingUsage{Current: float64(usageResp.FunctionsCount), Unit: "count"},
		Microfrontend:   models.PricingUsage{Current: float64(usageResp.MicrofrontendsCount), Unit: "count"},
		AssetSize:       models.PricingUsage{Current: float64(usageResp.AssetSize) / (1024 * 1024), Unit: "MB"},
		DatabaseSize:    models.PricingUsage{Current: float64(usageResp.DatabaseSize) / (1024 * 1024), Unit: "MB"},
		Users:           models.PricingUsage{Current: float64(usersCountResp.Count), Unit: "count"},
		Items:           models.PricingUsage{Current: float64(usageResp.ItemsCount), Unit: "count"},
		Tables:          models.PricingUsage{Current: float64(usageResp.TablesCount), Unit: "count"},
		ApiKeys:         models.PricingUsage{Current: float64(apiKeysResp.Count), Unit: "count"},
		TodayTokens:     models.PricingUsage{Current: float64(tokenMetrics.TodayInputTokens + tokenMetrics.TodayOutputTokens), Unit: "tokens"},
		MonthlyTokens:   models.PricingUsage{Current: float64(tokenMetrics.MonthlyInputTokens + tokenMetrics.MonthlyOutputTokens), Unit: "tokens"},
		MonthlyApiCalls: models.PricingUsage{Current: float64(apiMetrics.TotalMonthlyCalls), Unit: "count"},
		Projects:        models.PricingUsage{Current: float64(ugenStatus.CompanyProjectsCount - 1), Unit: "count"},
	}

	for _, limit := range limitsResp.Limits {
		switch limit.Type {
		case "function":
			response.Functions.Limit = cast.ToFloat64(limit.Value)
		case "microfrontend":
			response.Microfrontend.Limit = cast.ToFloat64(limit.Value)
		case "database":
			if limit.Name == "Database Size" {
				response.DatabaseSize.Limit = parseStorageLimitToMB(limit.Value)
			}
		case "asset_size":
			response.AssetSize.Limit = parseStorageLimitToMB(limit.Value)
		case "request_per_month":
			response.MonthlyApiCalls.Limit = cast.ToFloat64(limit.Value)
		case "users_count":
			response.Users.Limit = cast.ToFloat64(limit.Value)
		case "items":
			response.Items.Limit = cast.ToFloat64(limit.Value)
		case "tables":
			response.Tables.Limit = cast.ToFloat64(limit.Value)
		case "api_keys":
			response.ApiKeys.Limit = cast.ToFloat64(limit.Value)
		case "tokens_day":
			response.TodayTokens.Limit = cast.ToFloat64(limit.Value)
		case "tokens_month":
			response.MonthlyTokens.Limit = cast.ToFloat64(limit.Value)
		case "projects":
			response.Projects.Limit = cast.ToFloat64(limit.Value)
		}
	}

	h.HandleResponse(c, status_http.OK, response)
}

// GetTokenUsage godoc
// @Summary Get AI token usage
// @Description Get AI input/output token usage for today and this month
// @Tags Billing
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} status_http.Response{data=models.TokenUsageResponse} "Token usage"
// @Failure 401
// @Router /v1/pricing/token-usage [get]
func (h *HandlerV1) GetTokenUsage(c *gin.Context) {
	projectId := cast.ToString(c.MustGet("project_id"))

	resp, err := h.companyServices.Billing().GetAiTokenUsageMetrics(
		c.Request.Context(),
		&company_service.GetAiTokenUsageMetricsRequest{ProjectId: projectId},
	)
	if err != nil {
		h.log.Error("[GetTokenUsage] GetAiTokenUsageMetrics error", logger.Error(err))
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, models.TokenUsageResponse{
		Today: models.TokenUsage{
			InputTokens:  resp.TodayInputTokens,
			OutputTokens: resp.TodayOutputTokens,
		},
		Monthly: models.TokenUsage{
			InputTokens:  resp.MonthlyInputTokens,
			OutputTokens: resp.MonthlyOutputTokens,
		},
	})
}

// GetCompanyStats godoc
// @Summary Company-level stats
// @Description AI token usage (daily & monthly), total project count, and builder project count — all scoped to the company
// @Tags Billing
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} status_http.Response{data=models.CompanyStatsResponse} "Company stats"
// @Failure 401
// @Router /v1/pricing/company-stats [get]
func (h *HandlerV1) GetCompanyStats(c *gin.Context) {
	var (
		projectId     = cast.ToString(c.MustGet("project_id"))
		environmentId = cast.ToString(c.MustGet("environment_id"))

		companyId string

		ctx = c.Request.Context()
	)

	project, err := h.companyServices.Project().GetById(ctx, &company_service.GetProjectByIdRequest{ProjectId: projectId})
	if err != nil {
		h.log.Error("[GetCompanyStats] GetById", logger.Error(err))
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}
	companyId = project.GetCompanyId()

	resource, err := h.companyServices.ServiceResource().GetSingle(
		ctx, &company_service.GetSingleServiceResourceReq{
			ProjectId:     projectId,
			EnvironmentId: environmentId,
			ServiceType:   company_service.ServiceType_BUILDER_SERVICE,
		},
	)

	if err != nil || resource.GetResourceType() != company_service.ResourceType_POSTGRESQL {
		h.log.Error("[GetCompanyStats] GetSingleServiceResourceReq", logger.Error(err))
		h.HandleResponse(c, status_http.InternalServerError, logger.Error(err))
		return
	}

	var (
		tokenMetrics = new(company_service.GetAiTokenUsageMetricsResponse)
		limitsResp   = new(company_service.GetPricingLimitsResponse)
		packResp     = new(company_service.GetTokenPackBalanceResponse)
		projectCount int32
		builderCount int32
		userCount    int32
	)

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		resp, err := h.companyServices.Billing().GetPricingLimits(gCtx, &company_service.GetPricingLimitsRequest{ProjectId: projectId})
		if err != nil {
			h.log.Error("[GetCompanyStats] GetPricingLimits", logger.Error(err))
			return nil
		}
		limitsResp = resp
		return nil
	})

	g.Go(func() error {
		resp, err := h.companyServices.Billing().GetTokenPackBalance(gCtx, &company_service.GetTokenPackBalanceRequest{CompanyId: companyId})
		if err != nil {
			h.log.Error("[GetCompanyStats] GetTokenPackBalance", logger.Error(err))
			return nil
		}
		packResp = resp
		return nil
	})

	g.Go(func() error {
		resp, err := h.companyServices.Billing().GetAiTokenUsageMetrics(gCtx, &company_service.GetAiTokenUsageMetricsRequest{CompanyId: companyId})
		if err != nil {
			h.log.Error("[GetCompanyStats] GetAiTokenUsageMetrics", logger.Error(err))
			return nil
		}
		tokenMetrics = resp
		return nil
	})

	g.Go(func() error {
		service, resourceEnvId, err := h.getBuilderService(gCtx, projectId, environmentId)
		if err != nil {
			h.log.Error(fmt.Sprintf("[GetAllPricingUsage] getBuilderService failed: %v", err))
			return nil
		}

		resp, err := service.GoObjectBuilderService().McpProject().GetPublishedMcpProjectCount(gCtx, &nb.GetPublishedMcpProjectCountReq{ResourceEnvId: resourceEnvId})
		if err != nil {
			h.log.Error(fmt.Sprintf("[GetAllPricingUsage] GetResourceUsage failed: %v", err))
			return nil
		}

		projectCount = resp.GetCount()
		return nil
	})

	g.Go(func() error {
		resp, err := h.authService.User().GetCompanyUsersCount(gCtx, &auth_service.GetCompanyUsersCountRequest{CompanyId: companyId})
		if err != nil {
			h.log.Error("[GetCompanyStats] GetCompanyUsersCount", logger.Error(err))
			return nil
		}
		userCount = resp.GetCount()
		return nil
	})

	g.Go(func() error {
		resp, err := h.authService.User().GetProjectUsersCount(gCtx, &auth_service.GetProjectUsersCountRequest{ProjectId: projectId})
		if err != nil {
			h.log.Error("[GetCompanyStats] GetCompanyUsersCount", logger.Error(err))
			return nil
		}
		builderCount = resp.GetCount()
		return nil
	})

	g.Wait()

	var (
		dailyTokenLimit   int64
		monthlyTokenLimit int64
		projectLimit      int32
		builderLimit      int32
		userLimit         int32
	)

	for _, limit := range limitsResp.GetLimits() {
		switch limit.Type {
		case "tokens_day":
			dailyTokenLimit = cast.ToInt64(limit.Value)
		case "tokens_month":
			monthlyTokenLimit = cast.ToInt64(limit.Value)
		case "projects":
			projectLimit = cast.ToInt32(limit.Value)
		case "builders":
			builderLimit = cast.ToInt32(limit.Value)
		case "users_count":
			userLimit = cast.ToInt32(limit.Value)
		}
	}

	dailyPlanTokens := tokenMetrics.GetTodayPlanTokens()
	monthlyPlanTokens := tokenMetrics.GetMonthlyPlanTokens()
	packRemaining := packResp.GetRemainingTokens()

	dailyLimitReached := dailyTokenLimit > 0 && dailyPlanTokens >= dailyTokenLimit
	monthlyLimitReached := monthlyTokenLimit > 0 && monthlyPlanTokens >= monthlyTokenLimit

	// Once either fare limit is reached, usage falls back to the pack pool; the
	// active source mirrors the gateway's automatic plan→pack→exhausted switch.
	activeSource := "plan"
	if dailyLimitReached || monthlyLimitReached {
		if packRemaining > 0 {
			activeSource = "pack"
		} else {
			activeSource = "exhausted"
		}
	}

	h.HandleResponse(c, status_http.OK, models.CompanyStatsResponse{
		Tokens: models.CompanyTokenStats{
			Daily:   models.CompanyTokenStat{InputTokens: tokenMetrics.GetTodayInputTokens(), OutputTokens: tokenMetrics.GetTodayOutputTokens(), PlanTokens: dailyPlanTokens, Limit: dailyTokenLimit, LimitReached: dailyLimitReached},
			Monthly: models.CompanyTokenStat{InputTokens: tokenMetrics.GetMonthlyInputTokens(), OutputTokens: tokenMetrics.GetMonthlyOutputTokens(), PlanTokens: monthlyPlanTokens, Limit: monthlyTokenLimit, LimitReached: monthlyLimitReached},
			PackRemaining: packRemaining,
			ActiveSource:  activeSource,
		},
		ProjectCount: models.CompanyStat{Current: projectCount, Limit: projectLimit},
		Builders:     models.CompanyStat{Current: builderCount, Limit: builderLimit},
		UserCount:    models.CompanyStat{Current: userCount - builderCount, Limit: userLimit},
	})
}

// GetApiMetrics godoc
// @Summary API metrics
// @Description Real-time API rate metrics and historical call counts
// @Tags Billing
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} status_http.Response{data=models.ApiMetricsResponse} "Metrics Data"
// @Failure 401
// @Router /v1/pricing/api-metrics [get]
func (h *HandlerV1) GetApiMetrics(c *gin.Context) {
	projectId := cast.ToString(c.MustGet("project_id"))

	now := time.Now()
	prevNow := now.Add(-1 * time.Minute)

	minKey := fmt.Sprintf(config.KeyRateMin, projectId, now.Format("2006-01-02-15-04"))
	prevMinKey := fmt.Sprintf(config.KeyRateMin, projectId, prevNow.Format("2006-01-02-15-04"))
	hourKey := fmt.Sprintf(config.KeyRateHour, projectId, now.Format("2006-01-02-15"))
	dayKey := fmt.Sprintf(config.KeyRateDay, projectId, now.Format("2006-01-02"))

	var getRedisInt = func(key string) int64 {
		valStr, _ := h.redis.Get(c.Request.Context(), key, "", "")
		return cast.ToInt64(valStr)
	}

	currentMinCalls := getRedisInt(minKey)
	prevMinCalls := getRedisInt(prevMinKey)
	currentHourCalls := getRedisInt(hourKey)
	todayCalls := getRedisInt(dayKey)

	sec := float64(now.Second())

	// RPS — rolling 60-секундное окно
	prevMinuteWeight := (60.0 - sec) / 60.0
	rolling60sCalls := float64(currentMinCalls) + (float64(prevMinCalls) * prevMinuteWeight)
	rps := math.Round((rolling60sCalls/60.0)*100) / 100

	// RPM — экстраполяция текущей скорости на 60 сек
	var rpm int64
	elapsedSec := sec + 1
	if sec >= 5 {
		rpm = int64(math.Round(float64(currentMinCalls) / elapsedSec * 60.0))
	} else {
		rpm = int64(math.Round(rolling60sCalls))
	}

	// RPH — экстраполяция текущей скорости на 60 мин
	var rph int64
	elapsedMin := float64(now.Minute()) + sec/60.0 + (1.0 / 60.0)
	if elapsedMin >= 1 {
		rph = int64(math.Round(float64(currentHourCalls) / elapsedMin * 60.0))
	} else {
		rph = currentHourCalls
	}

	metricsResp, err := h.companyServices.Billing().GetApiCallMonitoringMetrics(
		c.Request.Context(),
		&company_service.GetApiCallMonitoringMetricsRequest{ProjectId: projectId},
	)

	var monthly int64
	if err == nil && metricsResp != nil {
		monthly = metricsResp.TotalMonthlyCalls
	} else if err != nil {
		h.log.Error("[GetApiMetrics] GetMonitoringMetrics error", logger.Error(err))
	}

	h.HandleResponse(c, status_http.OK, models.ApiMetricsResponse{
		Rps:          rps,
		Rpm:          rpm,
		Rph:          rph,
		TodayCalls:   todayCalls,
		MonthlyCalls: monthly,
	})
}

// GetApiChart godoc
// @Summary API Chart
// @Description Historical daily chart array
// @Tags Billing
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} status_http.Response{data=models.ApiChartResponse} "Chart"
// @Failure 401
// @Router /v1/pricing/api-chart [get]
func (h *HandlerV1) GetApiChart(c *gin.Context) {
	projectId := cast.ToString(c.MustGet("project_id"))

	metricsResp, err := h.companyServices.Billing().GetApiCallMonitoringMetrics(
		c.Request.Context(),
		&company_service.GetApiCallMonitoringMetricsRequest{ProjectId: projectId},
	)
	if err != nil {
		h.log.Error("GetMonitoringMetrics chart error", logger.Error(err))
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	chart := make([]models.DailyChartPoint, 0)
	if metricsResp != nil && metricsResp.DailyUsageChart != nil {
		for _, dbPoint := range metricsResp.DailyUsageChart {
			chart = append(chart, models.DailyChartPoint{
				Date:  dbPoint.Date,
				Count: dbPoint.Count,
			})
		}
	}

	h.HandleResponse(c, status_http.OK, models.ApiChartResponse{Chart: chart})
}

// GetPerformanceMetrics godoc
// @Summary Get performance metrics
// @Description Get average response duration (ms) and error rate for today's requests
// @Tags Billing
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} status_http.Response{data=models.PerformanceMetricsResponse} "Performance metrics"
// @Failure 401
// @Router /v1/pricing/performance [get]
func (h *HandlerV1) GetPerformanceMetrics(c *gin.Context) {
	service, resourceEnvId, err := h.getAiChatServices(c)
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	today := time.Now().Format("2006-01-02")
	resp, err := service.GoObjectBuilderService().VersionHistory().GetPerformanceMetrics(
		c.Request.Context(),
		&nb.GetPerformanceMetricsRequest{
			ProjectId: resourceEnvId,
			FromDate:  today,
			ToDate:    today,
		},
	)
	if err != nil {
		h.log.Error("[GetPerformanceMetrics] error", logger.Error(err))
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, models.PerformanceMetricsResponse{
		AverageResponseTime: float64(resp.AverageDuration),
		ErrorRate:           float64(resp.ErrorRate),
	})
}
