package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
)

func (h *HandlerV1) facebookQueueAppReviewCalls(ctx context.Context, userToken string) {
	if h.centralRedis == nil || h.baseConf.FacebookAppID == "" {
		return
	}

	lockKey := config.FacebookAppReviewLockPrefix + h.baseConf.FacebookAppID
	acquired, err := h.centralRedis.SetNX(ctx, lockKey, "running", config.FacebookAppReviewLockTTL).Result()
	if err != nil {
		h.log.Warn("facebook app review calls: acquire lock failed: " + err.Error())
		return
	}
	if !acquired {
		return
	}

	go func() {
		runCtx, cancel := context.WithTimeout(context.Background(), config.FacebookAppReviewRunTimeout)
		defer cancel()

		successes, attempts, runErr := h.facebookRunAppReviewCalls(
			runCtx,
			userToken,
			config.FacebookAppReviewTargetCalls,
			config.FacebookAppReviewMaxAttempts,
			config.FacebookAppReviewCallInterval,
		)
		result := fmt.Sprintf("successes=%d attempts=%d", successes, attempts)
		if runErr != nil {
			h.log.Warn("facebook app review calls: " + result + ": " + runErr.Error())
			_ = h.centralRedis.Del(context.Background(), lockKey).Err()
			return
		}

		resultKey := config.FacebookAppReviewResultPrefix + h.baseConf.FacebookAppID
		if err := h.centralRedis.Set(context.Background(), resultKey, result, config.FacebookAppReviewResultTTL).Err(); err != nil {
			h.log.Warn("facebook app review calls: store result failed: " + err.Error())
		}
		_ = h.centralRedis.Set(context.Background(), lockKey, "complete", config.FacebookAppReviewResultTTL).Err()
		h.log.Info("facebook app review calls: " + result)
	}()
}

func (h *HandlerV1) facebookRunAppReviewCalls(
	ctx context.Context,
	userToken string,
	target int,
	maxAttempts int,
	interval time.Duration,
) (int, int, error) {
	var adAccounts struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := h.facebookGraphGet(ctx, "me/adaccounts", url.Values{
		"fields":       {"id"},
		"limit":        {"1"},
		"access_token": {userToken},
	}, &adAccounts); err != nil {
		return 0, 0, fmt.Errorf("list ad accounts: %w", err)
	}
	if len(adAccounts.Data) == 0 || adAccounts.Data[0].ID == "" {
		return 0, 0, fmt.Errorf("no ad account available for review test calls")
	}

	var lastErr error
	successes := 0
	attempts := 0
	for attempts < maxAttempts && successes < target {
		var insights struct {
			Data []json.RawMessage `json:"data"`
		}
		lastErr = h.facebookGraphGet(ctx, adAccounts.Data[0].ID+"/insights", url.Values{
			"fields":       {"spend,impressions"},
			"date_preset":  {"last_7d"},
			"level":        {"account"},
			"limit":        {"1"},
			"access_token": {userToken},
		}, &insights)
		attempts++
		if lastErr == nil {
			successes++
		}
		if successes >= target {
			break
		}
		if interval > 0 {
			select {
			case <-ctx.Done():
				return successes, attempts, ctx.Err()
			case <-time.After(interval):
			}
		}
	}

	if successes < target {
		return successes, attempts, fmt.Errorf("target not reached: last error: %w", lastErr)
	}
	return successes, attempts, nil
}
