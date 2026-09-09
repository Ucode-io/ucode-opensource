package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/company/storage/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
)

type billingRepo struct {
	db     *Pool
	logger logger.Logger
	cfg    config.BaseConfig
}

// NewBillingRepo ...
func NewBillingRepo(db *Pool, log logger.Logger, cfg config.BaseConfig) repo.BillingStorageI {
	if log == nil && db != nil {
		log = db.logger
	}

	return &billingRepo{
		db:     db,
		logger: log,
		cfg:    cfg,
	}
}

func (b *billingRepo) CreateFare(ctx context.Context, fare *pb.CreateFareRequest) (*pb.Fare, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.Create")
	defer dbSpan.Finish()

	var (
		prices []*pb.FareItemPrice
	)

	tx, err := b.db.Begin(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	var (
		fareId = uuid.NewString()
		query  = `INSERT INTO fare (id, name, currency_id, price, trial_days, disactivate_day, description, is_public)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

		createdFare = &pb.Fare{}
	)

	_, err = b.db.Exec(ctx, query,
		fareId,
		fare.GetName(),
		fare.GetCurrencyId(),
		fare.GetPrice(),
		fare.GetTrialDays(),
		fare.GetDisactivateDay(),
		fare.GetDescription(),
		fare.GetIsPublic())
	if err != nil {
		return nil, fmt.Errorf("failed to insert fare: %w", err)
	}

	createdFare.Id = fareId
	createdFare.Name = fare.GetName()
	createdFare.CurrencyId = fare.GetCurrencyId()
	createdFare.Price = fare.GetPrice()
	createdFare.TrialDays = fare.GetTrialDays()
	createdFare.DisactivateDay = fare.GetDisactivateDay()
	createdFare.Description = fare.GetDescription()
	createdFare.IsPublic = fare.GetIsPublic()

	if len(fare.FareItemPrices) > 0 {
		for _, item := range fare.FareItemPrices {
			var (
				itemId = uuid.NewString()
				price  pb.FareItemPrice
			)

			insertQuery := `
				INSERT INTO fare_item_price (
					id, 
					fare_id, 
					fare_item_id, 
					value, 
					price, 
					over_limit_price,
					item_type
				) VALUES ($1, $2, $3, $4, $5, $6, (SELECT type FROM fare_item WHERE id = $3))
				RETURNING id, fare_item_id, value, price, over_limit_price`

			err = tx.QueryRow(ctx, insertQuery,
				itemId,
				fareId,
				item.FareItemId,
				item.Value,
				item.Price,
				item.OverLimitPrice).
				Scan(
					&price.Id,
					&price.FareItemId,
					&price.Value,
					&price.Price,
					&price.OverLimitPrice,
				)
			if err != nil {
				return nil, fmt.Errorf("failed to insert fare item price: %w", err)
			}

			prices = append(prices, &price)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	createdFare.FareItemPrices = prices

	return createdFare, nil
}

func (b *billingRepo) GetFareById(ctx context.Context, req *pb.PrimaryKey) (*pb.Fare, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.GetFareById")
	defer dbSpan.Finish()

	var (
		fare            = &pb.Fare{}
		fareItemsString = sql.NullString{}
		subscription    sql.NullString
		query           = `
		SELECT
			f.id,
			f.name,
			f.currency_id,
			f.price,
			f.trial_days,
			f.disactivate_day,
			f.description,
			f.is_public,
			f.product_type,
			COALESCE(json_agg(json_build_object(
				'id', fi.id,
				'fare_item_id', fi.fare_item_id,
				'value', fi.value,
				'price', fi.price,
				'over_limit_price', fi.over_limit_price,
				'fare_item', json_build_object(
					'id', fi.fare_item_id,
					'name', fi2.name,
					'parent_id', fi2.parent_id,
					'info', fi2.info,
					'type', fi2.type,
					'group', CASE WHEN fi3.id IS NOT NULL THEN json_build_object('id', fi3.id, 'name', fi3.name) ELSE NULL END
				)
			)) FILTER (WHERE fi.id IS NOT NULL), '[]') AS fare_item_prices,
			(SELECT row_to_json(s)
        		FROM (
            		SELECT id, project_id, status, start_date, end_date, created_at, updated_at
            		FROM subscription
            		WHERE fare_id = f.id AND project_id = $2
            		ORDER BY created_at DESC
            		LIMIT 1
        		) s
   			) AS latest_subscription
		FROM fare f
		LEFT JOIN fare_item_price fi ON f.id = fi.fare_id
		LEFT JOIN fare_item fi2 ON fi.fare_item_id = fi2.id
		LEFT JOIN fare_item fi3 ON fi2.parent_id = fi3.id
		WHERE f.id = $1 AND f.deleted_at = 0
		GROUP BY f.id
		`
	)

	err := b.db.QueryRow(ctx, query, req.Id, req.ProjectId).Scan(
		&fare.Id,
		&fare.Name,
		&fare.CurrencyId,
		&fare.Price,
		&fare.TrialDays,
		&fare.DisactivateDay,
		&fare.Description,
		&fare.IsPublic,
		&fare.ProductType,
		&fareItemsString,
		&subscription,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if err = json.Unmarshal([]byte(fareItemsString.String), &fare.FareItemPrices); err != nil {
		return nil, err
	}

	if subscription.Valid {
		if err = json.Unmarshal([]byte(subscription.String), &fare.Subscription); err != nil {
			return nil, err
		}
	}

	return fare, nil
}

func (b *billingRepo) GetListFares(ctx context.Context, queryParam *pb.ListRequest) (*pb.ListFareResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.GetListFares")
	defer dbSpan.Finish()

	var (
		response = &pb.ListFareResponse{}
		fares    = []*pb.Fare{}
		count    int32
		query    string
		args     []any
	)

	query = `
		SELECT
			COUNT(*) OVER() AS total_count,
			f.id,
			f.name,
			f.currency_id,
			f.price,
			f.trial_days,
			f.disactivate_day,
			f.description,
			f.is_public,
			f.product_type,
			COALESCE(json_agg(json_build_object(
				'id', fi.id,
				'fare_item_id', fi.fare_item_id,
				'value', fi.value,
				'price', fi.price,
				'over_limit_price', fi.over_limit_price,
				'fare_item', json_build_object(
					'id', fi.fare_item_id,
					'name', fi2.name,
					'parent_id', fi2.parent_id,
					'info', fi2.info,
					'type', fi2.type,
					'group', CASE WHEN fi3.id IS NOT NULL THEN json_build_object('id', fi3.id, 'name', fi3.name) ELSE NULL END
				)
			)) FILTER (WHERE fi.id IS NOT NULL), '[]') AS fare_item_prices
		FROM fare f
		LEFT JOIN fare_item_price fi ON f.id = fi.fare_id
		LEFT JOIN fare_item fi2 ON fi.fare_item_id = fi2.id
		LEFT JOIN fare_item fi3 ON fi2.parent_id = fi3.id
	`

	if queryParam.ProjectId != "" {
		query += `
		JOIN project p ON p.id = $3 AND f.id = p.fare_id
		WHERE f.deleted_at = 0
		GROUP BY f.id
		ORDER BY f.price ASC
		LIMIT $1 OFFSET $2`
		args = []any{queryParam.Limit, queryParam.Offset, queryParam.ProjectId}
	} else if queryParam.ProductType != "" {
		query += `
		WHERE f.deleted_at = 0
		  AND f.is_public = TRUE
		  AND f.product_type = $3::product_type
		GROUP BY f.id
		ORDER BY f.price ASC
		LIMIT $1 OFFSET $2`
		args = []any{queryParam.Limit, queryParam.Offset, queryParam.ProductType}
	} else {
		query += `
		WHERE f.deleted_at = 0 AND f.is_public = TRUE
		GROUP BY f.id
		ORDER BY f.price ASC
		LIMIT $1 OFFSET $2`
		args = []any{queryParam.Limit, queryParam.Offset}
	}

	rows, err := b.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			fare            pb.Fare
			fareItems       []*pb.FareItemPrice
			fareItemsString sql.NullString
		)

		err := rows.Scan(
			&count,
			&fare.Id,
			&fare.Name,
			&fare.CurrencyId,
			&fare.Price,
			&fare.TrialDays,
			&fare.DisactivateDay,
			&fare.Description,
			&fare.IsPublic,
			&fare.ProductType,
			&fareItemsString,
		)
		if err != nil {
			return nil, fmt.Errorf("row scanning failed: %w", err)
		}

		if fareItemsString.Valid {
			if err := json.Unmarshal([]byte(fareItemsString.String), &fareItems); err != nil {
				return nil, fmt.Errorf("failed to unmarshal fare item prices: %w", err)
			}
		}
		fare.FareItemPrices = fareItems
		fares = append(fares, &fare)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration failed: %w", err)
	}

	response.Fares = fares
	response.Count = count

	return response, nil
}

func (b *billingRepo) UpdateFare(ctx context.Context, fare *pb.Fare) (*pb.Fare, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.UpdateFare")
	defer dbSpan.Finish()

	tx, err := b.db.Begin(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	query := `UPDATE fare SET 
			name = $1, 
			currency_id = $2, 
			price = $3, 
			trial_days = $4, 
			disactivate_day = $5, 
			description = $6,
			is_public = $8
		WHERE id = $7`

	_, err = tx.Exec(ctx, query, fare.Name, fare.Currency, fare.Price, fare.TrialDays, fare.DisactivateDay, fare.Description, fare.Id, fare.IsPublic)
	if err != nil {
		return nil, err
	}

	if len(fare.FareItemPrices) > 0 {
		var (
			values       = []any{}
			placeholders = []string{}
		)

		for _, item := range fare.FareItemPrices {
			values = append(values, item.Id, fare.Id, item.FareItemId, item.Value, item.Price, item.OverLimitPrice)
			placeholders = append(placeholders, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d)", len(values)-5, len(values)-4, len(values)-3, len(values)-2, len(values)-1, len(values)))
		}

		insertQuery := fmt.Sprintf(`
		INSERT INTO fare_item_price (id, fare_id, fare_item_id, value, price, over_limit_price)
		VALUES %s
		ON CONFLICT (id) DO UPDATE SET
			fare_item_id = EXCLUDED.fare_item_id,
			value = EXCLUDED.value,
			price = EXCLUDED.price,
			over_limit_price = EXCLUDED.over_limit_price`, strings.Join(placeholders, ", "))

		_, err := tx.Exec(ctx, insertQuery, values...)
		if err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	return fare, nil
}

func (b *billingRepo) DeleteFare(ctx context.Context, req *pb.PrimaryKey) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "company.DeleteFare")
	defer dbSpan.Finish()

	result, err := b.db.Exec(ctx, `
		UPDATE 
			fare 
		SET deleted_at=DATE_PART('epoch', CURRENT_TIMESTAMP)::INT 
		WHERE id= $1 AND deleted_at = 0`, req.Id,
	)
	if err != nil {
		return err
	}

	if i := result.RowsAffected(); i == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (b *billingRepo) CreateFareItem(ctx context.Context, req *pb.CreateFareItemRequest) (*pb.FareItem, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.CreateFareItem")
	defer dbSpan.Finish()

	var (
		id               = uuid.NewString()
		item             = &pb.FareItem{}
		parentId     any = nil
		itemParentId sql.NullString
	)

	if req.ParentId != "" {
		parentId = req.ParentId
	}

	query := `
		INSERT INTO fare_item (id, name, parent_id, info, type)
		VALUES($1, $2, $3, $4, $5)
		RETURNING id, name, parent_id, info, type`

	err := b.db.QueryRow(ctx, query, id, req.Name, parentId, req.Info, req.Type).
		Scan(
			&item.Id,
			&item.Name,
			&itemParentId,
			&item.Info,
			&item.Type,
		)
	if err != nil {
		return nil, err
	}
	item.ParentId = itemParentId.String

	return item, nil
}

func (b *billingRepo) GetFareItem(ctx context.Context, req *pb.PrimaryKey) (*pb.FareItem, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.GetFareItem")
	defer dbSpan.Finish()

	var (
		item     = &pb.FareItem{}
		parentId sql.NullString
	)

	query := `SELECT id, name, parent_id, info, type
			FROM fare_item WHERE id = $1 AND deleted_at = 0`

	err := b.db.QueryRow(ctx, query, req.Id).
		Scan(
			&item.Id,
			&item.Name,
			&parentId,
			&item.Info,
			&item.Type,
		)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	item.ParentId = parentId.String

	return item, nil
}

func (b *billingRepo) ListFareItem(ctx context.Context, queryParam *pb.ListRequest) (*pb.ListFareItemsResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.ListFareItem")
	defer dbSpan.Finish()

	var (
		response = &pb.ListFareItemsResponse{}
		items    = []*pb.FareItem{}
		count    int32
		query    = `
		SELECT 
			COUNT(*) OVER() AS total_count,
			id, 
			name, 
			parent_id, 
			info,
			type
		FROM fare_item
		WHERE deleted_at = 0
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`
	)

	rows, err := b.db.Query(ctx, query, queryParam.Limit, queryParam.Offset)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			item     pb.FareItem
			parentId sql.NullString
		)

		err := rows.Scan(
			&count,
			&item.Id,
			&item.Name,
			&parentId,
			&item.Info,
			&item.Type,
		)
		if err != nil {
			return nil, fmt.Errorf("row scanning failed: %w", err)
		}

		item.ParentId = parentId.String
		items = append(items, &item)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration failed: %w", err)
	}

	response.FareItems = items
	response.Count = count

	return response, nil
}

func (b *billingRepo) UpdateFareItem(ctx context.Context, item *pb.FareItem) (*pb.FareItem, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.UpdateFareItem")
	defer dbSpan.Finish()

	var (
		updateParentId any = nil
		parentId       sql.NullString
	)

	tx, err := b.db.Begin(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	if item.ParentId != "" {
		updateParentId = item.ParentId
	}

	query := `UPDATE fare_item SET 
			name = $2, 
			parent_id = $3, 
			info = $4,
			type = $5,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, name, parent_id, info`

	err = tx.QueryRow(ctx, query, item.Id, item.Name, updateParentId, item.Info, item.Type).
		Scan(
			&item.Id,
			&item.Name,
			&parentId,
			&item.Info,
		)
	if err != nil {
		return nil, err
	}

	updateQuery := `UPDATE fare_item_price SET item_type = $2 WHERE fare_id = $1`
	_, err = tx.Exec(ctx, updateQuery, item.Id, item.Type)
	if err != nil {
		return nil, errors.Wrap(err, "failed to update fare item price")
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	item.ParentId = parentId.String

	return item, nil
}

func (b *billingRepo) DeleteFareItem(ctx context.Context, req *pb.PrimaryKey) error {
	result, err := b.db.Exec(ctx, `
		UPDATE 
			fare_item 
		SET deleted_at=DATE_PART('epoch', CURRENT_TIMESTAMP)::INT
		WHERE id= $1 AND deleted_at = 0`, req.Id,
	)
	if err != nil {
		return err
	}

	if i := result.RowsAffected(); i == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (b *billingRepo) CreateTransaction(ctx context.Context, req *pb.CreateTransactionRequest) (*pb.Transaction, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.CreateTransaction")
	defer dbSpan.Finish()

	return insertTransaction(ctx, b.db, req)
}

// insertTransaction writes a single transaction row. It runs against either the
// pool or an open tx so callers can record the transaction inside the same
// transaction that mutates the project balance (see ApplyTransaction).
func insertTransaction(ctx context.Context, q queryRower, req *pb.CreateTransactionRequest) (*pb.Transaction, error) {
	var subscriptionId any

	if req.SubscriptionId != "" {
		subscriptionId = req.SubscriptionId
	}

	var (
		transactionId = uuid.NewString()
		query         = `INSERT INTO transaction (
							id,
							project_id,
							creator_id,
							comment,
							payment_status,
							amount,
							transaction_type,
							creator_type,
							currency_id,
							fare_id,
							order_id,
							payment_type,
                         	subscription_id,
							external_id,
							rate,
							card_id
						) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, (SELECT fare_id FROM project WHERE id = $2), $10, $11, $12, $13, $14, $15)
						RETURNING
						id, project_id, creator_id, comment, payment_status, amount, transaction_type, creator_type,
						currency_id, fare_id, order_id, payment_type, subscription_id
						`
		createdTransaction = &pb.Transaction{}
		fareID             = sql.NullString{}
		cardId             any
	)

	if req.CardId != "" {
		cardId = req.CardId
	}

	err := q.
		QueryRow(
			ctx,
			query,
			transactionId,
			req.ProjectId,
			req.CreatorId,
			req.Comment,
			req.PaymentStatus,
			req.Amount,
			req.TransactionType,
			req.CreatorType,
			req.CurrencyId,
			req.OrderId,
			req.PaymentType,
			subscriptionId,
			req.ExternalId,
			req.Rate,
			cardId,
		).
		Scan(
			&createdTransaction.Id,
			&createdTransaction.ProjectId,
			&createdTransaction.CreatorId,
			&createdTransaction.Comment,
			&createdTransaction.PaymentStatus,
			&createdTransaction.Amount,
			&createdTransaction.TransactionType,
			&createdTransaction.CreatorType,
			&createdTransaction.CurrencyId,
			&fareID,
			&createdTransaction.OrderId,
			&createdTransaction.PaymentType,
			&subscriptionId,
		)

	createdTransaction.SubscriptionId = cast.ToString(subscriptionId)
	createdTransaction.FareId = fareID.String

	if err != nil {
		return nil, errors.Wrap(err, "failed to insert transaction")
	}

	return createdTransaction, nil
}

// ApplyTransaction records a transaction and mutates the project balance
// atomically. The project row is locked FOR UPDATE so a concurrent topup or
// renewal cannot race the overdraft check, and an insolvent subscription charge
// is rejected with config.ErrBalanceInsuffient (balance + credit_limit funds the
// charge, mirroring the renewal cron).
func (b *billingRepo) ApplyTransaction(ctx context.Context, req *pb.CreateTransactionRequest) (*pb.Transaction, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.ApplyTransaction")
	defer dbSpan.Finish()

	req.TransactionType = strings.ToLower(req.GetTransactionType())

	tx, err := b.db.Begin(ctx)
	if err != nil {
		return nil, b.db.HandleDatabaseError(err, "ApplyTransaction: begin")
	}
	defer tx.Rollback(ctx)

	var balance, creditLimit float64
	if err := tx.QueryRow(ctx, `
		SELECT balance, credit_limit
		FROM project
		WHERE id = $1
		FOR UPDATE`,
		req.GetProjectId(),
	).Scan(&balance, &creditLimit); err != nil {
		return nil, b.db.HandleDatabaseError(err, "ApplyTransaction: lock project")
	}

	var delta float64
	switch req.TransactionType {
	case config.TransactionTypeSubscription:
		if balance+creditLimit < req.GetAmount() {
			return nil, config.ErrBalanceInsuffient
		}
		delta = -req.GetAmount()
	case config.TransactionTypeTopup:
		delta = req.GetAmount()
	}

	var subscriptionId string
	if err := tx.QueryRow(ctx, `
		SELECT id FROM subscription
		WHERE project_id = $1
		  AND status IN ('active', 'pending_downgrade')
		ORDER BY created_at DESC
		LIMIT 1`,
		req.GetProjectId(),
	).Scan(&subscriptionId); err != nil && err != pgx.ErrNoRows {
		return nil, b.db.HandleDatabaseError(err, "ApplyTransaction: load subscription")
	}

	req.SubscriptionId = subscriptionId

	createdTransaction, err := insertTransaction(ctx, tx, req)
	if err != nil {
		return nil, err
	}

	if req.GetPaymentStatus() == config.PaymePaymentStatusAccepted && delta != 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE project SET balance = balance + $1 WHERE id = $2`,
			delta, req.GetProjectId(),
		); err != nil {
			return nil, b.db.HandleDatabaseError(err, "ApplyTransaction: update balance")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, b.db.HandleDatabaseError(err, "ApplyTransaction: commit")
	}

	return createdTransaction, nil
}

