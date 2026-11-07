package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/database"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/payments"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) (*Store, error) {
	if pool == nil {
		return nil, database.ErrNoPool
	}
	return &Store{pool: pool}, nil
}

var _ payments.Store = (*Store)(nil)

type Tx struct{ runner *database.TxRunner }

func NewTx(pool *pgxpool.Pool) *Tx { return &Tx{runner: database.NewTxRunner(pool)} }

func (t *Tx) RunInTx(ctx context.Context, work func(ctx context.Context) error) error {
	return t.runner.RunInTx(ctx, work)
}

func (s *Store) exec(ctx context.Context) database.Execer { return database.DB(ctx, s.pool) }

func (s *Store) scanPayment(row pgx.Row) (payments.Payment, error) {
	var p payments.Payment
	var amountPaid *int64
	var settled *time.Time
	err := row.Scan(&p.ID, &p.OrderID, &p.OrderReference, &p.Status, &p.AmountOwedCents, &amountPaid, &p.Currency, &p.ReconciliationRequired, &settled, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return payments.Payment{}, err
	}
	p.AmountPaidCents = amountPaid
	p.SettledAt = settled
	return p, nil
}

const paymentCols = `id, order_id, order_reference, status, amount_owed_cents, amount_paid_cents, currency, reconciliation_required, settled_at, created_at, updated_at`

func (s *Store) GetOrCreatePayment(ctx context.Context, order payments.OrderRef) (payments.Payment, error) {
	p, err := s.PaymentByOrderRef(ctx, order.Reference)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, payments.ErrNoPayment) {
		return payments.Payment{}, err
	}
	var p2 payments.Payment
	row := s.exec(ctx).QueryRow(ctx, `
		INSERT INTO payments.payments (order_id, order_reference, status, amount_owed_cents, currency)
		VALUES ($1,$2,'pending',$3,$4)
		ON CONFLICT (order_id) DO UPDATE SET order_reference=EXCLUDED.order_reference, amount_owed_cents=EXCLUDED.amount_owed_cents, updated_at=now()
		RETURNING `+paymentCols, order.ID, order.Reference, order.TotalCents, order.Currency)
	p2, err = s.scanPayment(row)
	if err != nil {
		return payments.Payment{}, fmt.Errorf("creating payment: %w", err)
	}
	return p2, nil
}

func (s *Store) PaymentByOrderRef(ctx context.Context, ref string) (payments.Payment, error) {
	row := s.exec(ctx).QueryRow(ctx, `SELECT `+paymentCols+` FROM payments.payments WHERE order_reference=$1`, ref)
	p, err := s.scanPayment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return payments.Payment{}, payments.ErrNoPayment
	}
	if err != nil {
		return payments.Payment{}, err
	}
	return p, nil
}

func (s *Store) PaymentByID(ctx context.Context, id int64) (payments.Payment, error) {
	row := s.exec(ctx).QueryRow(ctx, `SELECT `+paymentCols+` FROM payments.payments WHERE id=$1`, id)
	p, err := s.scanPayment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return payments.Payment{}, payments.ErrNoPayment
	}
	if err != nil {
		return payments.Payment{}, err
	}
	return p, nil
}

func (s *Store) CreateAttempt(ctx context.Context, a *payments.Attempt) error {
	row := s.exec(ctx).QueryRow(ctx, `
		INSERT INTO payments.payment_attempts (payment_id, order_id, provider, merchant_request_id, checkout_request_id, phone, amount_requested_cents, status, result_code, result_desc, raw_response)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, created_at, updated_at`,
		a.PaymentID, a.OrderID, a.Provider, a.MerchantRequestID, a.CheckoutRequestID, a.Phone, a.AmountRequestedCents, string(a.Status), a.ResultCode, a.ResultDesc, a.RawResponse)
	return row.Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
}

func (s *Store) UpdateAttempt(ctx context.Context, checkoutRequestID string, status payments.AttemptStatus, resultCode, resultDesc *string, merchantReq *string, raw []byte) error {
	_, err := s.exec(ctx).Exec(ctx, `
		UPDATE payments.payment_attempts
		SET status=$2, result_code=COALESCE($3,result_code), result_desc=COALESCE($4,result_desc),
		    merchant_request_id=COALESCE($5,merchant_request_id), raw_response=COALESCE($6,raw_response), updated_at=now()
		WHERE checkout_request_id=$1`, checkoutRequestID, string(status), resultCode, resultDesc, merchantReq, raw)
	return err
}

func (s *Store) AttemptByCheckout(ctx context.Context, checkoutRequestID string) (payments.Attempt, error) {
	row := s.exec(ctx).QueryRow(ctx, `
		SELECT id, payment_id, order_id, provider, merchant_request_id, checkout_request_id, phone, amount_requested_cents, status, result_code, result_desc, raw_response, created_at, updated_at
		FROM payments.payment_attempts WHERE checkout_request_id=$1`, checkoutRequestID)
	var a payments.Attempt
	var status string
	var merch, checkout, rc, rd *string
	err := row.Scan(&a.ID, &a.PaymentID, &a.OrderID, &a.Provider, &merch, &checkout, &a.Phone, &a.AmountRequestedCents, &status, &rc, &rd, &a.RawResponse, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return payments.Attempt{}, fmt.Errorf("no attempt")
	}
	if err != nil {
		return payments.Attempt{}, err
	}
	a.MerchantRequestID, a.CheckoutRequestID, a.ResultCode, a.ResultDesc = merch, checkout, rc, rd
	a.Status = payments.AttemptStatus(status)
	return a, nil
}

