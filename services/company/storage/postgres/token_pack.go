package postgres

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/opentracing/opentracing-go"
)

func normalizeProductType(product string) string {
	if product != config.PRODUCT_TYPE_UCODE && product != config.PRODUCT_TYPE_UGEN {
		return config.PRODUCT_TYPE_UGEN
	}
	return product
}

func (b *billingRepo) CreateTokenPack(ctx context.Context, req *pb.CreateTokenPackRequest) (*pb.TokenPack, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "billing.CreateTokenPack")
	defer span.Finish()

	if req.GetTokenAmount() <= 0 {
		return nil, config.ErrInvalidTokenPackAmount
	}

	var currencyID any
	if req.GetCurrencyId() != "" {
		currencyID = req.GetCurrencyId()
	}

	id := uuid.NewString()
	_, err := b.db.Exec(ctx, `
		INSERT INTO token_pack (id, name, token_amount, price, currency_id, product_type, is_active)
		VALUES ($1, $2, $3, $4, $5, $6::product_type, $7)`,
		id, req.GetName(), req.GetTokenAmount(), req.GetPrice(), currencyID,
		normalizeProductType(req.GetProductType()), req.GetIsActive(),
	)
	if err != nil {
		return nil, b.db.HandleDatabaseError(err, "CreateTokenPack: insert")
	}

	return b.getTokenPackByID(ctx, id)
}

func (b *billingRepo) UpdateTokenPack(ctx context.Context, req *pb.TokenPack) (*pb.TokenPack, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "billing.UpdateTokenPack")
	defer span.Finish()

	if req.GetTokenAmount() <= 0 {
		return nil, config.ErrInvalidTokenPackAmount
	}

	var currencyID any
	if req.GetCurrencyId() != "" {
		currencyID = req.GetCurrencyId()
	}

	tag, err := b.db.Exec(ctx, `
		UPDATE token_pack
		SET name         = $2,
		    token_amount = $3,
		    price        = $4,
		    currency_id  = $5,
		    product_type = $6::product_type,
		    is_active    = $7,
		    updated_at   = NOW()
		WHERE id = $1 AND deleted_at = 0`,
		req.GetId(), req.GetName(), req.GetTokenAmount(), req.GetPrice(), currencyID,
		normalizeProductType(req.GetProductType()), req.GetIsActive(),
	)
	if err != nil {
		return nil, b.db.HandleDatabaseError(err, "UpdateTokenPack: update")
	}
	if tag.RowsAffected() == 0 {
		return nil, config.ErrTokenPackNotFound
	}

	return b.getTokenPackByID(ctx, req.GetId())
}

func (b *billingRepo) DeleteTokenPack(ctx context.Context, req *pb.PrimaryKey) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "billing.DeleteTokenPack")
	defer span.Finish()

	tag, err := b.db.Exec(ctx, `
		UPDATE token_pack
		SET deleted_at = EXTRACT(EPOCH FROM NOW())::INTEGER, updated_at = NOW()
		WHERE id = $1 AND deleted_at = 0`,
		req.GetId(),
	)
	if err != nil {
		return b.db.HandleDatabaseError(err, "DeleteTokenPack: soft delete")
	}
	if tag.RowsAffected() == 0 {
		return config.ErrTokenPackNotFound
	}
	return nil
}