func (b *billingRepo) GetTransactionById(ctx context.Context, req *pb.PrimaryKey) (*pb.Transaction, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.GetTransactionById")
	defer dbSpan.Finish()

	var (
		transaction  = &pb.Transaction{}
		acceptedTime = sql.NullTime{}
		acceptorId   = sql.NullString{}
		fareId       = sql.NullString{}

		query = `
			SELECT 
				t.id, 
				t.project_id, 
				t.creator_id, 
				t.comment, 
				t.payment_status, 
				t.amount, 
				t.transaction_type, 
				t.creator_type, 
				t.currency_id, 
				t.acceptor_id, 
				t.fare_id, 
				t.accepted_at,
				t.receipt_file,
				t.order_id,
				t.payment_type
			FROM transaction t
			WHERE t.id = $1
		`
	)

	err := b.db.QueryRow(ctx, query, req.Id).Scan(
		&transaction.Id,
		&transaction.ProjectId,
		&transaction.CreatorId,
		&transaction.Comment,
		&transaction.PaymentStatus,
		&transaction.Amount,
		&transaction.TransactionType,
		&transaction.CreatorType,
		&transaction.CurrencyId,
		&acceptorId,
		&fareId,
		&acceptedTime,
		&transaction.ReceiptFile,
		&transaction.OrderId,
		&transaction.PaymentType,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction by id")
	}

	if acceptedTime.Valid {
		transaction.AcceptedAt = acceptedTime.Time.String()
	}
	transaction.AcceptorId = acceptorId.String
	transaction.FareId = fareId.String

	return transaction, nil
}

