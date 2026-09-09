package v1

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"

	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"

	"github.com/spf13/cast"
	"golang.org/x/sync/errgroup"
)

// unlimitedTokenCompanies are exempt from token-limit blocking: the budget is
// still tracked and deducted, but Check never refuses them. Keyed by company id.
var unlimitedTokenCompanies = map[string]bool{
	"324f86b1-4dd7-48cf-adcd-ec430548b942": true,
}

func (p *ChatProcessor) tokenLimitExempt() bool {
	return unlimitedTokenCompanies[p.companyId]
}

type TokenLimitError struct {
	Period string
	Used   int64
	Limit  int64
}

func (e *TokenLimitError) Error() string {
	return fmt.Sprintf("token limit exceeded (%s): used %d / %d", e.Period, e.Used, e.Limit)
}

func (p *ChatProcessor) initTokenBudget(ctx context.Context) {
	billingProjectID := p.billingProjectId
	if billingProjectID == "" {
		billingProjectID = p.ucodeProjectId
	}

	if billingProjectID == "" {
		log.Printf("[TOKEN BUDGET] skipped: no billing project id (ucode_project_id=%s mcp_ucode_project_id=%s company_id=%s)",
			p.ucodeProjectId, p.mcpUcodeProjectId, p.companyId)
		return
	}
	if p.companyId == "" {
		log.Printf("[TOKEN BUDGET] skipped: no company id (billing_project_id=%s ucode_project_id=%s mcp_ucode_project_id=%s)",
			billingProjectID, p.ucodeProjectId, p.mcpUcodeProjectId)
		return
	}

	var (
		limitsResp  *pb.GetPricingLimitsResponse
		metricsResp *pb.GetAiTokenUsageMetricsResponse
		packResp    *pb.GetTokenPackBalanceResponse
	)

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		limitsResp, err = p.h.companyServices.Billing().GetPricingLimits(gCtx, &pb.GetPricingLimitsRequest{ProjectId: billingProjectID})
		return err
	})
	g.Go(func() error {
		var err error
		metricsResp, err = p.h.companyServices.Billing().GetAiTokenUsageMetrics(gCtx, &pb.GetAiTokenUsageMetricsRequest{CompanyId: p.companyId})
		return err
	})
	g.Go(func() error {
		var err error
		packResp, err = p.h.companyServices.Billing().GetTokenPackBalance(gCtx, &pb.GetTokenPackBalanceRequest{CompanyId: p.companyId})
		return err
	})
	if err := g.Wait(); err != nil {
		log.Printf("[TOKEN BUDGET] billing unavailable, skipping limit check: %v", err)
		return
	}

	var snap models.TokenBudgetSnapshot
	for _, l := range limitsResp.GetLimits() {
		log.Printf("[TOKEN BUDGET] limit type=%q name=%q value=%q", l.GetType(), l.GetName(), l.GetValue())
		switch l.GetType() {
		case "tokens_day":
			snap.DayLimit = cast.ToInt64(l.GetValue())
		case "tokens_month":
			snap.MonthLimit = cast.ToInt64(l.GetValue())
		}
	}

	if snap.DayLimit == 0 && snap.MonthLimit == 0 {
		log.Printf("[TOKEN BUDGET] skipped: no token limits configured for billing_project_id=%s ucode_project_id=%s mcp_ucode_project_id=%s",
			billingProjectID, p.ucodeProjectId, p.mcpUcodeProjectId)
		return
	}

	// Plan (fare) budget excludes pack-funded tokens so it recovers cleanly each
	// period; the pack pool is a separate company-scoped fallback that never resets.
	snap.DayUsed = metricsResp.GetTodayPlanTokens()
	snap.MonthUsed = metricsResp.GetMonthlyPlanTokens()
	snap.PackRemain = packResp.GetRemainingTokens()

	remain := int64(-1)
	if snap.DayLimit > 0 {
		r := snap.DayLimit - snap.DayUsed
		if remain == -1 || r < remain {
			remain = r
		}
	}
	if snap.MonthLimit > 0 {
		r := snap.MonthLimit - snap.MonthUsed
		if remain == -1 || r < remain {
			remain = r
		}
	}

	p.tokenBudgetEnabled = true
	p.tokenBudgetSnap = snap
	atomic.StoreInt64(&p.tokenBudgetRemain, remain)
	atomic.StoreInt64(&p.tokenPackRemain, snap.PackRemain)

	log.Printf("[TOKEN BUDGET] initialized: billing_project_id=%s usage_project_id=%s plan_remain=%d pack_remain=%d (day %d/%d, month %d/%d)",
		billingProjectID, p.mcpUcodeProjectId, remain, snap.PackRemain, snap.DayUsed, snap.DayLimit, snap.MonthUsed, snap.MonthLimit)
}