func (b *billingRepo) ListTokenPacks(ctx context.Context, req *pb.ListTokenPacksRequest) (*pb.ListTokenPacksResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "billing.ListTokenPacks")
	defer span.Finish()

	var (
		filters = []string{"tp.deleted_at = 0"}
		args    []any
	)
	if req.GetProductType() != "" {
		args = append(args, req.GetProductType())
		filters = append(filters, "tp.product_type = $"+strconv.Itoa(len(args)))
	}
	if req.GetOnlyActive() {
		filters = append(filters, "tp.is_active = TRUE")
	}

	query := `
		SELECT tp.id, tp.name, tp.token_amount, tp.price, COALESCE(tp.currency_id::text, ''),
		       tp.product_type, tp.is_active, tp.created_at, tp.updated_at,
		       c.id, c.symbol, c.name, c.code
		FROM token_pack tp
		LEFT JOIN currency c ON c.id = tp.currency_id
		WHERE ` + strings.Join(filters, " AND ") + `
		ORDER BY tp.token_amount ASC, tp.created_at DESC`

	rows, err := b.db.Query(ctx, query, args...)
	if err != nil {
		return nil, b.db.HandleDatabaseError(err, "ListTokenPacks: query")
	}
	defer rows.Close()

	resp := &pb.ListTokenPacksResponse{}
	for rows.Next() {
		pack, err := scanTokenPack(rows)
		if err != nil {
			return nil, b.db.HandleDatabaseError(err, "ListTokenPacks: scan")
		}
		resp.TokenPacks = append(resp.TokenPacks, pack)
	}
	if err := rows.Err(); err != nil {
		return nil, b.db.HandleDatabaseError(err, "ListTokenPacks: rows")
	}
	resp.Count = int32(len(resp.TokenPacks))
	return resp, nil
}

func (b *billingRepo) getTokenPackByID(ctx context.Context, id string) (*pb.TokenPack, error) {
	row := b.db.QueryRow(ctx, `
		SELECT tp.id, tp.name, tp.token_amount, tp.price, COALESCE(tp.currency_id::text, ''),
		       tp.product_type, tp.is_active, tp.created_at, tp.updated_at,
		       c.id, c.symbol, c.name, c.code
		FROM token_pack tp
		LEFT JOIN currency c ON c.id = tp.currency_id
		WHERE tp.id = $1 AND tp.deleted_at = 0`, id)

	pack, err := scanTokenPack(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, config.ErrTokenPackNotFound
		}
		return nil, b.db.HandleDatabaseError(err, "getTokenPackByID: scan")
	}
	return pack, nil
}

func scanTokenPack(row pgx.Row) (*pb.TokenPack, error) {
	var (
		pack      = &pb.TokenPack{}
		createdAt time.Time
		updatedAt time.Time

		curID, curSymbol, curName, curCode sql.NullString
	)

	if err := row.Scan(
		&pack.Id, &pack.Name, &pack.TokenAmount, &pack.Price, &pack.CurrencyId,
		&pack.ProductType, &pack.IsActive, &createdAt, &updatedAt,
		&curID, &curSymbol, &curName, &curCode,
	); err != nil {
		return nil, err
	}

	pack.CreatedAt = createdAt.Format(time.RFC3339)
	pack.UpdatedAt = updatedAt.Format(time.RFC3339)

	if curID.Valid {
		pack.Currency = &pb.Currency{
			Id:     curID.String,
			Symbol: curSymbol.String,
			Name:   curName.String,
			Code:   curCode.String,
		}
	}
	return pack, nil
}

