package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/helper"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	defaultUgenBillingPeriodCode = "monthly"
	ugenCurrencyRateFetchTimeout = 10 * time.Second
)

var timeNow = time.Now

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type queryExecer interface {
	queryRower
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

type ugenBillingPeriod struct {
	code            string
	name            string
	months          int32
	discountPercent float64
	isActive        bool
	sortOrder       int32
}

func normalizeUgenBillingPeriodCode(code string) string {
	if code == "" {
		return defaultUgenBillingPeriodCode
	}
	return code
}

func loadUgenBillingPeriod(ctx context.Context, db queryRower, code string) (*ugenBillingPeriod, error) {
	code = normalizeUgenBillingPeriodCode(code)

	period := &ugenBillingPeriod{}
	err := db.QueryRow(ctx, `
		SELECT code, name, months, discount_percent, is_active, sort_order
		FROM ugen_billing_period
		WHERE code = $1 AND is_active = TRUE`,
		code,
	).Scan(
		&period.code,
		&period.name,
		&period.months,
		&period.discountPercent,
		&period.isActive,
		&period.sortOrder,
	)
	if err != nil {
		return nil, err
	}

	return period, nil
}

func loadUgenFreeFareID(ctx context.Context, db queryRower) (string, error) {
	var fareID string
	err := db.QueryRow(ctx, `
		SELECT id::TEXT
		FROM fare
		WHERE product_type = $1
		  AND deleted_at = 0
		  AND price = 0
		ORDER BY created_at ASC
		LIMIT 1`,
		config.PRODUCT_TYPE_UGEN,
	).Scan(&fareID)
	if err != nil {
		return "", err
	}

	return fareID, nil
}

func defaultUgenFreePeriod() *ugenBillingPeriod {
	return &ugenBillingPeriod{
		code:            defaultUgenBillingPeriodCode,
		months:          1,
		discountPercent: 0,
		isActive:        true,
	}
}

func fetchUgenCurrencyRate(ctx context.Context, db queryExecer, currency, date string) (float64, error) {
	currency = strings.ToUpper(currency)
	if currency == "" || currency == config.CURRENCY_UZS {
		return 1, nil
	}

	rateCtx, cancel := context.WithTimeout(ctx, ugenCurrencyRateFetchTimeout)
	defer cancel()

	rate, err := helper.FetchCurrencyRate(rateCtx, currency, date)
	if err == nil {
		_ = saveUgenCurrencyRate(ctx, db, currency, date, rate)
		return rate, nil
	}

	if cachedRate, cacheErr := loadLatestUgenCurrencyRate(ctx, db, currency); cacheErr == nil {
		return cachedRate, nil
	}

	if transactionRate, transactionErr := loadLatestUgenTransactionRate(ctx, db, currency); transactionErr == nil {
		return transactionRate, nil
	}

	if envRate, envErr := loadUgenCurrencyRateFromEnv(currency); envErr == nil {
		return envRate, nil
	}

	return 0, fmt.Errorf("fetch currency rate for %s: %w", currency, err)
}

func saveUgenCurrencyRate(ctx context.Context, db queryExecer, currency, date string, rate float64) error {
	if rate <= 0 {
		return nil
	}
	if !ugenCurrencyRateCacheExists(ctx, db) {
		return nil
	}

	_, err := db.Exec(ctx, `
		INSERT INTO ugen_currency_rate_cache(currency_code, rate_date, rate)
		VALUES ($1, $2, $3)
		ON CONFLICT (currency_code, rate_date) DO UPDATE SET
			rate = EXCLUDED.rate,
			updated_at = NOW()`,
		currency,
		date,
		rate,
	)
	return err
}

func loadLatestUgenCurrencyRate(ctx context.Context, db queryRower, currency string) (float64, error) {
	if !ugenCurrencyRateCacheExists(ctx, db) {
		return 0, pgx.ErrNoRows
	}

	var rate float64
	err := db.QueryRow(ctx, `
		SELECT rate::FLOAT8
		FROM ugen_currency_rate_cache
		WHERE currency_code = $1
		  AND rate > 0
		ORDER BY rate_date DESC, updated_at DESC
		LIMIT 1`,
		currency,
	).Scan(&rate)
	if err != nil {
		return 0, err
	}

	return rate, nil
}

func ugenCurrencyRateCacheExists(ctx context.Context, db queryRower) bool {
	var exists bool
	err := db.QueryRow(ctx, `
		SELECT to_regclass('public.ugen_currency_rate_cache') IS NOT NULL`,
	).Scan(&exists)
	return err == nil && exists
}

func loadLatestUgenTransactionRate(ctx context.Context, db queryRower, currency string) (float64, error) {
	var rate float64
	err := db.QueryRow(ctx, `
		SELECT t.rate::FLOAT8
		FROM transaction t
		JOIN fare f ON f.id = t.fare_id
		JOIN currency c ON c.id = f.currency_id
		WHERE f.product_type = $1
		  AND c.code = $2
		  AND t.rate > 1
		ORDER BY t.created_at DESC
		LIMIT 1`,
		config.PRODUCT_TYPE_UGEN,
		currency,
	).Scan(&rate)
	if err != nil {
		return 0, err
	}

	return rate, nil
}

func loadUgenCurrencyRateFromEnv(currency string) (float64, error) {
	for _, key := range []string{
		"UGEN_CURRENCY_RATE_" + currency,
		"CURRENCY_RATE_" + currency,
	} {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			continue
		}

		rate, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, err
		}
		if rate > 0 {
			return rate, nil
		}
	}

	return 0, pgx.ErrNoRows
}