func (p *ChatProcessor) Check() error {
	if !p.tokenBudgetEnabled || p.tokenLimitExempt() {
		return nil
	}
	// Blocked only when the fare budget AND the pack pool are both exhausted; the
	// pack is the automatic fallback once the fare day/month limit is reached.
	if atomic.LoadInt64(&p.tokenBudgetRemain) <= 0 && atomic.LoadInt64(&p.tokenPackRemain) <= 0 {
		return p.buildTokenLimitError()
	}
	return nil
}

// splitBudget decides how a usage of `total` tokens is funded: fare first, then
// the available pack pool. Any usage beyond both pools remains plan usage so it
// is visible as over-limit spend instead of being incorrectly hidden as pack use.
// It is read-only so RecordUsage (which reports the pack portion) and Deduct
// (which applies it) agree on the split.
func (p *ChatProcessor) splitBudget(total int64) (planPortion, packPortion int64) {
	if total <= 0 {
		return 0, 0
	}

	planRemain := atomic.LoadInt64(&p.tokenBudgetRemain)
	packRemain := atomic.LoadInt64(&p.tokenPackRemain)

	if planRemain < 0 {
		planRemain = 0
	}
	if packRemain < 0 {
		packRemain = 0
	}

	if total <= planRemain {
		return total, 0
	}

	overPlan := total - planRemain
	if packRemain > 0 {
		if overPlan <= packRemain {
			packPortion = overPlan
		} else {
			packPortion = packRemain
		}
	}

	planPortion = total - packPortion
	return planPortion, packPortion
}

func (p *ChatProcessor) Deduct(tokens int64) {
	if !p.tokenBudgetEnabled || tokens <= 0 {
		return
	}
	planPortion, packPortion := p.splitBudget(tokens)
	if planPortion > 0 {
		atomic.AddInt64(&p.tokenBudgetRemain, -planPortion)
	}
	if packPortion > 0 {
		if newRemain := atomic.AddInt64(&p.tokenPackRemain, -packPortion); newRemain < 0 {
			atomic.StoreInt64(&p.tokenPackRemain, 0)
		}
	}
}

func (p *ChatProcessor) buildTokenLimitError() *TokenLimitError {
	snap := p.tokenBudgetSnap
	spent := snap.DayLimit - atomic.LoadInt64(&p.tokenBudgetRemain) - snap.DayUsed
	if spent < 0 {
		spent = 0
	}

	DayUsedNow := snap.DayUsed + spent
	MonthUsedNow := snap.MonthUsed + spent

	// determine which limit is the bottleneck
	period := "day"
	used := DayUsedNow
	limit := snap.DayLimit

	if snap.MonthLimit > 0 && snap.DayLimit > 0 {
		dayRemain := snap.DayLimit - DayUsedNow
		monthRemain := snap.MonthLimit - MonthUsedNow
		if monthRemain < dayRemain {
			period, used, limit = "month", MonthUsedNow, snap.MonthLimit
		}
	} else if snap.MonthLimit > 0 {
		period, used, limit = "month", MonthUsedNow, snap.MonthLimit
	}

	return &TokenLimitError{Period: period, Used: used, Limit: limit}
}

func (p *ChatProcessor) tokenLimitData(err *TokenLimitError) models.TokenLimitData {
	snap := p.tokenBudgetSnap
	spent := snap.DayLimit - atomic.LoadInt64(&p.tokenBudgetRemain) - snap.DayUsed
	if spent < 0 {
		spent = 0
	}
	code := models.PaymentCodeTokenMonthLimit
	if err.Period == "day" {
		code = models.PaymentCodeTokenDayLimit
	}
	packRemain := atomic.LoadInt64(&p.tokenPackRemain)
	if packRemain < 0 {
		packRemain = 0
	}
	return models.TokenLimitData{
		Type:       models.PaymentRequiredType,
		Code:       code,
		Period:     err.Period,
		Used:       err.Used,
		Limit:      err.Limit,
		Unit:       models.PaymentUnitTokens,
		DayUsed:    snap.DayUsed + spent,
		DayLimit:   snap.DayLimit,
		MonthUsed:  snap.MonthUsed + spent,
		MonthLimit: snap.MonthLimit,
		PackRemain: packRemain,
	}
}