func (b *billingRepo) GetListTransactions(ctx context.Context, queryParam *pb.ListRequest) (*pb.ListTransactionsResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.GetListTransactions")
	defer dbSpan.Finish()

	var (
		response     = &pb.ListTransactionsResponse{}
		transactions = []*pb.Transaction{}
		count        int32

		baseQuery = `
		SELECT 
			COUNT(*) OVER() AS total_count,
			t.id, 
			t.project_id, 
			p.title AS project_name,
			t.creator_id, 
			t.comment, 
			t.payment_status, 
			t.amount, 
			t.transaction_type, 
			t.creator_type, 
			t.currency_id, 
			t.acceptor_id, 
			t.fare_id, 
			t.accepted_at,
			t.created_at,
			c.code AS currency_code,
			c.symbol AS currency_symbol,
			c.name AS currency_name,
			t.receipt_file,
			t.order_id,
			t.payment_type,
			f.name,
			t.external_id
		FROM transaction t
		LEFT JOIN currency c ON t.currency_id = c.id
		LEFT JOIN fare f ON t.fare_id = f.id
		LEFT JOIN project p on p.id = t.project_id
		`
		filter = ""
		args   = []any{queryParam.Limit, queryParam.Offset}
	)

	if queryParam.ProjectId != "" {
		filter = "WHERE t.project_id = $3"
		args = append(args, queryParam.ProjectId)
	}

	query := fmt.Sprintf("%s %s ORDER BY t.created_at DESC LIMIT $1 OFFSET $2", baseQuery, filter)

	rows, err := b.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			transaction                                                                               pb.Transaction
			acceptedTime                                                                              sql.NullTime
			acceptorId, createdAt, creatorId, fareName                                                sql.NullString
			currencyCode, currencySymbol, currencyName, receiptFile, paymentType, fareId, projectName sql.NullString
			currency                                                                                  pb.Currency
			orderId                                                                                   sql.NullInt64
		)

		err := rows.Scan(
			&count,
			&transaction.Id,
			&transaction.ProjectId,
			&projectName,
			&creatorId,
			&transaction.Comment,
			&transaction.PaymentStatus,
			&transaction.Amount,
			&transaction.TransactionType,
			&transaction.CreatorType,
			&transaction.CurrencyId,
			&acceptorId,
			&fareId,
			&acceptedTime,
			&createdAt,
			&currencyCode,
			&currencySymbol,
			&currencyName,
			&receiptFile,
			&orderId,
			&paymentType,
			&fareName,
			&transaction.ExternalId,
		)
		if err != nil {
			return nil, fmt.Errorf("row scanning failed: %w", err)
		}

		transaction.AcceptedAt = acceptedTime.Time.String()
		transaction.CreatedAt = createdAt.String
		transaction.CreatorId = creatorId.String
		transaction.AcceptorId = acceptorId.String
		currency.Code = currencyCode.String
		currency.Symbol = currencySymbol.String
		currency.Name = currencyName.String
		transaction.Currency = &currency
		transaction.ReceiptFile = receiptFile.String
		transaction.OrderId = orderId.Int64
		transaction.PaymentType = paymentType.String
		transaction.FareId = fareId.String
		transaction.ProjectName = projectName.String

		fare := &pb.Fare{
			Id:   fareId.String,
			Name: fareName.String,
		}

		transaction.Fare = fare
		transactions = append(transactions, &transaction)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration failed: %w", err)
	}

	response.Transactions = transactions
	response.Count = count

	return response, nil
}

