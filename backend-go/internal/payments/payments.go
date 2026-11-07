// Package payments owns payment state (P1): attempts are not settlements; a verified provider
// transaction (unique provider_transaction_id) is the source of truth, and callbacks/replays are
// idempotent no-ops.
package payments

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNoPayment   = errors.New("no such payment")
	ErrNoOrder     = errors.New("no such order")
	ErrNotPayable  = errors.New("order is not payable")
	ErrSettled     = errors.New("payment already settled")
	ErrUnavailable = errors.New("payment provider not configured")
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusPaid     Status = "paid"
	StatusFailed   Status = "failed"
	StatusExpired  Status = "expired"
	StatusRefunded Status = "refunded"
)

type AttemptStatus string

const (
	AttemptInitiated AttemptStatus = "initiated"
	AttemptPending   AttemptStatus = "pending"
	AttemptSucceeded AttemptStatus = "succeeded"
	AttemptFailed    AttemptStatus = "failed"
	AttemptUnknown   AttemptStatus = "unknown"
)

type Payment struct {
	ID                     int64
	OrderID                int64
	OrderReference         string
	Status                 Status
	AmountOwedCents        int64
	AmountPaidCents        *int64
	Currency               string
	ReconciliationRequired bool
	SettledAt              *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type Attempt struct {
	ID                   int64
	PaymentID            int64
	OrderID              int64
	Provider             string
	MerchantRequestID    *string
	CheckoutRequestID    *string
	Phone                string
	AmountRequestedCents int64
	Status               AttemptStatus
	ResultCode           *string
	ResultDesc           *string
	RawResponse          []byte
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type Transaction struct {
	ID                    int64
	PaymentID             int64
	AttemptID             *int64
	ProviderTransactionID string
	CheckoutRequestID     *string
	AmountPaidCents       int64
	ResultCode            string
	ResultDesc            *string
	Phone                 *string
	ReceivedAt            time.Time
	SourceVerified        bool
	VerifiedAt            *time.Time
	RawPayload            []byte
}

type OrderRef struct {
	ID         int64
	Reference  string
	UserID     *int64
	TotalCents int64
	Currency   string
	Status     string
	ExpiresAt  *time.Time
}

type Store interface {
	GetOrCreatePayment(ctx context.Context, order OrderRef) (Payment, error)
	PaymentByOrderRef(ctx context.Context, ref string) (Payment, error)
	PaymentByID(ctx context.Context, id int64) (Payment, error)
	CreateAttempt(ctx context.Context, a *Attempt) error
	UpdateAttempt(ctx context.Context, checkoutRequestID string, status AttemptStatus, resultCode, resultDesc *string, merchantReq *string, raw []byte) error
	AttemptByCheckout(ctx context.Context, checkoutRequestID string) (Attempt, error)
	RecordTransaction(ctx context.Context, tx *Transaction) (inserted bool, err error) // idempotent on provider_transaction_id
	SettlePayment(ctx context.Context, paymentID int64, amountPaid int64, at time.Time, reconciliation bool) error
	FailPayment(ctx context.Context, paymentID int64) error
	AttemptsByPayment(ctx context.Context, paymentID int64) ([]Attempt, error)
	TransactionsByPayment(ctx context.Context, paymentID int64) ([]Transaction, error)
}

type OrderReader interface {
	OrderByReference(ctx context.Context, ref string) (OrderRef, error)
}

type Transactor interface {
	RunInTx(ctx context.Context, work func(ctx context.Context) error) error
}
