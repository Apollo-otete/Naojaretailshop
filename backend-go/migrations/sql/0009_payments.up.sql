-- 0009: Payments context (P1) — payments, attempts, verified transactions (idempotent).
-- Aligns with contract/ERD: a payment attempt is not a settlement; provider_transaction_id
-- (MpesaReceiptNumber) is unique so replayed callbacks become no-ops.

CREATE TABLE payments.payments (
    id                   BIGSERIAL PRIMARY KEY,
    order_id             BIGINT NOT NULL UNIQUE REFERENCES commerce.orders (id) ON DELETE CASCADE,
    order_reference      TEXT NOT NULL,
    status               TEXT NOT NULL DEFAULT 'pending'
                         CHECK (status IN ('pending','paid','failed','expired','refunded')),
    amount_owed_cents    BIGINT NOT NULL CHECK (amount_owed_cents >= 0),
    amount_paid_cents    BIGINT,
    currency             TEXT NOT NULL DEFAULT 'KES',
    reconciliation_required BOOLEAN NOT NULL DEFAULT FALSE,
    settled_at           TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ON payments.payments (status);
CREATE INDEX ON payments.payments (order_reference);

CREATE TRIGGER payments_touch_updated_at
    BEFORE UPDATE ON payments.payments
    FOR EACH ROW EXECUTE FUNCTION identity.touch_updated_at();

CREATE TABLE payments.payment_attempts (
    id                   BIGSERIAL PRIMARY KEY,
    payment_id           BIGINT NOT NULL REFERENCES payments.payments (id) ON DELETE CASCADE,
    order_id             BIGINT NOT NULL REFERENCES commerce.orders (id) ON DELETE CASCADE,
    provider             TEXT NOT NULL DEFAULT 'daraja',
    merchant_request_id  TEXT,
    checkout_request_id  TEXT,
    phone                TEXT NOT NULL,
    amount_requested_cents BIGINT NOT NULL CHECK (amount_requested_cents >= 0),
    status               TEXT NOT NULL DEFAULT 'initiated'
                         CHECK (status IN ('initiated','pending','succeeded','failed','unknown')),
    result_code          TEXT,
    result_desc          TEXT,
    raw_response         JSONB,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (checkout_request_id)
);

CREATE INDEX ON payments.payment_attempts (payment_id, created_at DESC);
CREATE INDEX ON payments.payment_attempts (order_id);

CREATE TRIGGER payment_attempts_touch_updated_at
    BEFORE UPDATE ON payments.payment_attempts
    FOR EACH ROW EXECUTE FUNCTION identity.touch_updated_at();

CREATE TABLE payments.payment_transactions (
    id                      BIGSERIAL PRIMARY KEY,
    payment_id              BIGINT NOT NULL REFERENCES payments.payments (id) ON DELETE CASCADE,
    attempt_id              BIGINT REFERENCES payments.payment_attempts (id) ON DELETE SET NULL,
    provider_transaction_id TEXT NOT NULL UNIQUE, -- MpesaReceiptNumber; replay => no-op
    checkout_request_id     TEXT,
    amount_paid_cents       BIGINT NOT NULL CHECK (amount_paid_cents >= 0),
    result_code             TEXT NOT NULL,
    result_desc             TEXT,
    phone                   TEXT,
    received_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    source_verified         BOOLEAN NOT NULL DEFAULT FALSE,
    verified_at             TIMESTAMPTZ,
    raw_payload             JSONB
);

CREATE INDEX ON payments.payment_transactions (payment_id, received_at DESC);
CREATE INDEX ON payments.payment_transactions (checkout_request_id);