func (b *billingRepo) UpdateTransaction(ctx context.Context, req *pb.Transaction) (*pb.Transaction, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.UpdateTransaction")
	defer dbSpan.Finish()

	tx, err := b.db.Begin(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	query := `
		UPDATE transaction SET 
			payment_status = $2, 
			comment = $3,
			external_id = $4
		WHERE id = $1
		RETURNING 
			id, project_id, creator_id, comment, payment_status, amount, transaction_type, creator_type, 
			currency_id, acceptor_id, fare_id, accepted_at, order_id, payment_type`

	var (
		transaction  = &pb.Transaction{}
		acceptedTime = sql.NullTime{}
		acceptorId   sql.NullString
		fareId       = sql.NullString{}
	)

	err = tx.QueryRow(ctx, query, req.Id, req.PaymentStatus, req.Comment, req.ExternalId).
		Scan(
			&transaction.Id,
			&transaction.ProjectId,
			&transaction.CreatorId,
			&transaction.Comment,
			&transaction.PaymentStatus,
			&transaction.Amount,
			&transaction.TransactionType,
			&transaction.CreatorType,
			&transaction.CurrencyId,
			&acceptorId,
			&fareId,
			&acceptedTime,
			&transaction.OrderId,
			&transaction.PaymentType,
		)
	if err != nil {
		return nil, errors.Wrap(err, "failed to update transaction")
	}

	if acceptedTime.Valid {
		transaction.AcceptedAt = acceptedTime.Time.String()
	}
	transaction.AcceptorId = acceptorId.String
	transaction.FareId = fareId.String

	if req.PaymentStatus == "accepted" {
		updateProjectQuery := `
			UPDATE project
			SET balance = balance + $1
			WHERE id = $2
		`
		_, err = tx.Exec(ctx, updateProjectQuery, transaction.Amount, transaction.ProjectId)
		if err != nil {
			return nil, errors.Wrap(err, "failed to update project balance")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to commit transaction")
	}

	return transaction, nil
}

// GetTransactionByExternalId loads a transaction by its provider transfer id. Used
// by the Ipak Yo'li confirm/status paths to match a bank callback or poll to the
// pending top-up that opened it.
func (b *billingRepo) GetTransactionByExternalId(ctx context.Context, externalId string) (*pb.Transaction, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.GetTransactionByExternalId")
	defer dbSpan.Finish()

	var (
		t       = &pb.Transaction{}
		orderId sql.NullInt64
	)
	err := b.db.QueryRow(ctx, `
		SELECT id, project_id, amount, payment_status, payment_type, currency_id, order_id, external_id
		FROM transaction
		WHERE external_id = $1
		ORDER BY created_at DESC
		LIMIT 1`, externalId).
		Scan(&t.Id, &t.ProjectId, &t.Amount, &t.PaymentStatus, &t.PaymentType, &t.CurrencyId, &orderId, &t.ExternalId)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction by external id")
	}
	t.OrderId = orderId.Int64
	return t, nil
}

// MarkIpakTransactionAccepted flips a still-pending Ipak transaction to accepted
// and credits the project balance in one DB transaction. The UPDATE is guarded by
// payment_status = 'pending', so when the callback, the frontend-return poll and
// the reconcile cron all confirm the same payment, exactly one of them updates a
// row and credits the balance. Returns the transaction and whether this call
// performed the credit (false means it was already accepted or is unknown).
func (b *billingRepo) MarkIpakTransactionAccepted(ctx context.Context, externalId string) (*pb.Transaction, bool, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.MarkIpakTransactionAccepted")
	defer dbSpan.Finish()

	tx, err := b.db.Begin(ctx)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	var (
		id, projectId string
		amount        float64
	)
	err = tx.QueryRow(ctx, `
		UPDATE transaction
		SET payment_status = $2
		WHERE external_id = $1 AND payment_status = $3
		RETURNING id, project_id, amount`,
		externalId, config.PaymePaymentStatusAccepted, config.PaymePaymentStatusPending,
	).Scan(&id, &projectId, &amount)
	if errors.Is(err, pgx.ErrNoRows) {
		// Already accepted/cancelled or unknown transfer -- nothing to credit.
		return nil, false, nil
	}
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to accept ipak transaction")
	}

	if _, err = tx.Exec(ctx, `UPDATE project SET balance = balance + $1 WHERE id = $2`, amount, projectId); err != nil {
		return nil, false, errors.Wrap(err, "failed to credit project balance")
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, false, errors.Wrap(err, "failed to commit ipak accept")
	}

	return &pb.Transaction{
		Id:            id,
		ProjectId:     projectId,
		Amount:        amount,
		ExternalId:    externalId,
		PaymentStatus: config.PaymePaymentStatusAccepted,
	}, true, nil
}

