package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/storage/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/opentracing/opentracing-go"
)

var errUserSeatBillingSchemaMissing = errors.New("user seat billing schema is not migrated")

const (
	userSeatBillingStatusPaid              = "paid"
	userSeatBillingStatusInsufficientFunds = "insufficient_funds"
	userSeatBillingStatusFailed            = "failed"
	userSeatBillingStatusNoBillableSeats   = "no_billable_seats"
)

type userSeatRenewalProject struct {
	projectID     string
	headProjectID string
	title         string
	perUserPrice  float64
	currencyID    string
}

type userSeatBillingPeriodRecord struct {
	projectID         string
	headProjectID     string
	periodMonth       string
	totalUserCount    int32
	billableUserCount int32
	perUserPrice      float64
	currencyID        string
	amount            float64
	chargedAmount     float64
	transactionID     string
	externalID        string
	status            string
	lastError         string
}

func (b *billingRepo) ProcessUserSeatRenewals(ctx context.Context, countProjectUsers repo.ProjectUserCountFunc) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "billing.ProcessUserSeatRenewals")
	defer span.Finish()

	if countProjectUsers == nil {
		return errors.New("ProcessUserSeatRenewals: user count function is nil")
	}

	periodMonth := firstDayOfMonth(timeNow()).Format(time.DateOnly)
	projects, err := b.listUserSeatRenewalProjects(ctx)
	if err != nil {
		if errors.Is(err, errUserSeatBillingSchemaMissing) {
			return nil
		}
		return err
	}

	for _, project := range projects {
		externalID := fmt.Sprintf("user-seat-monthly:%s:%s", project.projectID, periodMonth[:7])

		existingStatus, err := b.getUserSeatBillingStatus(ctx, project.projectID, periodMonth)
		if err != nil {
			if errors.Is(err, errUserSeatBillingSchemaMissing) {
				return nil
			}
			return err
		}
		if existingStatus == userSeatBillingStatusPaid {
			continue
		}

		totalUsers, err := countProjectUsers(ctx, project.projectID)
		if err != nil {
			if err := b.upsertUserSeatBillingPeriod(ctx, userSeatBillingPeriodRecord{
				projectID:     project.projectID,
				headProjectID: project.headProjectID,
				periodMonth:   periodMonth,
				perUserPrice:  project.perUserPrice,
				currencyID:    project.currencyID,
				externalID:    externalID,
				status:        userSeatBillingStatusFailed,
				lastError:     truncateBillingError(err),
			}); err != nil {
				return err
			}
			continue
		}

		billableUsers := totalUsers - 1
		if billableUsers < 0 {
			billableUsers = 0
		}

		amount := float64(billableUsers) * project.perUserPrice
		if amount <= 0 {
			if err := b.upsertUserSeatBillingPeriod(ctx, userSeatBillingPeriodRecord{
				projectID:         project.projectID,
				headProjectID:     project.headProjectID,
				periodMonth:       periodMonth,
				totalUserCount:    totalUsers,
				billableUserCount: billableUsers,
				perUserPrice:      project.perUserPrice,
				currencyID:        project.currencyID,
				amount:            amount,
				externalID:        externalID,
				status:            userSeatBillingStatusNoBillableSeats,
			}); err != nil {
				return err
			}
			continue
		}

		charge, chargeErr := b.ChargeProjectBalance(ctx, &pb.ChargeProjectBalanceRequest{
			ProjectId:       project.headProjectID,
			Amount:          amount,
			CurrencyId:      project.currencyID,
			Comment:         fmt.Sprintf("monthly user seats: %s (%s)", project.title, periodMonth[:7]),
			TransactionType: config.TransactionTypeUserSeatMonthly,
			ExternalId:      externalID,
		})
		if chargeErr != nil {
			status := userSeatBillingStatusFailed
			if errors.Is(chargeErr, config.ErrBalanceInsuffient) {
				status = userSeatBillingStatusInsufficientFunds
			}

			if err := b.upsertUserSeatBillingPeriod(ctx, userSeatBillingPeriodRecord{
				projectID:         project.projectID,
				headProjectID:     project.headProjectID,
				periodMonth:       periodMonth,
				totalUserCount:    totalUsers,
				billableUserCount: billableUsers,
				perUserPrice:      project.perUserPrice,
				currencyID:        project.currencyID,
				amount:            amount,
				externalID:        externalID,
				status:            status,
				lastError:         truncateBillingError(chargeErr),
			}); err != nil {
				return err
			}
			continue
		}

		if err := b.upsertUserSeatBillingPeriod(ctx, userSeatBillingPeriodRecord{
			projectID:         project.projectID,
			headProjectID:     project.headProjectID,
			periodMonth:       periodMonth,
			totalUserCount:    totalUsers,
			billableUserCount: billableUsers,
			perUserPrice:      project.perUserPrice,
			currencyID:        project.currencyID,
			amount:            amount,
			chargedAmount:     charge.GetChargedAmount(),
			transactionID:     charge.GetTransactionId(),
			externalID:        externalID,
			status:            userSeatBillingStatusPaid,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (b *billingRepo) listUserSeatRenewalProjects(ctx context.Context) ([]userSeatRenewalProject, error) {
	rows, err := b.db.Query(ctx, `
		SELECT
			p.id::text,
			head.id::text,
			p.title,
			p.per_user_price,
			COALESCE(p.per_user_currency_id::text, '')
		FROM project p
		JOIN LATERAL (
			SELECT hp.id
			FROM project hp
			WHERE hp.company_id = p.company_id
			  AND hp.is_ugen = TRUE
			  AND hp.deleted_at IS NULL
			ORDER BY hp.created_at ASC
			LIMIT 1
		) head ON TRUE
		WHERE p.deleted_at IS NULL
		  AND COALESCE(p.is_ugen, FALSE) = FALSE
		  AND p.status::text = $1
		  AND p.per_user_price > 0`,
		config.STATUS_ACTIVE,
	)
	if err != nil {
		if isUserSeatBillingSchemaMissing(err) {
			return nil, errUserSeatBillingSchemaMissing
		}
		return nil, b.db.HandleDatabaseError(err, "ProcessUserSeatRenewals: list projects")
	}
	defer rows.Close()

	var projects []userSeatRenewalProject
	for rows.Next() {
		var project userSeatRenewalProject
		if err := rows.Scan(
			&project.projectID,
			&project.headProjectID,
			&project.title,
			&project.perUserPrice,
			&project.currencyID,
		); err != nil {
			return nil, b.db.HandleDatabaseError(err, "ProcessUserSeatRenewals: scan project")
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, b.db.HandleDatabaseError(err, "ProcessUserSeatRenewals: rows")
	}

	return projects, nil
}

func (b *billingRepo) getUserSeatBillingStatus(ctx context.Context, projectID, periodMonth string) (string, error) {
	var status string
	err := b.db.QueryRow(ctx, `
		SELECT status
		FROM project_user_seat_billing_period
		WHERE project_id = $1
		  AND period_month = $2`,
		projectID,
		periodMonth,
	).Scan(&status)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	if err != nil {
		if isUserSeatBillingSchemaMissing(err) {
			return "", errUserSeatBillingSchemaMissing
		}
		return "", b.db.HandleDatabaseError(err, "ProcessUserSeatRenewals: get billing period")
	}

	return status, nil
}

func (b *billingRepo) upsertUserSeatBillingPeriod(ctx context.Context, rec userSeatBillingPeriodRecord) error {
	if rec.status == "" {
		rec.status = userSeatBillingStatusFailed
	}
	if rec.externalID == "" {
		rec.externalID = uuid.NewString()
	}

	_, err := b.db.Exec(ctx, `
		INSERT INTO project_user_seat_billing_period (
			id,
			project_id,
			head_project_id,
			period_month,
			total_user_count,
			billable_user_count,
			per_user_price,
			currency_id,
			amount,
			charged_amount,
			transaction_id,
			external_id,
			status,
			last_error
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, '')::uuid, $9, $10, NULLIF($11, '')::uuid, $12, $13, $14)
		ON CONFLICT (project_id, period_month)
		DO UPDATE SET
			head_project_id      = EXCLUDED.head_project_id,
			total_user_count     = EXCLUDED.total_user_count,
			billable_user_count  = EXCLUDED.billable_user_count,
			per_user_price       = EXCLUDED.per_user_price,
			currency_id          = EXCLUDED.currency_id,
			amount               = EXCLUDED.amount,
			charged_amount       = EXCLUDED.charged_amount,
			transaction_id        = COALESCE(EXCLUDED.transaction_id, project_user_seat_billing_period.transaction_id),
			external_id          = EXCLUDED.external_id,
			status               = EXCLUDED.status,
			last_error           = EXCLUDED.last_error,
			updated_at           = NOW()`,
		uuid.NewString(),
		rec.projectID,
		rec.headProjectID,
		rec.periodMonth,
		rec.totalUserCount,
		rec.billableUserCount,
		rec.perUserPrice,
		rec.currencyID,
		rec.amount,
		rec.chargedAmount,
		rec.transactionID,
		rec.externalID,
		rec.status,
		rec.lastError,
	)
	if err != nil {
		if isUserSeatBillingSchemaMissing(err) {
			return errUserSeatBillingSchemaMissing
		}
		return b.db.HandleDatabaseError(err, "ProcessUserSeatRenewals: upsert billing period")
	}

	return nil
}

func firstDayOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

func truncateBillingError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if len(msg) <= 500 {
		return msg
	}
	return msg[:500]
}

func isUserSeatBillingSchemaMissing(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	return pgErr.Code == "42P01" || pgErr.Code == "42703"
}
