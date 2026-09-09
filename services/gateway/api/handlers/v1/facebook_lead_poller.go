package v1

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"
)

// The lead poller is a safety net for webhook delivery. Meta can delay or (for an
// unpublished app) skip webhook delivery, so the poller periodically pulls new
// leads directly from each connected page's forms via Graph API and ingests them
// through the same contact+deal path. Writes are idempotent (deduped by the
// deterministic guid from leadgen_id), so a lead already delivered by the webhook
// is never duplicated.
const (
	facebookPollProjectsKey = "facebook-lead-poll:projects" // Redis SET of "<project_id>|<environment_id>"
	facebookPollLockKey     = "facebook-lead-poll:lock"     // single-runner lock across replicas
	facebookPollCursorKey   = "facebook-lead-poll:cursor:"  // + resource_id -> last seen created_time (unix)
	facebookPollCursorTTL   = 30 * 24 * time.Hour
	facebookLeadsFields     = "id,created_time,ad_id,ad_name,form_id,platform,is_organic,field_data"
)

// registerFacebookPollProject records a project/environment that has a connected
// Meta Leads page, so the poller knows to scan it. Called at connect time; safe
// to call repeatedly (SET semantics).
func (h *HandlerV1) registerFacebookPollProject(ctx context.Context, projectID, environmentID string) {
	if h.centralRedis == nil || projectID == "" || environmentID == "" {
		return
	}
	if err := h.centralRedis.SAdd(ctx, facebookPollProjectsKey, projectID+"|"+environmentID).Err(); err != nil {
		h.log.Warn("facebook poll: register project failed: " + err.Error())
	}
}

// StartLeadPoller launches the background polling loop when enabled. No-op if
// disabled or central Redis is unavailable.
func (h *HandlerV1) StartLeadPoller(ctx context.Context) {
	if !h.baseConf.FacebookLeadPollEnabled {
		h.log.Info("facebook poll: disabled")
		return
	}
	if h.centralRedis == nil {
		h.log.Warn("facebook poll: central redis unavailable, poller not started")
		return
	}

	interval := time.Duration(h.baseConf.FacebookLeadPollIntervalSec) * time.Second
	if interval < 30*time.Second {
		interval = 2 * time.Minute
	}
	h.log.Info("facebook poll: enabled, interval=" + interval.String())

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.runFacebookPollCycle(interval)
			}
		}
	}()
}

// runFacebookPollCycle polls once. A central-Redis lock ensures only one replica
// runs a given cycle, avoiding duplicate Graph traffic (correctness is guaranteed
// by idempotent writes regardless).
func (h *HandlerV1) runFacebookPollCycle(interval time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			h.log.Error("facebook poll: recovered from panic", logger.Any("panic", r))
		}
	}()

	ctx := context.Background()

	lockTTL := interval - time.Second
	if lockTTL <= 0 {
		lockTTL = interval
	}
	acquired, err := h.centralRedis.SetNX(ctx, facebookPollLockKey, time.Now().UnixNano(), lockTTL).Result()
	if err != nil {
		h.log.Warn("facebook poll: lock error: " + err.Error())
		return
	}
	if !acquired {
		return // another replica owns this cycle
	}

	members, err := h.centralRedis.SMembers(ctx, facebookPollProjectsKey).Result()
	if err != nil {
		h.log.Warn("facebook poll: read projects failed: " + err.Error())
		return
	}
	for _, m := range members {
		parts := strings.SplitN(m, "|", 2)
		if len(parts) != 2 {
			continue
		}
		h.pollProjectLeads(ctx, parts[0], parts[1])
	}
}

func (h *HandlerV1) pollProjectLeads(ctx context.Context, projectID, environmentID string) {
	list, err := h.companyServices.Resource().GetProjectResourceList(ctx, &pb.GetProjectResourceListRequest{
		ProjectId:     projectID,
		EnvironmentId: environmentID,
		Type:          pb.ResourceType_META_LEADS,
	})
	if err != nil {
		h.log.Warn("facebook poll: list resources failed: " + err.Error())
		return
	}
	for _, resource := range list.GetResources() {
		h.pollResourceLeads(ctx, resource)
	}
}

