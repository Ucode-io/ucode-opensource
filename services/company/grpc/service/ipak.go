package service

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/ipak"
	span "github.com/Ucode-io/ucode-opensource/services/company/pkg/jaeger"
	l "github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ipakClient builds a per-call Ipak Yo'li client from config. The AccessToken and
// base URL come from the merchant cabinet / Vault (see config.IpakEcomm*).
func (s *BillingService) ipakClient() *ipak.Client {
	return ipak.New(
		s.config.IpakEcommBaseUrl,
		s.config.IpakEcommAccessToken,
		s.config.IpakHttpProxy,
		s.config.IpakCACert,
		s.config.IpakInsecureSkipVerify,
		time.Duration(s.config.IpakEcommTimeoutMs)*time.Millisecond,
	)
}

// CreateIpakPayment opens a Visa/Mastercard hosted-page top-up. It calls
// transfer.create_token, records a pending topup transaction keyed by the returned
// transfer_id, and hands the frontend the payment_url to redirect the customer to.
// The customer is charged in UZS (som); the balance is credited on confirmation.
func (s *BillingService) CreateIpakPayment(ctx context.Context, req *pb.CreateIpakPaymentRequest) (*pb.CreateIpakPaymentResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.CreateIpakPayment", req)
	defer dbSpan.Finish()

	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be positive")
	}

	client := s.ipakClient()
	if !client.Configured() {
		return nil, status.Error(codes.FailedPrecondition, "ipak yo'li payment is not configured")
	}

	orderId := helper.GenerateRandomNumber(9)
	orderIdStr := strconv.Itoa(orderId)

	base := strings.TrimRight(s.config.PublicAppUrl, "/")
	// No query string on the return URLs: the bank's WAF rejects payloads that
	// contain "?param=" ("Unauthorized Request Blocked"). The frontend tracks the
	// transfer via sessionStorage, so the query param was redundant anyway.
	successURL := fmt.Sprintf("%s/billing/topup/return", base)
	failURL := fmt.Sprintf("%s/billing/topup/fail", base)

	res, err := client.CreateToken(ctx, ipak.CreateTokenParams{
		OrderID:          orderIdStr,
		Amount:           req.Amount,
		// No parentheses in the description: the bank's WAF rejects "( )" as an
		// injection attempt ("Unauthorized Request Blocked").
		Description:      fmt.Sprintf("uCode balance top-up order %s", orderIdStr),
		SuccessURL:       successURL,
		FailURL:          failURL,
		ExpiresInMinutes: config.IpakYuliLinkExpiryMin,
		Lang:             "ru",
	})
	if err != nil {
		s.logger.Error("--CreateIpakPayment--create token", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	txn, err := s.storage.Billing().CreateTransaction(ctx, &pb.CreateTransactionRequest{
		ProjectId:       req.ProjectId,
		CreatorId:       req.UserId,
		PaymentStatus:   config.PaymePaymentStatusPending,
		Amount:          req.Amount,
		TransactionType: config.TransactionTypeTopup,
		CreatorType:     config.PaymeCreatorType,
		CurrencyId:      config.UZS_CURRENCY_ID,
		OrderId:         int64(orderId),
		PaymentType:     config.IpakYuliPaymentType,
		ExternalId:      res.TransferID,
	})
	if err != nil {
		s.logger.Error("--CreateIpakPayment--create transaction", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.CreateIpakPaymentResponse{
		PaymentUrl:    res.PaymentURL,
		TransferId:    res.TransferID,
		TransactionId: txn.Id,
	}, nil
}

// ConfirmIpakPayment verifies a payment against the bank (transfer.get) and settles
// the local transaction. It is the single, idempotent settlement path shared by the
// callback webhook, the frontend-return poll and the reconcile cron -- required
// because the bank callback fires exactly once with no retries. The credit happens
// at most once (guarded in storage).
func (s *BillingService) ConfirmIpakPayment(ctx context.Context, req *pb.ConfirmIpakPaymentRequest) (*pb.IpakPaymentStatusResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.ConfirmIpakPayment", req)
	defer dbSpan.Finish()

	if req.TransferId == "" {
		return nil, status.Error(codes.InvalidArgument, "transfer_id is required")
	}

	txn, err := s.storage.Billing().GetTransactionByExternalId(ctx, req.TransferId)
	if err != nil {
		s.logger.Error("--ConfirmIpakPayment--get transaction", l.Error(err))
		return nil, status.Error(codes.NotFound, "transaction not found for transfer")
	}

	// Already resolved -- idempotent no-op.
	if txn.PaymentStatus == config.PaymePaymentStatusAccepted || txn.PaymentStatus == config.PaymePaymentStatusCancelled {
		return &pb.IpakPaymentStatusResponse{Status: txn.PaymentStatus, Amount: txn.Amount, TransferId: req.TransferId}, nil
	}

	transfer, err := s.ipakClient().TransferGet(ctx, req.TransferId)
	if err != nil {
		s.logger.Error("--ConfirmIpakPayment--transfer get", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	localStatus, terminal := config.IpakTerminalStatuses[transfer.Status]
	if !terminal {
		// await_payment / in_progress / held -- leave pending for the next attempt.
		return &pb.IpakPaymentStatusResponse{Status: config.PaymePaymentStatusPending, Amount: txn.Amount, TransferId: req.TransferId}, nil
	}

	if localStatus == config.PaymePaymentStatusAccepted {
		// Reject a settled amount that does not match what we opened (never trust the
		// callback/redirect; transfer.get is the source of truth).
		if math.Round(transfer.NetAmount) != math.Round(txn.Amount) {
			s.logger.Error("--ConfirmIpakPayment--amount mismatch",
				l.Any("expected", txn.Amount), l.Any("got", transfer.NetAmount))
			return nil, status.Error(codes.FailedPrecondition, "settled amount does not match order")
		}

		_, credited, err := s.storage.Billing().MarkIpakTransactionAccepted(ctx, req.TransferId)
		if err != nil {
			s.logger.Error("--ConfirmIpakPayment--mark accepted", l.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
		if credited {
			go func() {
				bg := context.Background()
				if err := s.storage.Billing().ProcessUcodeRenewals(bg); err != nil {
					s.logger.Error("--ConfirmIpakPayment--ucode renewals", l.Error(err))
				}
				if err := s.storage.Billing().ProcessUgenRenewals(bg); err != nil {
					s.logger.Error("--ConfirmIpakPayment--ugen renewals", l.Error(err))
				}
			}()
		}
		return &pb.IpakPaymentStatusResponse{Status: config.PaymePaymentStatusAccepted, Amount: txn.Amount, TransferId: req.TransferId}, nil
	}

	// failed / canceled / expired -- close the pending transaction.
	if err := s.storage.Billing().MarkIpakTransactionCancelled(ctx, req.TransferId, fmt.Sprintf("ipak status: %s", transfer.Status)); err != nil {
		s.logger.Error("--ConfirmIpakPayment--mark cancelled", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.IpakPaymentStatusResponse{Status: config.PaymePaymentStatusCancelled, Amount: txn.Amount, TransferId: req.TransferId}, nil
}

// GetIpakPaymentStatus returns the current local status for the frontend return
// page to poll. If the transaction is still pending it actively confirms against
// the bank so the page gets a fresh answer without waiting for the callback/cron.
func (s *BillingService) GetIpakPaymentStatus(ctx context.Context, req *pb.ConfirmIpakPaymentRequest) (*pb.IpakPaymentStatusResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.GetIpakPaymentStatus", req)
	defer dbSpan.Finish()

	if req.TransferId == "" {
		return nil, status.Error(codes.InvalidArgument, "transfer_id is required")
	}

	txn, err := s.storage.Billing().GetTransactionByExternalId(ctx, req.TransferId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "transaction not found for transfer")
	}

	if txn.PaymentStatus == config.PaymePaymentStatusPending {
		return s.ConfirmIpakPayment(ctx, req)
	}
	return &pb.IpakPaymentStatusResponse{Status: txn.PaymentStatus, Amount: txn.Amount, TransferId: req.TransferId}, nil
}
