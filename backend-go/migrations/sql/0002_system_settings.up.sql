-- 0002: administration.system_settings.
--
-- Today these values live in two places that can disagree: one JSONB blob written by
-- initPg.js:131-142 and hardcoded literals scattered through route handlers (the "5 items is
-- low stock" rule appears twice in index.js). One row per setting makes each individually
-- editable, auditable and diffable.

CREATE TABLE administration.system_settings (
    key          CITEXT PRIMARY KEY,
    value        JSONB   NOT NULL,
    -- The unit is recorded because a column called fee_cents seeded with 150 is a 100x bug
    -- that no test catches until a customer is charged the wrong amount.
    unit         TEXT    NOT NULL DEFAULT 'raw'
                 CHECK (unit IN ('raw','cents','minutes','hours','days','percent','boolean','text')),
    description  TEXT    NOT NULL,
    -- ADR-018 is not ratified: the shop has not confirmed its delivery fee, free-delivery
    -- threshold or zone coverage. Marking those rows here means a placeholder can never be
    -- read as an approved business term by whoever builds Phase 5.
    ratified     BOOLEAN NOT NULL DEFAULT true,
    ratified_at  TIMESTAMPTZ,
    -- No FK yet: identity.users does not exist until Phase 3. The column is nullable now and
    -- gains its constraint in the migration that creates the referenced table.
    updated_by   BIGINT  NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE administration.system_settings IS
  'Operational configuration, one row per key. Money is integer cents (§9). Rows with ratified=false are placeholders pending product-owner input.';
COMMENT ON COLUMN administration.system_settings.unit IS
  'How to interpret value. cents means whole-shilling amounts are multiples of 100 (ADR-021).';

INSERT INTO administration.system_settings (key, value, unit, description, ratified, ratified_at) VALUES
  ('currency',                       '"KES"',   'text',    'Single supported currency; every money column is KES minor units.', true, now()),
  ('presentation_timezone',          '"Africa/Nairobi"', 'text', 'Presentation zone only. Storage and comparison stay UTC (§9, ADR-018).', true, now()),

  ('low_stock_limit',                '5',       'raw',     'Default threshold for the low-stock label; overridable per product on inventory.low_stock_threshold.', true, now()),
  ('critical_stock_limit',           '2',       'raw',     'Default critical-stock threshold.', true, now()),
  ('dead_stock_days',                '30',      'days',    'No sale for this long counts as dead stock in analytics.', true, now()),

  ('reservation_ttl_minutes',        '30',      'minutes', 'How long placed-but-unpaid stock stays reserved. Drives orders.expires_at and the T-05 sweep.', true, now()),
  ('stuck_order_minutes',            '30',      'minutes', 'Alert threshold for a confirmed order with no movement.', true, now()),
  ('fulfillment_delay_hours',        '48',      'hours',   'Alert threshold for an order awaiting fulfilment too long.', true, now()),

  ('revenue_drop_pct',               '20',      'percent', 'Alert when revenue falls this far against the prior period.', true, now()),
  ('failed_payment_spike_count',     '3',       'raw',     'Alert when this many payments fail in the window.', true, now()),

  ('revenue_target_monthly_cents',   '15000000','cents',   'KSh 150,000/mo. Carried from the current thresholds blob.', true, now()),
  ('revenue_target_quarterly_cents', '60000000','cents',   'KSh 600,000/quarter.', true, now()),
  ('revenue_target_annual_cents',    '720000000','cents',  'KSh 7,200,000/year.', true, now()),

  ('delivery_standard_fee_cents',    '15000',   'cents',   'PLACEHOLDER — ADR-018 not ratified. KSh 150 is the value the current frontend displays, not a decision.', false, NULL),
  ('free_delivery_threshold_cents',  '60000',   'cents',   'PLACEHOLDER — ADR-018 not ratified. KSh 600 carried over from the prototype UI.', false, NULL);

-- No payment-window or Daraja credential settings here: provider configuration is secrets,
-- which belong in the environment (§10), never in a readable table.