// MarkIpakTransactionCancelled flips a still-pending Ipak transaction to cancelled.
// Guarded by payment_status = 'pending' so it never overturns an accepted payment.
func (b *billingRepo) MarkIpakTransactionCancelled(ctx context.Context, externalId, comment string) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.MarkIpakTransactionCancelled")
	defer dbSpan.Finish()

	_, err := b.db.Exec(ctx, `
		UPDATE transaction
		SET payment_status = $2, comment = $3
		WHERE external_id = $1 AND payment_status = $4`,
		externalId, config.PaymePaymentStatusCancelled, comment, config.PaymePaymentStatusPending,
	)
	if err != nil {
		return errors.Wrap(err, "failed to cancel ipak transaction")
	}
	return nil
}

// ListPendingIpakTransactions returns Ipak top-up transactions still pending after
// olderThan, so the reconcile cron can re-check them via transfer.get (the bank
// callback fires only once and never retries).
func (b *billingRepo) ListPendingIpakTransactions(ctx context.Context, olderThan time.Duration) ([]*pb.Transaction, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.ListPendingIpakTransactions")
	defer dbSpan.Finish()

	rows, err := b.db.Query(ctx, `
		SELECT id, project_id, amount, external_id, order_id
		FROM transaction
		WHERE payment_type = $1
		  AND payment_status = $2
		  AND external_id <> ''
		  AND created_at <= NOW() - $3::interval
		ORDER BY created_at ASC
		LIMIT 100`,
		config.IpakYuliPaymentType,
		config.PaymePaymentStatusPending,
		fmt.Sprintf("%d seconds", int(olderThan.Seconds())),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list pending ipak transactions")
	}
	defer rows.Close()

	var out []*pb.Transaction
	for rows.Next() {
		var (
			t          = &pb.Transaction{}
			externalId sql.NullString
			orderId    sql.NullInt64
		)
		if err := rows.Scan(&t.Id, &t.ProjectId, &t.Amount, &externalId, &orderId); err != nil {
			return nil, errors.Wrap(err, "failed to scan pending ipak transaction")
		}
		t.ExternalId = externalId.String
		t.OrderId = orderId.Int64
		t.PaymentStatus = config.PaymePaymentStatusPending
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "pending ipak transactions iteration")
	}
	return out, nil
}

func parseFareValue(val string) int32 {
	val = strings.TrimSpace(val)
	if strings.HasSuffix(val, "GB") {
		return int32(cast.ToFloat64(strings.TrimSuffix(val, "GB")) * 1024)
	}
	if strings.HasSuffix(val, "MB") {
		return int32(cast.ToFloat64(strings.TrimSuffix(val, "MB")))
	}
	return cast.ToInt32(val)
}

func (b *billingRepo) CompareValue(ctx context.Context, req *pb.CompareFunctionRequest) (*pb.CompareFunctionResponse, error) {
	var (
		resp  = pb.CompareFunctionResponse{HasAccess: true}
		value sql.NullString
		query = `
		SELECT fip.value
		FROM fare_item_price fip
		JOIN fare_item fi ON fip.fare_item_id = fi.id
		WHERE fip.fare_id = $1 AND fi.type = $2 AND fip.deleted_at = 0
		`
	)

	err := b.db.QueryRow(ctx, query, req.FareId, req.Type).Scan(&value)
	if err != nil {
		if err == pgx.ErrNoRows {
			return &resp, nil
		}
		return nil, fmt.Errorf("query execution failed: %w", err)
	}

	count := parseFareValue(value.String)
	if count > 0 && count < req.Count {
		return &pb.CompareFunctionResponse{
			HasAccess: false,
		}, nil
	}

	return &resp, nil
}

