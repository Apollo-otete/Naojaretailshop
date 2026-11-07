package payments

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/inventory"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/payments/daraja"
)

type Daraja interface {
	Enabled() bool
	STKPush(ctx context.Context, in daraja.STKRequest) (daraja.STKResponse, error)
}

type OrderUpdater interface {
	SetOrderPaymentState(ctx context.Context, orderID int64, orderStatus string, paymentStatus string) error
}

type Deps struct {
	Store      Store
	Orders     OrderReader
	OrderState OrderUpdater // may be same concrete type as Store
	Inventory  inventory.Store
	Tx         Transactor
	Daraja     Daraja
	Clock      func() time.Time
	Logger     *slog.Logger
}

type Service struct {
	d Deps
}

func NewService(d Deps) *Service {
	if d.Clock == nil {
		d.Clock = time.Now
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Service{d: d}
}

type InitiateResult struct {
	Payment           Payment `json:"payment"`
	Attempt           Attempt `json:"attempt"`
	CustomerMessage   string  `json:"customer_message,omitempty"`
	CheckoutRequestID string  `json:"checkout_request_id,omitempty"`
}

// InitiateSTK starts an M-Pesa STK push for an order. Amount is always the server's owed amount.
func (s *Service) InitiateSTK(ctx context.Context, orderRef string, phone string) (InitiateResult, error) {
	if s.d.Daraja == nil || !s.d.Daraja.Enabled() {
		return InitiateResult{}, apierr.Conflict(apierr.CodeProviderUnavailable, "M-Pesa is not configured on this server.")
	}
	order, err := s.d.Orders.OrderByReference(ctx, orderRef)
	if err != nil {
		return InitiateResult{}, apierr.NotFound("order")
	}
	if order.Status == "cancelled" || order.Status == "expired" || order.Status == "failed" {
		return InitiateResult{}, apierr.Conflict(apierr.CodeConflict, "This order cannot be paid.")
	}
	if order.ExpiresAt != nil && !order.ExpiresAt.After(s.d.Clock()) {
		return InitiateResult{}, apierr.Conflict(apierr.CodeConflict, "This order has expired.")
	}
	if order.TotalCents < 100 {
		return InitiateResult{}, apierr.Validation(apierr.Detail{Field: "amount", Issue: "order total is too small for M-Pesa"})
	}
	phone = normalizePhone(phone)
	if phone == "" {
		return InitiateResult{}, apierr.Validation(apierr.Detail{Field: "phone", Issue: "a valid Kenyan phone number is required"})
	}

	pay, err := s.d.Store.GetOrCreatePayment(ctx, order)
	if err != nil {
		return InitiateResult{}, err
	}
	if pay.Status == StatusPaid {
		return InitiateResult{}, apierr.Conflict(apierr.CodePaymentAlreadySettled, "This order has already been paid.")
	}

	resp, stkErr := s.d.Daraja.STKPush(ctx, daraja.STKRequest{
		AmountCents: order.TotalCents, Phone: phone, Reference: order.Reference,
		Description: "Naoja " + order.Reference,
	})
	att := Attempt{
		PaymentID: pay.ID, OrderID: order.ID, Provider: "daraja",
		Phone: phone, AmountRequestedCents: order.TotalCents, RawResponse: resp.Raw,
	}
	if resp.CheckoutRequestID != "" {
		c := resp.CheckoutRequestID
		att.CheckoutRequestID = &c
	}
	if resp.MerchantRequestID != "" {
		m := resp.MerchantRequestID
		att.MerchantRequestID = &m
	}
	if stkErr != nil {
		msg := stkErr.Error()
		att.Status = AttemptFailed
		att.ResultDesc = &msg
		_ = s.d.Store.CreateAttempt(ctx, &att)
		return InitiateResult{}, apierr.Conflict(apierr.CodeProviderUnavailable, "Could not initiate M-Pesa payment. Please try again.")
	}
	att.Status = AttemptPending
	rc := resp.ResponseCode
	rd := resp.ResponseDescription
	if rd == "" {
		rd = resp.CustomerMessage
	}
	att.ResultCode = &rc
	att.ResultDesc = &rd
	if err := s.d.Store.CreateAttempt(ctx, &att); err != nil {
		return InitiateResult{}, err
	}
	return InitiateResult{Payment: pay, Attempt: att, CustomerMessage: resp.CustomerMessage, CheckoutRequestID: resp.CheckoutRequestID}, nil
}

// ProcessCallback handles a Daraja STK callback idempotently. It never trusts the callback
// amount until correlated and compared to amount_owed_cents; duplicate receipts are no-ops.
func (s *Service) ProcessCallback(ctx context.Context, cb daraja.CallbackResult) error {
	if cb.CheckoutRequestID == "" {
		// Cannot correlate; acknowledge but do not mutate state.
		s.d.Logger.Warn("daraja callback missing CheckoutRequestID")
		return nil
	}
	// Resolve attempt/payment/order.
	att, err := s.d.Store.AttemptByCheckout(ctx, cb.CheckoutRequestID)
	if err != nil {
		// Unknown checkout id - could be stale/foreign; acknowledge safely.
		s.d.Logger.Warn("daraja callback for unknown checkout", slog.String("checkout", cb.CheckoutRequestID))
		return nil
	}
	pay, err := s.d.Store.PaymentByID(ctx, att.PaymentID)
	if err != nil {
		return nil
	}

	resultStr := fmt.Sprintf("%d", cb.ResultCode)
	desc := cb.ResultDesc
	merch := cb.MerchantRequestID
	if cb.Success() {
		ok := AttemptSucceeded
		_ = s.d.Store.UpdateAttempt(ctx, cb.CheckoutRequestID, ok, &resultStr, &desc, &merch, cb.Raw)
	} else {
		fail := AttemptFailed
		_ = s.d.Store.UpdateAttempt(ctx, cb.CheckoutRequestID, fail, &resultStr, &desc, &merch, cb.Raw)
	}

	if !cb.Success() || cb.MpesaReceipt == "" {
		// Failed/cancelled: mark failed and release reservation (idempotent). Never consume.
		run := func(ctx context.Context) error {
			_ = s.d.Store.FailPayment(ctx, pay.ID)
			if s.d.OrderState != nil {
				_ = s.d.OrderState.SetOrderPaymentState(ctx, pay.OrderID, "failed", "failed")
			}
			if s.d.Inventory != nil {
				_ = s.d.Inventory.ReleaseOrder(ctx, pay.OrderID, "payment_failed_or_cancelled")
			}
			return nil
		}
		if s.d.Tx != nil {
			return s.d.Tx.RunInTx(ctx, run)
		}
		return run(ctx)
	}

	// Verify amount when provided.
	amountPaid := pay.AmountOwedCents
	reconciliation := false
	if cb.AmountCents != nil {
		amountPaid = *cb.AmountCents
		if amountPaid != pay.AmountOwedCents {
			reconciliation = true
			s.d.Logger.Warn("daraja amount mismatch", slog.String("order", pay.OrderReference), slog.Int64("owed", pay.AmountOwedCents), slog.Int64("paid", amountPaid))
		}
	}

	now := s.d.Clock()
	tx := &Transaction{
		PaymentID: pay.ID, AttemptID: &att.ID, ProviderTransactionID: cb.MpesaReceipt,
		CheckoutRequestID: &cb.CheckoutRequestID, AmountPaidCents: amountPaid,
		ResultCode: resultStr, ResultDesc: &desc, ReceivedAt: now, SourceVerified: true, VerifiedAt: &now, RawPayload: cb.Raw,
	}
	if cb.Phone != "" {
		ph := cb.Phone
		tx.Phone = &ph
	}

	// Atomic/idempotent settlement: unique receipt => replay no-op.
	run := func(ctx context.Context) error {
		inserted, err := s.d.Store.RecordTransaction(ctx, tx)
		if err != nil {
			return err
		}
		if !inserted {
			return nil // replay, no-op
		}
		if err := s.d.Store.SettlePayment(ctx, pay.ID, amountPaid, now, reconciliation); err != nil {
			return err
		}
		if !reconciliation {
			if s.d.OrderState != nil {
				if err := s.d.OrderState.SetOrderPaymentState(ctx, pay.OrderID, "confirmed", "paid"); err != nil {
					return err
				}
			}
			if s.d.Inventory != nil {
				if err := s.d.Inventory.ConsumeOrder(ctx, pay.OrderID); err != nil {
					s.d.Logger.Error("consume reservation after payment failed", slog.Int64("order_id", pay.OrderID), slog.Any("err", err))
				}
			}
		} else {
			// Amount mismatch: record for reconciliation; release reservation to avoid oversell on
			// unverified settlement, and mark payment/order for review.
			if s.d.Inventory != nil {
				_ = s.d.Inventory.ReleaseOrder(ctx, pay.OrderID, "payment_amount_mismatch_reconciliation")
			}
			if s.d.OrderState != nil {
				_ = s.d.OrderState.SetOrderPaymentState(ctx, pay.OrderID, "pending_payment", "pending")
			}
		}
		return nil
	}
	if s.d.Tx != nil {
		return s.d.Tx.RunInTx(ctx, run)
	}
	return run(ctx)
}

// ReleaseReservationOnFailure releases any active reservation for an order (idempotent). Exposed
// for sweeper/expiry hooks and operational use.
func (s *Service) ReleaseReservationOnFailure(ctx context.Context, orderID int64, reason string) error {
	if s.d.Inventory == nil {
		return nil
	}
	return s.d.Inventory.ReleaseOrder(ctx, orderID, reason)
}

func (s *Service) PaymentByOrderRef(ctx context.Context, ref string) (Payment, []Attempt, []Transaction, error) {
	p, err := s.d.Store.PaymentByOrderRef(ctx, ref)
	if err != nil {
		return Payment{}, nil, nil, err
	}
	atts, _ := s.d.Store.AttemptsByPayment(ctx, p.ID)
	txs, _ := s.d.Store.TransactionsByPayment(ctx, p.ID)
	return p, atts, txs, nil
}

func normalizePhone(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "+")
	if strings.HasPrefix(s, "0") && len(s) == 10 {
		s = "254" + s[1:]
	}
	if strings.HasPrefix(s, "254") && len(s) == 12 {
		return s
	}
	return ""
}
