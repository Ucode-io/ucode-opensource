package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/helper"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/opentracing/opentracing-go"
)

var MonthsMap = map[time.Month]int{
	time.January:   1,
	time.February:  2,
	time.March:     3,
	time.April:     4,
	time.May:       5,
	time.June:      6,
	time.July:      7,
	time.August:    8,
	time.September: 9,
	time.October:   10,
	time.November:  11,
	time.December:  12,
}

type PaymentInfo struct {
	SubscriptionID string
	ProjectID      string
	Price          float64
	Balance        float64
	CreditLimit    float64
	FareId         string
	DiscountId     string
	DiscountMonths int16
	DiscountValue  float64
	CurrencyCode   string
}

type UgenPaymentInfo struct {
	SubscriptionID        string
	ProjectID             string
	FareID                string
	EffectiveFareID       string
	PendingFareID         string
	Status                string
	StartDate             string
	EndDate               string
	Price                 float64
	CurrentPrice          float64
	Balance               float64
	CreditLimit           float64
	CurrencyCode          string
	BillingPeriodCode     string
	BillingPeriodMonths   int32
	BillingPeriodDiscount float64
	CancelAtPeriodEnd     bool
}

func (b *billingRepo) ProcessUcodeRenewals(ctx context.Context) error {
	log.Println("Starting ucode subscription renewals...")

	rows, err := b.db.Query(ctx, `
		SELECT DISTINCT ON (s.project_id)
			s.id,
			s.project_id,
			f.price,
			p.balance,
			p.credit_limit,
			f.id,
			COALESCE(s.discount_id::TEXT, ''),
			COALESCE(d.months, 0),
			COALESCE(d.value, 0),
			COALESCE(c.code, '')
		FROM subscription s
		JOIN fare f ON s.fare_id = f.id
		JOIN project p ON s.project_id = p.id
		JOIN currency c ON c.id = f.currency_id
		LEFT JOIN discount d ON s.discount_id = d.id
		WHERE f.product_type = $1
		  AND s.status = $2
		  AND s.renewal_date <= NOW()
		ORDER BY s.project_id, s.created_at DESC`,
		config.PRODUCT_TYPE_UCODE,
		config.SUBSCRIPTION_STATUS_ACTIVE,
	)
	if err != nil {
		return b.db.HandleDatabaseError(err, "ProcessUcodeRenewals: fetch due subscriptions")
	}
	defer rows.Close()

	var payments []PaymentInfo
	for rows.Next() {
		var (
			info           PaymentInfo
			discountID     sql.NullString
			discountMonths sql.NullInt16
		)
		if err := rows.Scan(
			&info.SubscriptionID,
			&info.ProjectID,
			&info.Price,
			&info.Balance,
			&info.CreditLimit,
			&info.FareId,
			&discountID,
			&discountMonths,
			&info.DiscountValue,
			&info.CurrencyCode,
		); err != nil {
			return b.db.HandleDatabaseError(err, "ProcessUcodeRenewals: scan subscription")
		}

		info.DiscountId = discountID.String
		info.DiscountMonths = 1
		if discountMonths.Valid && discountMonths.Int16 > 0 {
			info.DiscountMonths = discountMonths.Int16
		}

		payments = append(payments, info)
	}
	if err := rows.Err(); err != nil {
		return b.db.HandleDatabaseError(err, "ProcessUcodeRenewals: rows")
	}
	if len(payments) == 0 {
		return nil
	}

	// Phase 1: compute the charge for each subscription up front. calculatePrice may
	type ucodeCharge struct {
		info    PaymentInfo
		amount  float64
		endDate string
	}

	charges := make([]ucodeCharge, 0, len(payments))
	for i := range payments {
		priceResp, err := calculatePrice(ctx, &payments[i])
		if err != nil {
			return b.db.HandleDatabaseError(err, "ProcessUcodeRenewals: calculate price")
		}
		charges = append(charges, ucodeCharge{
			info:    payments[i],
			amount:  priceResp.CalculatedPrice,
			endDate: priceResp.EndDate,
		})
	}

	// Phase 2: apply charges. Each subscription is re-validated under a row lock so a
	tx, err := b.db.Begin(ctx)
	if err != nil {
		return b.db.HandleDatabaseError(err, "ProcessUcodeRenewals: begin")
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	for _, ch := range charges {
		var balance, creditLimit float64

		err = tx.QueryRow(ctx, `
			SELECT p.balance, p.credit_limit
			FROM subscription s
			JOIN project p ON p.id = s.project_id
			WHERE s.id = $1
			  AND s.status = $2
			  AND s.renewal_date <= NOW()
			FOR UPDATE`,
			ch.info.SubscriptionID,
			config.SUBSCRIPTION_STATUS_ACTIVE,
		).Scan(&balance, &creditLimit)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return b.db.HandleDatabaseError(err, "ProcessUcodeRenewals: lock subscription")
		}

		// Insolvent: block immediately. The subscription stays active and renewal_date
		if balance+creditLimit < ch.amount {
			if _, err := tx.Exec(ctx, `
				UPDATE project SET status = $1 WHERE id = $2`,
				config.STATUS_INSUFFICIENT_FUNDS, ch.info.ProjectID,
			); err != nil {
				return b.db.HandleDatabaseError(err, "ProcessUcodeRenewals: block project")
			}
			continue
		}

		endDate, err := time.Parse(time.DateOnly, ch.endDate)
		if err != nil {
			return b.db.HandleDatabaseError(err, "ProcessUcodeRenewals: parse end date")
		}
		nextRenewal := endDate.AddDate(0, 0, 1).Format(time.DateOnly)

		if _, err := tx.Exec(ctx, `
			UPDATE project
			SET balance = balance - $1,
				status  = $2
			WHERE id = $3`,
			ch.amount, config.STATUS_ACTIVE, ch.info.ProjectID,
		); err != nil {
			return b.db.HandleDatabaseError(err, "ProcessUcodeRenewals: debit project")
		}

		if _, err := tx.Exec(ctx, `
			UPDATE subscription
			SET status       = $1,
				type         = $2,
				end_date     = $3,
				renewal_date = $4,
				updated_at   = NOW()
			WHERE id = $5`,
			config.SUBSCRIPTION_STATUS_ACTIVE,
			config.SUBSCRIPTION_PAID,
			ch.endDate,
			nextRenewal,
			ch.info.SubscriptionID,
		); err != nil {
			return b.db.HandleDatabaseError(err, "ProcessUcodeRenewals: renew subscription")
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO transaction(
				id, project_id, subscription_id, amount, transaction_type,
				payment_status, creator_type, currency_id, created_at, fare_id)
			VALUES ($1, $2, $3, $4, $5, 'accepted', 'server', $6, NOW(), $7)`,
			uuid.NewString(),
			ch.info.ProjectID,
			ch.info.SubscriptionID,
			ch.amount,
			config.TransactionTypeSubscription,
			config.PaymeCurrencyId,
			ch.info.FareId,
		); err != nil {
			return b.db.HandleDatabaseError(err, "ProcessUcodeRenewals: insert transaction")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return b.db.HandleDatabaseError(err, "ProcessUcodeRenewals: commit")
	}

	log.Println("Ucode subscription renewals processed successfully.")
	return nil
}

func (b *billingRepo) ProcessUgenRenewals(ctx context.Context) error {
	log.Println("Starting ugen subscription renewals...")

	query := `
		WITH latest AS (
			SELECT DISTINCT ON (s.project_id)
				s.id,
				s.project_id,
				s.fare_id,
				CASE
					WHEN s.status = 'pending_downgrade' AND s.pending_fare_id IS NOT NULL THEN s.pending_fare_id
					WHEN s.status = 'active' AND f.price = 0 AND s.pending_fare_id IS NOT NULL THEN s.pending_fare_id
					ELSE s.fare_id
				END AS effective_fare_id,
				COALESCE(s.pending_fare_id::TEXT, '') AS pending_fare_id,
				s.status,
				s.start_date::TEXT,
				s.end_date::TEXT,
				ef.price,
				f.price AS current_price,
				p.balance,
				p.credit_limit,
				COALESCE(ec.code, '') AS currency_code,
				COALESCE(
					CASE WHEN s.status = 'active' AND f.price = 0 AND s.pending_fare_id IS NOT NULL THEN restore.billing_period_code END,
					s.billing_period_code,
					'monthly'
				) AS billing_period_code,
				COALESCE(
					CASE WHEN s.status = 'active' AND f.price = 0 AND s.pending_fare_id IS NOT NULL THEN restore.billing_period_months END,
					s.billing_period_months,
					1
				) AS billing_period_months,
				COALESCE(
					CASE WHEN s.status = 'active' AND f.price = 0 AND s.pending_fare_id IS NOT NULL THEN restore.billing_period_discount_percent END,
					s.billing_period_discount_percent,
					0
				) AS billing_period_discount_percent,
				COALESCE(s.cancel_at_period_end, FALSE) AS cancel_at_period_end
			FROM subscription s
			JOIN fare f ON s.fare_id = f.id
			LEFT JOIN LATERAL (
				SELECT
					os.billing_period_code,
					os.billing_period_months,
					os.billing_period_discount_percent
				FROM subscription os
				JOIN fare old_fare ON old_fare.id = os.fare_id
				WHERE os.project_id = s.project_id
				  AND os.fare_id = s.pending_fare_id
				  AND os.status = 'expired'
				  AND old_fare.product_type = $1
				  AND old_fare.price > 0
				ORDER BY os.created_at DESC
				LIMIT 1
			) restore ON TRUE
			JOIN fare ef ON ef.id = CASE
				WHEN s.status = 'pending_downgrade' AND s.pending_fare_id IS NOT NULL THEN s.pending_fare_id
				WHEN s.status = 'active' AND f.price = 0 AND s.pending_fare_id IS NOT NULL THEN s.pending_fare_id
				ELSE s.fare_id
			END
			JOIN project p ON s.project_id = p.id
			JOIN currency ec ON ec.id = ef.currency_id
			WHERE f.product_type = $1
			ORDER BY s.project_id, s.created_at DESC
		)
		SELECT
			id,
			project_id,
			fare_id,
			effective_fare_id,
			pending_fare_id,
			status,
			start_date,
			end_date,
			price,
			current_price,
			balance,
			credit_limit,
			currency_code,
			billing_period_code,
			billing_period_months,
			billing_period_discount_percent,
			cancel_at_period_end
		FROM latest
		WHERE (status IN ('active', 'pending_downgrade') AND end_date::DATE <= CURRENT_DATE)
		   OR status = 'expired'
		   OR (status = 'active' AND current_price = 0 AND pending_fare_id <> '')
	`

	rows, err := b.db.Query(ctx, query, config.PRODUCT_TYPE_UGEN)
	if err != nil {
		return b.db.HandleDatabaseError(err, "ProcessUgenRenewals: fetch subscriptions")
	}
	defer rows.Close()

	var payments []UgenPaymentInfo
	for rows.Next() {
		var pmt UgenPaymentInfo
		if err := rows.Scan(
			&pmt.SubscriptionID,
			&pmt.ProjectID,
			&pmt.FareID,
			&pmt.EffectiveFareID,
			&pmt.PendingFareID,
			&pmt.Status,
			&pmt.StartDate,
			&pmt.EndDate,
			&pmt.Price,
			&pmt.CurrentPrice,
			&pmt.Balance,
			&pmt.CreditLimit,
			&pmt.CurrencyCode,
			&pmt.BillingPeriodCode,
			&pmt.BillingPeriodMonths,
			&pmt.BillingPeriodDiscount,
			&pmt.CancelAtPeriodEnd,
		); err != nil {
			return b.db.HandleDatabaseError(err, "ProcessUgenRenewals: scan subscription")
		}
		payments = append(payments, pmt)
	}
	if err := rows.Err(); err != nil {
		return b.db.HandleDatabaseError(err, "ProcessUgenRenewals: rows")
	}
	if len(payments) == 0 {
		return nil
	}

	now := timeNow()

	rateByCurrency := make(map[string]float64)
	for _, pmt := range payments {
		currency := pmt.CurrencyCode
		if currency == "" || currency == config.CURRENCY_UZS {
			continue
		}
		if _, ok := rateByCurrency[currency]; ok {
			continue
		}
		rate, rateErr := fetchUgenCurrencyRate(ctx, b.db, currency, now.Format(time.DateOnly))
		if rateErr != nil {
			return b.db.HandleDatabaseError(rateErr, "ProcessUgenRenewals: fetch currency rate")
		}
		rateByCurrency[currency] = rate
	}

	tx, err := b.db.Begin(ctx)
	if err != nil {
		return b.db.HandleDatabaseError(err, "ProcessUgenRenewals: begin")
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	for _, pmt := range payments {
		if pmt.Status == config.SUBSCRIPTION_STATUS_CANCELED {
			continue
		}

		if pmt.CancelAtPeriodEnd && (pmt.Status == config.SUBSCRIPTION_STATUS_ACTIVE || pmt.Status == config.SUBSCRIPTION_STATUS_PENDING_DOWNGRADE) {
			// Explicit cancel at period end. ugen always has a free tier, so drop the
			// project onto the free plan and keep it active instead of blocking it.
			// Unlike the insolvency fallback below, no pending_fare_id is recorded: the
			// user opted out, so a later topup must not silently restore the paid plan.
			if _, err := tx.Exec(ctx, `
				UPDATE subscription
				SET status = $1,
				    updated_at = NOW()
				WHERE id = $2`,
				config.SUBSCRIPTION_STATUS_CANCELED, pmt.SubscriptionID,
			); err != nil {
				return b.db.HandleDatabaseError(err, "ProcessUgenRenewals: cancel subscription")
			}

			freeFareID, err := loadUgenFreeFareID(ctx, tx)
			if err != nil {
				return b.db.HandleDatabaseError(err, "ProcessUgenRenewals: load free fare")
			}

			freePeriod := defaultUgenFreePeriod()
			freeStartDate := now
			freeEndDate := addCalendarMonths(freeStartDate, freePeriod.months)
			freeSubscriptionID := uuid.NewString()
			if _, err := tx.Exec(ctx, `
				INSERT INTO subscription(
					id,
					project_id,
					fare_id,
					status,
					type,
					start_date,
					end_date,
					renewal_date,
					billing_period_code,
					billing_period_months,
					billing_period_discount_percent,
					cancel_at_period_end
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $7, $8, $9, $10, FALSE)`,
				freeSubscriptionID,
				pmt.ProjectID,
				freeFareID,
				config.SUBSCRIPTION_STATUS_ACTIVE,
				config.SUBSCRIPTION_PAID,
				formatDateOnly(freeStartDate),
				formatDateOnly(freeEndDate),
				freePeriod.code,
				freePeriod.months,
				freePeriod.discountPercent,
			); err != nil {
				return b.db.HandleDatabaseError(err, "ProcessUgenRenewals: create free subscription after cancel")
			}

			if err := chargeProjectInTx(ctx, tx, pmt.ProjectID, freeFareID, freeSubscriptionID, 0, 1, config.TransactionTypeSubscription); err != nil {
				return b.db.HandleDatabaseError(err, "ProcessUgenRenewals: activate free plan after cancel")
			}
			continue
		}

		period := &ugenBillingPeriod{
			code:            normalizeUgenBillingPeriodCode(pmt.BillingPeriodCode),
			months:          pmt.BillingPeriodMonths,
			discountPercent: pmt.BillingPeriodDiscount,
		}
		if period.months <= 0 {
			period.months = 1
		}

		rate := 1.0
		if pmt.CurrencyCode != config.CURRENCY_UZS {
			rate = rateByCurrency[pmt.CurrencyCode]
			if rate <= 0 {
				rate = 1.0
			}
		}

		charge := ugenPeriodCharge(pmt.Price, rate, period)
		restoreFromFree := pmt.Status == config.SUBSCRIPTION_STATUS_ACTIVE &&
			pmt.CurrentPrice == 0 &&
			pmt.PendingFareID != "" &&
			pmt.EffectiveFareID != pmt.FareID &&
			pmt.Price > 0
		if restoreFromFree {
			if pmt.Balance+pmt.CreditLimit < charge {
				currentEndDate, err := parseDateOnly(pmt.EndDate)
				if err != nil {
					return b.db.HandleDatabaseError(err, "ProcessUgenRenewals: parse free end date")
				}
				if !currentEndDate.After(now) {
					freePeriod := defaultUgenFreePeriod()
					freeStartDate := now
					freeEndDate := addCalendarMonths(freeStartDate, freePeriod.months)
					if _, err := tx.Exec(ctx, `
						UPDATE subscription
						SET start_date                       = $1,
						    end_date                         = $2,
						    renewal_date                     = $2,
						    billing_period_code              = $3,
						    billing_period_months            = $4,
						    billing_period_discount_percent  = $5,
						    updated_at                       = NOW()
						WHERE id = $6`,
						formatDateOnly(freeStartDate),
						formatDateOnly(freeEndDate),
						freePeriod.code,
						freePeriod.months,
						freePeriod.discountPercent,
						pmt.SubscriptionID,
					); err != nil {
						return b.db.HandleDatabaseError(err, "ProcessUgenRenewals: renew free fallback")
					}
				}
				continue
			}

			paidStartDate := now
			paidEndDate := addCalendarMonths(paidStartDate, period.months)
			if err := execInitialSubscription(
				ctx,
				tx,
				pmt.ProjectID,
				pmt.EffectiveFareID,
				charge,
				rate,
				formatDateOnly(paidStartDate),
				formatDateOnly(paidEndDate),
				period,
			); err != nil {
				return b.db.HandleDatabaseError(err, "ProcessUgenRenewals: restore paid plan")
			}
			continue
		}

		if pmt.Balance+pmt.CreditLimit < charge {
			if pmt.Status != config.SUBSCRIPTION_STATUS_EXPIRED {
				if _, err := tx.Exec(ctx, `
					UPDATE subscription
					SET status = $1,
					    updated_at = NOW()
					WHERE id = $2`,
					config.SUBSCRIPTION_STATUS_EXPIRED, pmt.SubscriptionID,
				); err != nil {
					return b.db.HandleDatabaseError(err, "ProcessUgenRenewals: expire subscription")
				}

				freeFareID, err := loadUgenFreeFareID(ctx, tx)
				if err != nil {
					return b.db.HandleDatabaseError(err, "ProcessUgenRenewals: load free fare")
				}

				freePeriod := defaultUgenFreePeriod()
				freeStartDate := now
				freeEndDate := addCalendarMonths(freeStartDate, freePeriod.months)
				freeSubscriptionID := uuid.NewString()
				if _, err := tx.Exec(ctx, `
					INSERT INTO subscription(
						id,
						project_id,
						fare_id,
						status,
						type,
						start_date,
						end_date,
						renewal_date,
						pending_fare_id,
						billing_period_code,
						billing_period_months,
						billing_period_discount_percent,
						cancel_at_period_end
					)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $7, $8, $9, $10, $11, FALSE)`,
					freeSubscriptionID,
					pmt.ProjectID,
					freeFareID,
					config.SUBSCRIPTION_STATUS_ACTIVE,
					config.SUBSCRIPTION_PAID,
					formatDateOnly(freeStartDate),
					formatDateOnly(freeEndDate),
					pmt.FareID,
					freePeriod.code,
					freePeriod.months,
					freePeriod.discountPercent,
				); err != nil {
					return b.db.HandleDatabaseError(err, "ProcessUgenRenewals: create free subscription")
				}

				if err := chargeProjectInTx(ctx, tx, pmt.ProjectID, freeFareID, freeSubscriptionID, 0, 1, config.TransactionTypeSubscription); err != nil {
					return b.db.HandleDatabaseError(err, "ProcessUgenRenewals: activate free plan")
				}
			}
			continue
		}

		startDate := now
		if pmt.Status != config.SUBSCRIPTION_STATUS_EXPIRED {
			parsedEndDate, err := parseDateOnly(pmt.EndDate)
			if err != nil {
				return b.db.HandleDatabaseError(err, "ProcessUgenRenewals: parse end date")
			}
			startDate = parsedEndDate
		}
		endDate := addCalendarMonths(startDate, period.months)

		if _, err := tx.Exec(ctx, `
			UPDATE subscription
			SET fare_id                         = $1,
			    status                          = $2,
			    type                            = $3,
			    start_date                      = $4,
			    end_date                        = $5,
			    renewal_date                    = $5,
			    pending_fare_id                 = NULL,
			    cancel_at_period_end            = FALSE,
			    canceled_at                     = NULL,
			    billing_period_code             = $6,
			    billing_period_months           = $7,
			    billing_period_discount_percent = $8,
			    updated_at                      = NOW()
			WHERE id = $9`,
			pmt.EffectiveFareID,
			config.SUBSCRIPTION_STATUS_ACTIVE,
			config.SUBSCRIPTION_PAID,
			formatDateOnly(startDate),
			formatDateOnly(endDate),
			period.code,
			period.months,
			period.discountPercent,
			pmt.SubscriptionID,
		); err != nil {
			return b.db.HandleDatabaseError(err, "ProcessUgenRenewals: renew subscription")
		}

		if charge == 0 {
			if _, err := tx.Exec(ctx, `
				UPDATE project
				SET fare_id = $1,
				    status = $2
				WHERE id = $3`,
				pmt.EffectiveFareID,
				config.STATUS_ACTIVE,
				pmt.ProjectID,
			); err != nil {
				return b.db.HandleDatabaseError(err, "ProcessUgenRenewals: activate no-charge plan")
			}
			continue
		}

		if err := chargeProjectInTx(ctx, tx, pmt.ProjectID, pmt.EffectiveFareID, pmt.SubscriptionID, charge, rate, config.TransactionTypeSubscription); err != nil {
			return b.db.HandleDatabaseError(err, "ProcessUgenRenewals: charge project")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return b.db.HandleDatabaseError(err, "ProcessUgenRenewals: commit")
	}

	log.Println("Ugen subscription renewals processed successfully.")
	return nil
}

func (b *billingRepo) UpsertMonthlyRequest(ctx context.Context, req *pb.MonthlyRequest) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.UpsertMonthlyRequest")
	defer dbSpan.Finish()

	now := time.Now()

	query := `
		INSERT INTO monthly_requests (
			project_id,
			requests,
			month,
			year
		) VALUES (
			$1,
			$2,
			$3,
			$4
		) ON CONFLICT (project_id, month, year) DO 
		UPDATE SET requests = monthly_requests.requests + $2
	`

	_, err := b.db.Exec(ctx, query,
		req.ProjectId,
		req.Count,
		MonthsMap[now.Month()],
		now.Year(),
	)
	if err != nil {
		return err
	}

	return nil
}

func (b *billingRepo) InactivateProjects(ctx context.Context) error {
	log.Println("* * * ** * * * - CHECKING PROJECTS FOR INACTIVATION - * * * ** * * *")

	query := `
		SELECT 
			p.id AS project_id
		FROM project p
		JOIN fare_item_price fip ON p.fare_id = fip.fare_id
		JOIN monthly_requests mr ON p.id = mr.project_id
		WHERE fip.item_type = 'request_per_month'
		AND mr.year = EXTRACT(YEAR FROM CURRENT_DATE)
		AND mr.month = EXTRACT(MONTH FROM CURRENT_DATE)
		AND CAST(fip.value AS INTEGER) < mr.requests;
	`

	rows, err := b.db.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	var projectsToInactivate []string
	for rows.Next() {
		var projectID string
		if err := rows.Scan(&projectID); err != nil {
			return err
		}
		projectsToInactivate = append(projectsToInactivate, projectID)
	}

	if len(projectsToInactivate) > 0 {
		updateQuery := `UPDATE project SET status = 'inactive' WHERE id = ANY($1)`
		_, err := b.db.Exec(ctx, updateQuery, projectsToInactivate)
		if err != nil {
			return err
		}

		log.Printf("PROJECTS %v HAVE BEEN SET SET INACTIVATE", projectsToInactivate)
	}

	return nil
}

// DeactivateDeadProjects retires "dead" projects: ucode projects that have sat in
// insufficient_funds with an active, overdue subscription for longer than
// BillingDeadProjectMonths calendar months. The renewal_date (parked in the past the
// moment a renewal could not be paid) is the start of the unpaid stretch, so a
// renewal_date older than the cutoff means the project has been unpaid that whole time.
// Such a project is flipped to inactive and its subscription expired, which also stops
// ProcessUcodeRenewals from re-touching it on the next tick.
func (b *billingRepo) DeactivateDeadProjects(ctx context.Context) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "DeactivateDeadProjects")
	defer span.Finish()

	log.Println("Checking for dead ucode projects to deactivate...")

	rows, err := b.db.Query(ctx, `
		SELECT DISTINCT ON (s.project_id)
			s.id,
			s.project_id
		FROM subscription s
		JOIN fare f ON s.fare_id = f.id
		JOIN project p ON p.id = s.project_id
		WHERE f.product_type = $1
		  AND s.status = $2
		  AND p.status = $3
		  AND s.renewal_date < CURRENT_DATE - make_interval(months => $4)
		ORDER BY s.project_id, s.created_at DESC`,
		config.PRODUCT_TYPE_UCODE,
		config.SUBSCRIPTION_STATUS_ACTIVE,
		config.STATUS_INSUFFICIENT_FUNDS,
		config.BillingDeadProjectMonths,
	)
	if err != nil {
		return b.db.HandleDatabaseError(err, "DeactivateDeadProjects: fetch dead projects")
	}
	defer rows.Close()

	type deadProject struct {
		subscriptionID string
		projectID      string
	}

	var candidates []deadProject
	for rows.Next() {
		var dp deadProject
		if err := rows.Scan(&dp.subscriptionID, &dp.projectID); err != nil {
			return b.db.HandleDatabaseError(err, "DeactivateDeadProjects: scan candidate")
		}
		candidates = append(candidates, dp)
	}
	if err := rows.Err(); err != nil {
		return b.db.HandleDatabaseError(err, "DeactivateDeadProjects: rows")
	}
	if len(candidates) == 0 {
		return nil
	}

	tx, err := b.db.Begin(ctx)
	if err != nil {
		return b.db.HandleDatabaseError(err, "DeactivateDeadProjects: begin")
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var deactivated []string
	for _, dp := range candidates {
		// Re-validate under a row lock so a concurrent post-topup renewal (which clears
		// insufficient_funds and pushes renewal_date forward) is not clobbered.
		var locked string
		err = tx.QueryRow(ctx, `
			SELECT s.id
			FROM subscription s
			JOIN fare f ON s.fare_id = f.id
			JOIN project p ON p.id = s.project_id
			WHERE s.id = $1
			  AND f.product_type = $2
			  AND s.status = $3
			  AND p.status = $4
			  AND s.renewal_date < CURRENT_DATE - make_interval(months => $5)
			FOR UPDATE`,
			dp.subscriptionID,
			config.PRODUCT_TYPE_UCODE,
			config.SUBSCRIPTION_STATUS_ACTIVE,
			config.STATUS_INSUFFICIENT_FUNDS,
			config.BillingDeadProjectMonths,
		).Scan(&locked)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return b.db.HandleDatabaseError(err, "DeactivateDeadProjects: lock subscription")
		}

		if _, err := tx.Exec(ctx, `
			UPDATE project SET status = $1 WHERE id = $2`,
			config.STATUS_INACTIVE, dp.projectID,
		); err != nil {
			return b.db.HandleDatabaseError(err, "DeactivateDeadProjects: deactivate project")
		}

		if _, err := tx.Exec(ctx, `
			UPDATE subscription
			SET status     = $1,
				updated_at = NOW()
			WHERE id = $2 AND status = $3`,
			config.SUBSCRIPTION_STATUS_EXPIRED,
			dp.subscriptionID,
			config.SUBSCRIPTION_STATUS_ACTIVE,
		); err != nil {
			return b.db.HandleDatabaseError(err, "DeactivateDeadProjects: expire subscription")
		}

		deactivated = append(deactivated, dp.projectID)
	}

	if err := tx.Commit(ctx); err != nil {
		return b.db.HandleDatabaseError(err, "DeactivateDeadProjects: commit")
	}

	if len(deactivated) > 0 {
		log.Printf("Deactivated %d dead project(s): %v", len(deactivated), deactivated)
	}
	return nil
}

func calculatePrice(ctx context.Context, info *PaymentInfo) (*pb.CalculatePriceResponse, error) {
	var (
		percentage, overAllPrice float64
		months                   int     = 1
		rate                     float64 = 1
	)

	if info.DiscountId != "" {
		percentage = info.DiscountValue
		months = int(info.DiscountMonths)
	}

	now := time.Now()
	startDate := now.Format(time.DateOnly)
	endDate := now.AddDate(0, months, -now.Day()).Format(time.DateOnly)
	daysOfMonth := daysInCurrentMonth(now)
	remainingDays := daysOfMonth - now.Day() + 1

	if info.CurrencyCode != config.CURRENCY_UZS {
		var err error
		rate, err = helper.FetchCurrencyRate(ctx, info.CurrencyCode, startDate)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch currency rate: %w", err)
		}
	}

	deductedPrice := info.Price * float64(remainingDays) / float64(daysOfMonth)

	overAllPriceWithoutRate := (deductedPrice + (float64(months)-1)*info.Price) * (1 - percentage/100)
	overAllPrice = ((deductedPrice + (float64(months)-1)*info.Price) * (1 - percentage/100)) * rate

	return &pb.CalculatePriceResponse{
		CalculatedPrice:            helper.RoundToTwoDecimals(overAllPrice, 2),
		ProjectBalance:             info.Balance,
		CreditLimit:                info.CreditLimit,
		DiscountPercentage:         percentage,
		StartDate:                  startDate,
		EndDate:                    endDate,
		CurrencyRate:               rate,
		CalculatedPriceWithoutRate: math.Ceil(overAllPriceWithoutRate * 100),
	}, nil
}