// PurchaseTokenPack charges the paying project's balance for the pack price and
// credits the company's pack-token pool atomically. The project row is locked
// FOR UPDATE so a concurrent topup/renewal cannot race the overdraft check;
// the charge is funded by balance + credit_limit, mirroring subscription charges.
func (b *billingRepo) PurchaseTokenPack(ctx context.Context, req *pb.PurchaseTokenPackRequest) (*pb.PurchaseTokenPackResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "billing.PurchaseTokenPack")
	defer span.Finish()

	// Read the pack and its currency code before opening the transaction.
	var (
		packName     string
		tokenAmount  int64
		price        float64
		packCurrency sql.NullString // currency_id
		currencyCode sql.NullString
	)
	if err := b.db.QueryRow(ctx, `
		SELECT tp.name, tp.token_amount, tp.price, tp.currency_id, c.code
		FROM token_pack tp
		LEFT JOIN currency c ON c.id = tp.currency_id
		WHERE tp.id = $1 AND tp.deleted_at = 0 AND tp.is_active = TRUE`,
		req.GetPackId(),
	).Scan(&packName, &tokenAmount, &price, &packCurrency, &currencyCode); err != nil {
		if err == pgx.ErrNoRows {
			return nil, config.ErrTokenPackNotFound
		}
		return nil, b.db.HandleDatabaseError(err, "PurchaseTokenPack: load pack")
	}
	if tokenAmount <= 0 {
		return nil, config.ErrInvalidTokenPackAmount
	}

	// Convert the price into UZS (project.balance is stored in UZS). UZS/empty -> rate 1.
	rate, err := fetchUgenCurrencyRate(ctx, b.db, currencyCode.String, time.Now().Format(time.DateOnly))
	if err != nil {
		return nil, b.db.HandleDatabaseError(err, "PurchaseTokenPack: fetch currency rate")
	}
	amountUZS := price * rate

	tx, err := b.db.Begin(ctx)
	if err != nil {
		return nil, b.db.HandleDatabaseError(err, "PurchaseTokenPack: begin")
	}
	defer tx.Rollback(ctx)

	var balance, creditLimit float64
	if err := tx.QueryRow(ctx, `
		SELECT balance, credit_limit FROM project WHERE id = $1 FOR UPDATE`,
		req.GetProjectId(),
	).Scan(&balance, &creditLimit); err != nil {
		return nil, b.db.HandleDatabaseError(err, "PurchaseTokenPack: lock project")
	}
	if balance+creditLimit < amountUZS {
		return nil, config.ErrBalanceInsuffient
	}

	if _, err := tx.Exec(ctx, `
		UPDATE project SET balance = balance - $1 WHERE id = $2`,
		amountUZS, req.GetProjectId(),
	); err != nil {
		return nil, b.db.HandleDatabaseError(err, "PurchaseTokenPack: debit balance")
	}

	var creatorID any
	if req.GetUserId() != "" {
		creatorID = req.GetUserId()
	}

	transactionID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO transaction (id, project_id, creator_id, comment, amount, transaction_type,
		                         payment_status, creator_type, currency_id, payment_type, rate)
		VALUES ($1, $2, $3, $4, $5, $6, 'accepted', 'user', $7, 'Payme', $8)`,
		transactionID, req.GetProjectId(), creatorID, "token pack: "+packName,
		amountUZS, config.TransactionTypeTokenPack, config.UZS_CURRENCY_ID, rate,
	); err != nil {
		return nil, b.db.HandleDatabaseError(err, "PurchaseTokenPack: insert transaction")
	}

	var remaining int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO company_token_balance (company_id, remaining_tokens, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (company_id) DO UPDATE
		SET remaining_tokens = company_token_balance.remaining_tokens + EXCLUDED.remaining_tokens,
		    updated_at = NOW()
		RETURNING remaining_tokens`,
		req.GetCompanyId(), tokenAmount,
	).Scan(&remaining); err != nil {
		return nil, b.db.HandleDatabaseError(err, "PurchaseTokenPack: credit balance")
	}

	var packCurrencyArg any
	if packCurrency.Valid {
		packCurrencyArg = packCurrency.String
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO token_pack_purchase (company_id, project_id, pack_id, token_amount, price, currency_id, transaction_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		req.GetCompanyId(), req.GetProjectId(), req.GetPackId(), tokenAmount, price, packCurrencyArg, transactionID,
	); err != nil {
		return nil, b.db.HandleDatabaseError(err, "PurchaseTokenPack: insert ledger")
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, b.db.HandleDatabaseError(err, "PurchaseTokenPack: commit")
	}

	return &pb.PurchaseTokenPackResponse{
		CompanyId:       req.GetCompanyId(),
		TokensAdded:     tokenAmount,
		RemainingTokens: remaining,
		ChargedAmount:   amountUZS,
		TransactionId:   transactionID,
	}, nil
}

func (b *billingRepo) GetTokenPackBalance(ctx context.Context, req *pb.GetTokenPackBalanceRequest) (*pb.GetTokenPackBalanceResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "billing.GetTokenPackBalance")
	defer span.Finish()

	var remaining int64
	err := b.db.QueryRow(ctx, `
		SELECT remaining_tokens FROM company_token_balance WHERE company_id = $1`,
		req.GetCompanyId(),
	).Scan(&remaining)
	if err != nil && err != pgx.ErrNoRows {
		return nil, b.db.HandleDatabaseError(err, "GetTokenPackBalance: query")
	}

	return &pb.GetTokenPackBalanceResponse{
		CompanyId:       req.GetCompanyId(),
		RemainingTokens: remaining,
	}, nil
}