func (b *billingRepo) CreateSubscription(ctx context.Context, req *pb.Subscription) (*pb.Subscription, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.CreateSubscription")
	defer dbSpan.Finish()

	var (
		id           = uuid.NewString()
		periodCode   = normalizeUgenBillingPeriodCode(req.BillingPeriodCode)
		periodMonths = req.BillingPeriodMonths
	)
	if periodMonths <= 0 {
		periodMonths = 1
	}

	query := fmt.Sprintf(`
		INSERT INTO subscription(
			id,
			project_id,
			fare_id,
			status,
			start_date,
			end_date,
			renewal_date,
			type,
			billing_period_code,
			billing_period_months,
			billing_period_discount_percent,
			cancel_at_period_end
		) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING %s
	`, subscriptionSelectColumns())

	resp, err := scanSubscription(b.db.QueryRow(ctx, query,
		id,
		req.ProjectId,
		req.FareId,
		req.Status,
		req.StartDate,
		req.EndDate,
		req.RenewalDate,
		req.Type,
		periodCode,
		periodMonths,
		req.BillingPeriodDiscountPercent,
		req.CancelAtPeriodEnd,
	))
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (b *billingRepo) GetSubscriptionById(ctx context.Context, in *pb.PrimaryKey) (*pb.Subscription, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.GetSubscriptionById")
	defer dbSpan.Finish()

	query := fmt.Sprintf(`
		SELECT %s
		FROM subscription
		WHERE project_id = $1
		  AND status IN ('active', 'pending_downgrade')
		ORDER BY created_at DESC
		LIMIT 1
	`, subscriptionSelectColumns())

	subscription, err := scanSubscription(b.db.QueryRow(ctx, query, in.ProjectId))
	if err != nil {
		return nil, err
	}

	return subscription, nil
}

func (b *billingRepo) UpdateSubscription(ctx context.Context, in *pb.Subscription) (*pb.Subscription, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.GetSubscriptionById")
	defer dbSpan.Finish()

	query := fmt.Sprintf(`
		UPDATE subscription SET
			fare_id = $1,
			status = $2,
			start_date = CURRENT_DATE,
			end_date = $3,
			renewal_date = $4,
			type = $5,
			updated_at = NOW()
		WHERE id = $6
		RETURNING %s
	`, subscriptionSelectColumns())

	subscription, err := scanSubscription(b.db.QueryRow(ctx, query, in.FareId, in.Status, in.EndDate, in.RenewalDate, in.Type, in.Id))
	if err != nil {
		return nil, err
	}

	return subscription, nil
}

func (b *billingRepo) CancelSubscription(ctx context.Context, in *pb.CancelSubscriptionRequest) (*pb.Subscription, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.CancelSubscription")
	defer dbSpan.Finish()

	tx, err := b.db.Begin(ctx)
	if err != nil {
		return nil, b.db.HandleDatabaseError(err, "CancelSubscription: begin")
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var (
		subscriptionID string
		productType    string
	)
	if err := tx.QueryRow(ctx, `
		SELECT s.id, f.product_type
		FROM subscription s
		JOIN fare f ON f.id = s.fare_id
		WHERE s.project_id = $1
		  AND s.status IN ('active', 'pending_downgrade')
		ORDER BY s.created_at DESC
		LIMIT 1
		FOR UPDATE`,
		in.ProjectId,
	).Scan(&subscriptionID, &productType); err != nil {
		return nil, b.db.HandleDatabaseError(err, "CancelSubscription: load subscription")
	}

	if !isUgenProduct(productType) {
		return nil, config.ErrUcodePlanChangeNotAllowed
	}

	query := fmt.Sprintf(`
		UPDATE subscription
		SET cancel_at_period_end = TRUE,
		    canceled_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING %s
	`, subscriptionSelectColumns())

	subscription, err := scanSubscription(tx.QueryRow(ctx, query, subscriptionID))
	if err != nil {
		return nil, b.db.HandleDatabaseError(err, "CancelSubscription: update subscription")
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, b.db.HandleDatabaseError(err, "CancelSubscription: commit")
	}

	return subscription, nil
}

func (b *billingRepo) GetSubscriptionByProjectId(ctx context.Context, req string) (string, error) {

	var (
		query = `
			SELECT id FROM subscription
			WHERE project_id = $1
			  AND status IN ('active', 'pending_downgrade')
			ORDER BY created_at DESC
			LIMIT 1`
		response string
	)

	err := b.db.QueryRow(ctx, query, req).Scan(&response)
	if err != nil {
		return "", err
	}

	return response, nil
}

func (b *billingRepo) GetProjectBillingStatus(ctx context.Context, req *pb.GetProjectBillingStatusRequest) (*pb.GetProjectBillingStatusResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "billing.GetProjectBillingStatus")
	defer span.Finish()

	var (
		projectStatus        string
		balance, creditLimit float64
		subscriptionStatus   sql.NullString
		renewalDate          sql.NullString
		periodMonths         sql.NullInt32
		periodDiscount       sql.NullFloat64
		farePrice            sql.NullFloat64
		currencyCode         sql.NullString
		productType          sql.NullString
	)

	// Pull the project together with its current billing subscription (active or
	// pending_downgrade). The effective fare is the one the next renewal will charge:
	// the pending fare for a scheduled downgrade, otherwise the active fare.
	query := `
		SELECT
			p.status,
			p.balance,
			p.credit_limit,
			s.status,
			s.renewal_date::DATE::TEXT,
			s.billing_period_months,
			s.billing_period_discount_percent,
			ef.price,
			c.code,
			ef.product_type
		FROM project p
		LEFT JOIN LATERAL (
			SELECT
				sub.status,
				sub.renewal_date,
				sub.fare_id,
				sub.pending_fare_id,
				sub.billing_period_months,
				sub.billing_period_discount_percent
			FROM subscription sub
			WHERE sub.project_id = p.id
			  AND sub.status IN ('active', 'pending_downgrade')
			ORDER BY sub.created_at DESC
			LIMIT 1
		) s ON TRUE
		LEFT JOIN fare ef ON ef.id = CASE
			WHEN s.status = 'pending_downgrade' AND s.pending_fare_id IS NOT NULL THEN s.pending_fare_id
			ELSE s.fare_id
		END
		LEFT JOIN currency c ON c.id = ef.currency_id
		WHERE p.id = $1`

	err := b.db.QueryRow(ctx, query, req.ProjectId).Scan(
		&projectStatus,
		&balance,
		&creditLimit,
		&subscriptionStatus,
		&renewalDate,
		&periodMonths,
		&periodDiscount,
		&farePrice,
		&currencyCode,
		&productType,
	)
	if err != nil {
		return nil, b.db.HandleDatabaseError(err, "GetProjectBillingStatus: load project billing state")
	}

	resp := &pb.GetProjectBillingStatusResponse{
		ProjectStatus:      projectStatus,
		SubscriptionStatus: subscriptionStatus.String,
		ProductType:        productType.String,
		RenewalDate:        renewalDate.String,
		CurrencyCode:       currencyCode.String,
		ProjectBalance:     balance,
		CreditLimit:        creditLimit,
		AvailableFunds:     balance + creditLimit,
	}

	// A free plan (price 0) or a project without a billing subscription has no upcoming
	// charge, so there is nothing to warn about.
	if !farePrice.Valid || farePrice.Float64 <= 0 || renewalDate.String == "" {
		return resp, nil
	}

	rate, err := fetchUgenCurrencyRate(ctx, b.db, currencyCode.String, time.Now().Format(time.DateOnly))
	if err != nil {
		return nil, b.db.HandleDatabaseError(err, "GetProjectBillingStatus: fetch currency rate")
	}

	months := periodMonths.Int32
	if months <= 0 {
		months = 1
	}

	// ucode is billed one calendar month at a time. ugen charges the whole billing
	// period up front and applies the period discount.
	nextCharge := farePrice.Float64 * rate
	if productType.String == config.PRODUCT_TYPE_UGEN {
		nextCharge = farePrice.Float64 * float64(months) * (1 - periodDiscount.Float64/100) * rate
	}
	resp.NextCharge = helper.RoundToTwoDecimals(nextCharge, 2)

	renewal, err := parseDateOnly(renewalDate.String)
	if err != nil {
		return nil, b.db.HandleDatabaseError(err, "GetProjectBillingStatus: parse renewal date")
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	resp.DaysUntilRenewal = int32(math.Round(renewal.Sub(today).Hours() / 24))

	if resp.DaysUntilRenewal <= int32(config.BillingRenewalWarningDays) && resp.AvailableFunds < resp.NextCharge {
		resp.LowBalanceWarning = true
		resp.Shortfall = helper.RoundToTwoDecimals(resp.NextCharge-resp.AvailableFunds, 2)
	}

	return resp, nil
}

func (b *billingRepo) CalculatePrice(ctx context.Context, in *pb.CalculatePriceRequest) (*pb.CalculatePriceResponse, error) {
	var (
		balance, creditLimit                float64
		fareId, currencyCode                sql.NullString
		farePrice, percentage, overAllPrice float64
		months                              int     = 1
		rate                                float64 = 1
	)

	query := `
	SELECT
		fare_id,
		balance,
		credit_limit
	FROM
		project
	WHERE id = $1`

	err := b.db.QueryRow(ctx, query, in.ProjectId).Scan(&fareId, &balance, &creditLimit)
	if err != nil {
		return nil, b.db.HandleDatabaseError(err, "get project balance info")
	}

	query = `
	SELECT 
		f.price, 
		c.code 
	FROM fare f
	JOIN currency c ON c.id = f.currency_id
	WHERE f.id = $1`

	err = b.db.QueryRow(ctx, query, in.FareId).Scan(&farePrice, &currencyCode)
	if err != nil {
		return nil, b.db.HandleDatabaseError(err, "get project balance info")
	}

	if in.DiscountId != "" {
		query = `SELECT months, value FROM discount WHERE id = $1`
		err = b.db.QueryRow(ctx, query, in.DiscountId).Scan(&months, &percentage)
		if err != nil {
			return nil, b.db.HandleDatabaseError(err, "get project balance info")
		}
	}

	now := time.Now()
	startDate := now.Format(time.DateOnly)
	endDate := now.AddDate(0, months, -now.Day()).Format(time.DateOnly)
	daysOfMonth := daysInCurrentMonth(now)
	remainingDays := daysOfMonth - now.Day() + 1

	if currencyCode.String != config.CURRENCY_UZS {
		rate, err = helper.FetchCurrencyRate(ctx, currencyCode.String, startDate)
		if err != nil {
			return nil, b.db.HandleDatabaseError(err, "rate error")
		}
	}

	deductedPrice := farePrice * float64(remainingDays) / float64(daysOfMonth)

	overAllPrice = ((deductedPrice + (float64(months)-1)*farePrice) * (1 - percentage/100)) * rate

	return &pb.CalculatePriceResponse{
		CalculatedPrice:    helper.RoundToTwoDecimals(overAllPrice, 2),
		ProjectBalance:     balance,
		CreditLimit:        creditLimit,
		DiscountPercentage: percentage,
		StartDate:          startDate,
		EndDate:            endDate,
		CurrencyRate:       rate,
	}, nil
}

func (b *billingRepo) ListDiscounts(ctx context.Context, req *pb.ListRequest) (*pb.ListDiscountsResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.ListDiscounts")
	defer dbSpan.Finish()

	var (
		discounts = []*pb.Discount{}
		count     int32

		query = `
		SELECT 
			COUNT(*) OVER() AS total_count,
			id, 
			months, 
			value
		FROM discount
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`
		args = []any{req.Limit, req.Offset}
	)

	rows, err := b.db.Query(ctx, query, args...)
	if err != nil {
		return nil, b.db.HandleDatabaseError(err, "query execution failed")
	}
	defer rows.Close()

	for rows.Next() {
		var discount pb.Discount

		err := rows.Scan(
			&count,
			&discount.Id,
			&discount.Months,
			&discount.Value,
		)
		if err != nil {
			return nil, b.db.HandleDatabaseError(err, "row scanning failed")
		}

		discounts = append(discounts, &discount)
	}

	if err = rows.Err(); err != nil {
		return nil, b.db.HandleDatabaseError(err, "row iteration failed")
	}

	return &pb.ListDiscountsResponse{
		Discounts: discounts,
		Count:     count,
	}, nil
}

func (b *billingRepo) ListBillingPeriods(ctx context.Context, req *pb.ListBillingPeriodsRequest) (*pb.ListBillingPeriodsResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.ListBillingPeriods")
	defer dbSpan.Finish()

	if req.ProductType != "" && req.ProductType != config.PRODUCT_TYPE_UGEN {
		return &pb.ListBillingPeriodsResponse{}, nil
	}

	rows, err := b.db.Query(ctx, `
		SELECT code, name, months, discount_percent, is_active, sort_order
		FROM ugen_billing_period
		WHERE is_active = TRUE
		ORDER BY sort_order ASC`)
	if err != nil {
		return nil, b.db.HandleDatabaseError(err, "ListBillingPeriods: query")
	}
	defer rows.Close()

	resp := &pb.ListBillingPeriodsResponse{}
	for rows.Next() {
		period := &pb.BillingPeriod{}
		if err := rows.Scan(
			&period.Code,
			&period.Name,
			&period.Months,
			&period.DiscountPercent,
			&period.IsActive,
			&period.SortOrder,
		); err != nil {
			return nil, b.db.HandleDatabaseError(err, "ListBillingPeriods: scan")
		}
		resp.BillingPeriods = append(resp.BillingPeriods, period)
	}
	if err := rows.Err(); err != nil {
		return nil, b.db.HandleDatabaseError(err, "ListBillingPeriods: rows")
	}

	return resp, nil
}

func (b *billingRepo) AddBalanceToProject(ctx context.Context, projectId string, amount float64) error {
	updateProjectQuery := `
			UPDATE project
			SET balance = balance + $1
			WHERE id = $2
		`
	_, err := b.db.Exec(ctx, updateProjectQuery, amount, projectId)
	if err != nil {
		return errors.Wrap(err, "failed to update project balance")
	}

	return nil
}

func (b *billingRepo) UpdateSubscriptionEndDate(ctx context.Context, req *pb.UpdateSubscriptionEndDateReq) (*pb.UpdateSubscriptionEndDateResp, error) {

	var query = fmt.Sprintf(
		`SELECT activate_project_subscription('%s','%s')`,
		req.ProjectId, req.EndDate,
	)

	_, err := b.db.Exec(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to update subscription end date")
	}

	return &pb.UpdateSubscriptionEndDateResp{
		ProjectId: req.ProjectId,
		EndDate:   req.EndDate,
		Status:    "success",
	}, nil
}

func (b *billingRepo) GetPricingLimits(ctx context.Context, req *pb.GetPricingLimitsRequest) (*pb.GetPricingLimitsResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.GetPricingLimits")
	defer dbSpan.Finish()

	query := `
		SELECT 
			fi.type,
			fi.name,
			COALESCE(fip.value, '0')  AS value,
			COALESCE(fip.price, 0)    AS price
		FROM subscription s
		JOIN fare f             ON s.fare_id = f.id           AND f.deleted_at = 0
		JOIN fare_item_price fip ON f.id = fip.fare_id        AND fip.deleted_at = 0
		JOIN fare_item fi        ON fip.fare_item_id = fi.id  AND fi.deleted_at = 0
		WHERE s.project_id = $1
		  AND s.status IN ('active', 'pending_downgrade')
		ORDER BY fi.name
	`

	rows, err := b.db.Query(ctx, query, req.ProjectId)
	if err != nil {
		return nil, fmt.Errorf("GetPricingLimits query: %w", err)
	}
	defer rows.Close()

	var limits []*pb.ResourceLimit
	for rows.Next() {
		var limit pb.ResourceLimit
		if err := rows.Scan(
			&limit.Type,
			&limit.Name,
			&limit.Value,
			&limit.Price,
		); err != nil {
			return nil, fmt.Errorf("GetPricingLimits scan: %w", err)
		}
		limits = append(limits, &limit)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetPricingLimits rows: %w", err)
	}

	return &pb.GetPricingLimitsResponse{Limits: limits}, nil
}

func (b *billingRepo) LogUsage(ctx context.Context, req *pb.LogUsageRequest) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.LogUsage")
	defer dbSpan.Finish()

	query := `
		INSERT INTO billing_usage (
			project_id,
			date_time,
			time_range,
			count
		) VALUES ($1, $2, $3, $4)
		ON CONFLICT (project_id, date_time, time_range) 
		DO UPDATE SET count = billing_usage.count + EXCLUDED.count
	`

	_, err := b.db.Exec(ctx, query,
		req.ProjectId,
		req.DateTime,
		req.TimeRange,
		req.Count,
	)
	if err != nil {
		b.logger.Error("failed to log usage", logger.Error(err))
	}
	return err
}

func (b *billingRepo) GetApiCallMonitoringMetrics(ctx context.Context, req *pb.GetApiCallMonitoringMetricsRequest) (*pb.GetApiCallMonitoringMetricsResponse, error) {
	resp := &pb.GetApiCallMonitoringMetricsResponse{
		DailyUsageChart: make([]*pb.MonitoringDailyUsage, 0),
	}

	// 1. Total monthly calls (from start of current UTC month)
	queryMonth := `
		SELECT COALESCE(SUM(count), 0)
		FROM billing_usage
		WHERE project_id = $1 
		  AND date_time >= DATE_TRUNC('month', now())
	`
	err := b.db.QueryRow(ctx, queryMonth, req.ProjectId).Scan(&resp.TotalMonthlyCalls)
	if err != nil {
		return nil, err
	}

	// 2. Total last day calls (last 24 hours relative to now)
	queryLastDay := `
		SELECT COALESCE(SUM(count), 0)
		FROM billing_usage
		WHERE project_id = $1 
		  AND date_time >= timezone('UTC', now()) - interval '24 hours'
	`
	err = b.db.QueryRow(ctx, queryLastDay, req.ProjectId).Scan(&resp.TotalLastDayCalls)
	if err != nil {
		return nil, err
	}

	// 3. Daily usage chart (last 7 days grouped by Date)
	queryChart := `
		SELECT DATE(date_time), COALESCE(SUM(count), 0)
		FROM billing_usage
		WHERE project_id = $1 
		  AND date_time >= DATE_TRUNC('day', timezone('UTC', now())) - interval '6 days'
		GROUP BY DATE(date_time)
		ORDER BY DATE(date_time) ASC
	`
	rows, err := b.db.Query(ctx, queryChart, req.ProjectId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var date time.Time
		var count int64
		if err := rows.Scan(&date, &count); err != nil {
			return nil, err
		}
		resp.DailyUsageChart = append(resp.DailyUsageChart, &pb.MonitoringDailyUsage{
			Date:  date.Format("2006-01-02"),
			Count: count,
		})
	}

	return resp, nil
}

func (b *billingRepo) RecordAiTokenUsage(ctx context.Context, req *pb.RecordAiTokenUsageRequest) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.RecordAiTokenUsage")
	defer dbSpan.Finish()

	var (
		id = uuid.NewString()

		projectID, companyID *string

		product = req.Product
	)

	if req.ProjectId != "" {
		projectID = &req.ProjectId
	}
	if req.CompanyId != "" {
		companyID = &req.CompanyId
	}
	if product != config.PRODUCT_TYPE_UCODE && product != config.PRODUCT_TYPE_UGEN {
		product = config.PRODUCT_TYPE_UGEN
	}

	packTokens := req.GetPackTokens()
	if packTokens < 0 {
		packTokens = 0
	}
	totalTokens := int64(req.GetInputTokens()) + int64(req.GetOutputTokens())
	if packTokens > totalTokens {
		packTokens = totalTokens
	}

	insertUsage := `
		INSERT INTO ai_token_usage (id, project_id, company_id, input_tokens, output_tokens, model, description, product, pack_tokens)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::product_type, $9)`

	// When no pack tokens were spent, a single insert is enough.
	if packTokens == 0 || companyID == nil {
		_, err := b.db.Exec(ctx, insertUsage,
			id, projectID, companyID, req.InputTokens, req.OutputTokens, req.Model, req.Description, product, packTokens)
		if err != nil {
			b.logger.Error("failed to record AI token usage", logger.Error(err))
		}
		return err
	}

	// Pack tokens were spent: insert the usage row and debit the company pool
	// atomically so the breakdown ledger and the pool can never diverge. The
	// pool is clamped at zero to stay within the remaining_tokens >= 0 CHECK.
	tx, err := b.db.Begin(ctx)
	if err != nil {
		b.logger.Error("failed to begin AI token usage tx", logger.Error(err))
		return err
	}
	defer tx.Rollback(ctx)

	var remainingTokens int64
	if err := tx.QueryRow(ctx, `
		SELECT remaining_tokens
		FROM company_token_balance
		WHERE company_id = $1
		FOR UPDATE`,
		*companyID,
	).Scan(&remainingTokens); err != nil {
		if err != pgx.ErrNoRows {
			b.logger.Error("failed to lock company token balance", logger.Error(err))
			return err
		}
		remainingTokens = 0
	}

	effectivePackTokens := packTokens
	if effectivePackTokens > remainingTokens {
		effectivePackTokens = remainingTokens
	}
	if effectivePackTokens < 0 {
		effectivePackTokens = 0
	}

	if _, err := tx.Exec(ctx, insertUsage,
		id, projectID, companyID, req.InputTokens, req.OutputTokens, req.Model, req.Description, product, effectivePackTokens); err != nil {
		b.logger.Error("failed to record AI token usage", logger.Error(err))
		return err
	}

	if effectivePackTokens > 0 {
		if _, err := tx.Exec(ctx, `
		UPDATE company_token_balance
		SET remaining_tokens = GREATEST(remaining_tokens - $2, 0), updated_at = NOW()
		WHERE company_id = $1`,
			*companyID, effectivePackTokens); err != nil {
			b.logger.Error("failed to debit company token balance", logger.Error(err))
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		b.logger.Error("failed to commit AI token usage tx", logger.Error(err))
		return err
	}
	return nil
}

func (b *billingRepo) GetAiTokenUsageMetrics(ctx context.Context, req *pb.GetAiTokenUsageMetricsRequest) (*pb.GetAiTokenUsageMetricsResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.GetAiTokenUsageMetrics")
	defer dbSpan.Finish()

	var (
		resp = &pb.GetAiTokenUsageMetricsResponse{}

		filters []string
		args    []any
	)

	if req.ProjectId != "" {
		args = append(args, req.ProjectId)
		filters = append(filters, fmt.Sprintf("project_id = $%d", len(args)))
	}
	if req.CompanyId != "" {
		args = append(args, req.CompanyId)
		filters = append(filters, fmt.Sprintf("company_id = $%d", len(args)))
	}

	where := strings.Join(filters, " AND ")

	// pack_tokens records how many of a row's tokens were charged against the
	// company pack pool; plan tokens are the remainder, so the fare budget the
	// gateway gates on excludes pack spend and recovers cleanly each period.
	queryToday := `
		SELECT
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(pack_tokens), 0),
			COALESCE(SUM(input_tokens + output_tokens - pack_tokens), 0)
		FROM ai_token_usage
		WHERE ` + where + `
		  AND created_at >= DATE_TRUNC('day', NOW())
	`
	if err := b.db.QueryRow(ctx, queryToday, args...).Scan(
		&resp.TodayInputTokens,
		&resp.TodayOutputTokens,
		&resp.TodayPackTokens,
		&resp.TodayPlanTokens,
	); err != nil {
		return nil, err
	}

	queryMonthly := `
		SELECT
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(pack_tokens), 0),
			COALESCE(SUM(input_tokens + output_tokens - pack_tokens), 0)
		FROM ai_token_usage
		WHERE ` + where + `
		  AND created_at >= DATE_TRUNC('month', NOW())
	`
	if err := b.db.QueryRow(ctx, queryMonthly, args...).Scan(
		&resp.MonthlyInputTokens,
		&resp.MonthlyOutputTokens,
		&resp.MonthlyPackTokens,
		&resp.MonthlyPlanTokens,
	); err != nil {
		return nil, err
	}

	return resp, nil
}