func addCalendarMonths(date time.Time, months int32) time.Time {
	monthStart := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
	targetStart := monthStart.AddDate(0, int(months), 0)
	targetLastDay := time.Date(targetStart.Year(), targetStart.Month()+1, 0, 0, 0, 0, 0, date.Location()).Day()
	day := date.Day()
	if day > targetLastDay {
		day = targetLastDay
	}

	return time.Date(targetStart.Year(), targetStart.Month(), day, 0, 0, 0, 0, date.Location())
}

func ugenPeriodCharge(price, rate float64, period *ugenBillingPeriod) float64 {
	charge := price * float64(period.months) * (1 - period.discountPercent/100) * rate
	return helper.RoundToTwoDecimals(charge, 2)
}

func ugenProratedUpgradeCharge(currentPrice, newPrice, rate float64, periodMonths int32, discountPercent float64, startDate, endDate, now time.Time) float64 {
	diff := newPrice - currentPrice
	if diff <= 0 || !now.Before(endDate) {
		return 0
	}

	periodEndDate := addCalendarMonths(startDate, periodMonths)
	periodSeconds := periodEndDate.Sub(startDate).Seconds()
	if periodSeconds <= 0 {
		return 0
	}

	remainingSeconds := endDate.Sub(now).Seconds()
	if remainingSeconds <= 0 {
		return 0
	}

	charge := diff * float64(periodMonths) * (1 - discountPercent/100) * (remainingSeconds / periodSeconds) * rate
	return helper.RoundToTwoDecimals(charge, 2)
}

func parseDateOnly(date string) (time.Time, error) {
	return time.ParseInLocation(time.DateOnly, date, time.Local)
}

func formatDateOnly(date time.Time) string {
	return date.Format(time.DateOnly)
}

func scanSubscription(row pgx.Row) (*pb.Subscription, error) {
	var (
		sub         pb.Subscription
		discountID  sql.NullString
		pendingFare sql.NullString
		periodCode  sql.NullString
		months      sql.NullInt32
		discount    sql.NullFloat64
		cancelAtEnd sql.NullBool
		canceledAt  sql.NullString
		startDate   sql.NullString
		endDate     sql.NullString
		renewalDate sql.NullString
	)

	err := row.Scan(
		&sub.Id,
		&sub.ProjectId,
		&sub.FareId,
		&discountID,
		&sub.Status,
		&sub.Type,
		&startDate,
		&endDate,
		&renewalDate,
		&pendingFare,
		&periodCode,
		&months,
		&discount,
		&cancelAtEnd,
		&canceledAt,
	)
	if err != nil {
		return nil, err
	}

	sub.DiscountId = discountID.String
	sub.PendingFareId = pendingFare.String
	sub.BillingPeriodCode = periodCode.String
	sub.BillingPeriodMonths = months.Int32
	sub.BillingPeriodDiscountPercent = discount.Float64
	sub.CancelAtPeriodEnd = cancelAtEnd.Bool
	sub.CanceledAt = canceledAt.String
	sub.StartDate = startDate.String
	sub.EndDate = endDate.String
	sub.RenewalDate = renewalDate.String

	return &sub, nil
}

func subscriptionSelectColumns() string {
	return `
		id,
		project_id,
		fare_id,
		discount_id,
		status,
		type,
		COALESCE(start_date::TEXT, ''),
		COALESCE(end_date::TEXT, ''),
		COALESCE(renewal_date::TEXT, ''),
		pending_fare_id::TEXT,
		billing_period_code,
		billing_period_months,
		billing_period_discount_percent,
		cancel_at_period_end,
		COALESCE(canceled_at::TEXT, '')`
}

func isUgenProduct(productType string) bool {
	return productType == config.PRODUCT_TYPE_UGEN
}