func (h *HandlerV1) pollResourceLeads(ctx context.Context, resource *pb.ProjectResource) {
	credentials := resource.GetSettings().GetFacebookLeads()
	if credentials == nil {
		return
	}
	pageToken := strings.TrimSpace(credentials.GetPageAccessToken())
	pageID := strings.TrimSpace(resource.GetExternalId())
	if pageToken == "" || pageID == "" {
		return
	}

	cursorKey := facebookPollCursorKey + resource.GetId()
	since := h.facebookPollCursor(ctx, cursorKey)
	if since == 0 {
		// First time we see this page: start from now so enabling the poller does
		// not backfill historical leads. Going forward we pull only newer leads.
		h.setFacebookPollCursor(ctx, cursorKey, time.Now().Unix())
		return
	}

	forms, err := h.facebookListForms(ctx, pageID, pageToken)
	if err != nil {
		h.log.Warn("facebook poll: list forms failed: " + err.Error())
		return
	}

	newest := since
	for _, form := range forms {
		leads, maxTime, err := h.facebookFetchFormLeads(ctx, form.ID, pageToken, since)
		if err != nil {
			h.log.Warn("facebook poll: fetch leads failed: " + err.Error())
			continue
		}
		if maxTime > newest {
			newest = maxTime
		}

		for i := range leads {
			lead := leads[i]
			value := models.FacebookLeadChangeValue{LeadgenID: lead.ID, PageID: pageID, FormID: form.ID}
			if err := h.writeProfessionalCRMLead(ctx, resource, lead, value); err != nil {
				h.log.Error("facebook poll: write failed", logger.Error(err),
					logger.String("leadgen_id", lead.ID),
					logger.String("page_id", pageID),
				)
			}
		}
	}

	if newest > since {
		h.setFacebookPollCursor(ctx, cursorKey, newest)
	}
}

// facebookFetchFormLeads returns the form's leads created after `since` (unix
// seconds) plus the newest created_time seen, for cursor advancement.
func (h *HandlerV1) facebookFetchFormLeads(ctx context.Context, formID, pageToken string, since int64) ([]models.FacebookLead, int64, error) {
	query := url.Values{
		"fields":       {facebookLeadsFields},
		"access_token": {pageToken},
		"limit":        {"100"},
	}
	if since > 0 {
		query.Set("filtering", fmt.Sprintf(`[{"field":"time_created","operator":"GREATER_THAN","value":%d}]`, since))
	}

	var page struct {
		Data []models.FacebookLead `json:"data"`
	}
	if err := h.facebookGraphGet(ctx, formID+"/leads", query, &page); err != nil {
		return nil, since, err
	}

	maxTime := since
	for i := range page.Data {
		if t := parseFacebookTime(page.Data[i].CreatedTime); t > maxTime {
			maxTime = t
		}
	}
	return page.Data, maxTime, nil
}

func (h *HandlerV1) facebookPollCursor(ctx context.Context, key string) int64 {
	v, err := h.centralRedis.Get(ctx, key).Result()
	if err != nil {
		return 0
	}
	ts, _ := strconv.ParseInt(v, 10, 64)
	return ts
}

func (h *HandlerV1) setFacebookPollCursor(ctx context.Context, key string, ts int64) {
	if err := h.centralRedis.Set(ctx, key, ts, facebookPollCursorTTL).Err(); err != nil {
		h.log.Warn("facebook poll: set cursor failed: " + err.Error())
	}
}

// parseFacebookTime parses Graph's lead created_time (e.g. 2026-08-17T06:00:00+0000)
// into a unix timestamp; returns 0 if unparseable.
func parseFacebookTime(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	for _, layout := range []string{"2006-01-02T15:04:05-0700", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Unix()
		}
	}
	return 0
}
