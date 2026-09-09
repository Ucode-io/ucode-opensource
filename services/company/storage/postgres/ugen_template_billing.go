package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/opentracing/opentracing-go"
)

// ChargeProjectBalance debits the paying project's balance for a paid action
// (e.g. provisioning a project from a priced Ugen template). The project row is
// locked FOR UPDATE so a concurrent topup/renewal cannot race the overdraft
// check; the charge is funded by balance + credit_limit, mirroring subscription
// and token-pack charges. When external_id is set the charge is idempotent: a
// retry with the same key returns the original transaction instead of debiting
// twice (the project lock serializes the lookup-then-insert).
func (b *billingRepo) ChargeProjectBalance(ctx context.Context, req *pb.ChargeProjectBalanceRequest) (*pb.ChargeProjectBalanceResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "billing.ChargeProjectBalance")
	defer span.Finish()

	if req.GetProjectId() == "" {
		return nil, config.ErrProjectNotFound
	}
	if req.GetAmount() <= 0 {
		return nil, config.ErrInvalidChargeAmount
	}

	transactionType := req.GetTransactionType()
	if transactionType == "" {
		transactionType = config.TransactionTypeTemplate
	}

	// Convert the amount into UZS (project.balance is stored in UZS). UZS/empty -> rate 1.
	currencyCode, err := b.resolveCurrencyCode(ctx, req.GetCurrencyId())
	if err != nil {
		return nil, err
	}
	rate, err := fetchUgenCurrencyRate(ctx, b.db, currencyCode, time.Now().Format(time.DateOnly))
	if err != nil {
		return nil, b.db.HandleDatabaseError(err, "ChargeProjectBalance: fetch currency rate")
	}
	amountUZS := req.GetAmount() * rate

	tx, err := b.db.Begin(ctx)
	if err != nil {
		return nil, b.db.HandleDatabaseError(err, "ChargeProjectBalance: begin")
	}
	defer tx.Rollback(ctx)

	var balance, creditLimit float64
	if err := tx.QueryRow(ctx, `
		SELECT balance, credit_limit FROM project WHERE id = $1 FOR UPDATE`,
		req.GetProjectId(),
	).Scan(&balance, &creditLimit); err != nil {
		if err == pgx.ErrNoRows {
			return nil, config.ErrProjectNotFound
		}
		return nil, b.db.HandleDatabaseError(err, "ChargeProjectBalance: lock project")
	}

	// Idempotency: a retry with the same external_id returns the original charge.
	if req.GetExternalId() != "" {
		var (
			existingID     string
			existingAmount float64
		)
		err := tx.QueryRow(ctx, `
			SELECT id, amount FROM transaction
			WHERE external_id = $1 AND project_id = $2 AND transaction_type::text = $3
			LIMIT 1`,
			req.GetExternalId(), req.GetProjectId(), transactionType,
		).Scan(&existingID, &existingAmount)
		if err == nil {
			return &pb.ChargeProjectBalanceResponse{
				TransactionId:  existingID,
				ChargedAmount:  existingAmount,
				ProjectBalance: balance,
			}, nil
		}
		if err != pgx.ErrNoRows {
			return nil, b.db.HandleDatabaseError(err, "ChargeProjectBalance: idempotency lookup")
		}
	}

	if balance+creditLimit < amountUZS {
		return nil, config.ErrBalanceInsuffient
	}

	if _, err := tx.Exec(ctx, `
		UPDATE project SET balance = balance - $1 WHERE id = $2`,
		amountUZS, req.GetProjectId(),
	); err != nil {
		return nil, b.db.HandleDatabaseError(err, "ChargeProjectBalance: debit balance")
	}

	comment := req.GetComment()
	if comment == "" {
		comment = "project balance charge"
	}
	var creatorID any
	if req.GetCreatorId() != "" {
		creatorID = req.GetCreatorId()
	}

	transactionID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO transaction (id, project_id, creator_id, comment, amount, transaction_type,
		                         payment_status, creator_type, currency_id, payment_type, rate, external_id)
		VALUES ($1, $2, $3, $4, $5, $6, 'accepted', 'user', $7, 'Payme', $8, $9)`,
		transactionID, req.GetProjectId(), creatorID, comment,
		amountUZS, transactionType, config.UZS_CURRENCY_ID, rate, req.GetExternalId(),
	); err != nil {
		return nil, b.db.HandleDatabaseError(err, "ChargeProjectBalance: insert transaction")
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, b.db.HandleDatabaseError(err, "ChargeProjectBalance: commit")
	}

	return &pb.ChargeProjectBalanceResponse{
		TransactionId:  transactionID,
		ChargedAmount:  amountUZS,
		ProjectBalance: balance - amountUZS,
	}, nil
}

// RefundProjectBalance credits a previously charged amount back to the project,
// referencing the original charge transaction. It is the compensating reverse of
// ChargeProjectBalance and is idempotent per charge: the refund transaction is
// keyed by "<charge_id>:refund", so a repeated refund returns the existing one.
func (b *billingRepo) RefundProjectBalance(ctx context.Context, req *pb.RefundProjectBalanceRequest) (*pb.RefundProjectBalanceResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "billing.RefundProjectBalance")
	defer span.Finish()

	if req.GetProjectId() == "" || req.GetTransactionId() == "" {
		return nil, config.ErrTransactionNotFound
	}

	refundExternalID := req.GetTransactionId() + ":refund"

	tx, err := b.db.Begin(ctx)
	if err != nil {
		return nil, b.db.HandleDatabaseError(err, "RefundProjectBalance: begin")
	}
	defer tx.Rollback(ctx)

	var balance float64
	if err := tx.QueryRow(ctx, `
		SELECT balance FROM project WHERE id = $1 FOR UPDATE`,
		req.GetProjectId(),
	).Scan(&balance); err != nil {
		if err == pgx.ErrNoRows {
			return nil, config.ErrProjectNotFound
		}
		return nil, b.db.HandleDatabaseError(err, "RefundProjectBalance: lock project")
	}

	// Idempotency: if this charge was already refunded, return the existing refund.
	var existingID string
	err = tx.QueryRow(ctx, `
		SELECT id FROM transaction WHERE external_id = $1 AND project_id = $2 LIMIT 1`,
		refundExternalID, req.GetProjectId(),
	).Scan(&existingID)
	if err == nil {
		return &pb.RefundProjectBalanceResponse{
			TransactionId:  existingID,
			ProjectBalance: balance,
		}, nil
	}
	if err != pgx.ErrNoRows {
		return nil, b.db.HandleDatabaseError(err, "RefundProjectBalance: idempotency lookup")
	}

	// Load the original charge to mirror its amount, currency and creator.
	var (
		amount     float64
		creatorID  sql.NullString
		currencyID sql.NullString
		rate       sql.NullFloat64
		chargeType string
	)
	if err := tx.QueryRow(ctx, `
		SELECT amount, creator_id, currency_id, rate, transaction_type
		FROM transaction WHERE id = $1 AND project_id = $2`,
		req.GetTransactionId(), req.GetProjectId(),
	).Scan(&amount, &creatorID, &currencyID, &rate, &chargeType); err != nil {
		if err == pgx.ErrNoRows {
			return nil, config.ErrTransactionNotFound
		}
		return nil, b.db.HandleDatabaseError(err, "RefundProjectBalance: load charge")
	}
	// Only a charge (template import or user seat) is refundable, and each maps to
	// its own compensating transaction type.
	var refundType string
	switch chargeType {
	case config.TransactionTypeTemplate:
		refundType = config.TransactionTypeTemplateRfnd
	case config.TransactionTypeUserSeat:
		refundType = config.TransactionTypeUserSeatRfnd
	default:
		return nil, config.ErrTransactionNotRefundable
	}

	if _, err := tx.Exec(ctx, `
		UPDATE project SET balance = balance + $1 WHERE id = $2`,
		amount, req.GetProjectId(),
	); err != nil {
		return nil, b.db.HandleDatabaseError(err, "RefundProjectBalance: credit balance")
	}

	comment := req.GetComment()
	if comment == "" {
		comment = "refund for transaction " + req.GetTransactionId()
	}
	var creatorArg any
	if creatorID.Valid && creatorID.String != "" {
		creatorArg = creatorID.String
	}
	currencyArg := config.UZS_CURRENCY_ID
	if currencyID.Valid && currencyID.String != "" {
		currencyArg = currencyID.String
	}
	rateArg := 1.0
	if rate.Valid {
		rateArg = rate.Float64
	}

	refundID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO transaction (id, project_id, creator_id, comment, amount, transaction_type,
		                         payment_status, creator_type, currency_id, payment_type, rate, external_id)
		VALUES ($1, $2, $3, $4, $5, $6, 'accepted', 'user', $7, 'Payme', $8, $9)`,
		refundID, req.GetProjectId(), creatorArg, comment, amount,
		refundType, currencyArg, rateArg, refundExternalID,
	); err != nil {
		return nil, b.db.HandleDatabaseError(err, "RefundProjectBalance: insert refund")
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, b.db.HandleDatabaseError(err, "RefundProjectBalance: commit")
	}

	return &pb.RefundProjectBalanceResponse{
		TransactionId:  refundID,
		RefundedAmount: amount,
		ProjectBalance: balance + amount,
	}, nil
}

// resolveCurrencyCode maps a currency id to its ISO code. An empty id or unknown
// currency is treated as UZS (rate 1) so a missing currency never blocks a charge.
func (b *billingRepo) resolveCurrencyCode(ctx context.Context, currencyID string) (string, error) {
	if currencyID == "" {
		return "", nil
	}
	var code string
	if err := b.db.QueryRow(ctx, `SELECT code FROM currency WHERE id = $1`, currencyID).Scan(&code); err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", b.db.HandleDatabaseError(err, "resolveCurrencyCode")
	}
	return code, nil
}