func (s *Store) RecordTransaction(ctx context.Context, tx *payments.Transaction) (bool, error) {
	// Idempotent on provider_transaction_id (MpesaReceiptNumber). Replayed callback => no-op.
	row := s.exec(ctx).QueryRow(ctx, `
		INSERT INTO payments.payment_transactions (payment_id, attempt_id, provider_transaction_id, checkout_request_id, amount_paid_cents, result_code, result_desc, phone, received_at, source_verified, verified_at, raw_payload)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (provider_transaction_id) DO NOTHING
		RETURNING id`,
		tx.PaymentID, tx.AttemptID, tx.ProviderTransactionID, tx.CheckoutRequestID, tx.AmountPaidCents, tx.ResultCode, tx.ResultDesc, tx.Phone, tx.ReceivedAt, tx.SourceVerified, tx.VerifiedAt, tx.RawPayload)
	var id int64
	err := row.Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // replay/no-op
	}
	if err != nil {
		return false, err
	}
	tx.ID = id
	return true, nil
}

func (s *Store) SettlePayment(ctx context.Context, paymentID int64, amountPaid int64, at time.Time, reconciliation bool) error {
	_, err := s.exec(ctx).Exec(ctx, `
		UPDATE payments.payments
		SET status=CASE WHEN $3 THEN 'pending' ELSE 'paid' END,
		    amount_paid_cents=$2, settled_at=CASE WHEN $3 THEN settled_at ELSE $4 END,
		    reconciliation_required=$3, updated_at=now()
		WHERE id=$1`, paymentID, amountPaid, reconciliation, at)
	return err
}

func (s *Store) FailPayment(ctx context.Context, paymentID int64) error {
	_, err := s.exec(ctx).Exec(ctx, `UPDATE payments.payments SET status='failed', updated_at=now() WHERE id=$1 AND status='pending'`, paymentID)
	return err
}

func (s *Store) AttemptsByPayment(ctx context.Context, paymentID int64) ([]payments.Attempt, error) {
	rows, err := s.exec(ctx).Query(ctx, `
		SELECT id, payment_id, order_id, provider, merchant_request_id, checkout_request_id, phone, amount_requested_cents, status, result_code, result_desc, raw_response, created_at, updated_at
		FROM payments.payment_attempts WHERE payment_id=$1 ORDER BY created_at DESC`, paymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []payments.Attempt
	for rows.Next() {
		var a payments.Attempt
		var status string
		var merch, checkout, rc, rd *string
		if err := rows.Scan(&a.ID, &a.PaymentID, &a.OrderID, &a.Provider, &merch, &checkout, &a.Phone, &a.AmountRequestedCents, &status, &rc, &rd, &a.RawResponse, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.MerchantRequestID, a.CheckoutRequestID, a.ResultCode, a.ResultDesc = merch, checkout, rc, rd
		a.Status = payments.AttemptStatus(status)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) TransactionsByPayment(ctx context.Context, paymentID int64) ([]payments.Transaction, error) {
	rows, err := s.exec(ctx).Query(ctx, `
		SELECT id, payment_id, attempt_id, provider_transaction_id, checkout_request_id, amount_paid_cents, result_code, result_desc, phone, received_at, source_verified, verified_at, raw_payload
		FROM payments.payment_transactions WHERE payment_id=$1 ORDER BY received_at DESC`, paymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []payments.Transaction
	for rows.Next() {
		var t payments.Transaction
		var attemptID *int64
		var checkout, rd, phone *string
		var verifiedAt *time.Time
		if err := rows.Scan(&t.ID, &t.PaymentID, &attemptID, &t.ProviderTransactionID, &checkout, &t.AmountPaidCents, &t.ResultCode, &rd, &phone, &t.ReceivedAt, &t.SourceVerified, &verifiedAt, &t.RawPayload); err != nil {
			return nil, err
		}
		t.AttemptID, t.CheckoutRequestID, t.ResultDesc, t.Phone, t.VerifiedAt = attemptID, checkout, rd, phone, verifiedAt
		out = append(out, t)
	}
	return out, rows.Err()
}

// OrderByReference reads the minimal order info needed for payments (separate from payments tables).
func (s *Store) OrderByReference(ctx context.Context, ref string) (payments.OrderRef, error) {
	var o payments.OrderRef
	var userID *int64
	var expires *time.Time
	err := s.exec(ctx).QueryRow(ctx, `
		SELECT id, reference, user_id, total_cents, currency, status, expires_at
		FROM commerce.orders WHERE reference=$1`, ref).Scan(&o.ID, &o.Reference, &userID, &o.TotalCents, &o.Currency, &o.Status, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return payments.OrderRef{}, payments.ErrNoOrder
	}
	if err != nil {
		return payments.OrderRef{}, err
	}
	o.UserID = userID
	o.ExpiresAt = expires
	return o, nil
}

// SetOrderPaid/Failed update commerce order status as part of reconciliation (used by service in tx).
func (s *Store) SetOrderPaymentState(ctx context.Context, orderID int64, orderStatus string, paymentStatus string) error {
	_, err := s.exec(ctx).Exec(ctx, `
		UPDATE commerce.orders
		SET status=$2, payment_status=$3,
		    paid_at = CASE WHEN $3='paid' THEN COALESCE(paid_at, now()) ELSE paid_at END,
		    updated_at=now()
		WHERE id=$1`, orderID, orderStatus, paymentStatus)
	return err
}
