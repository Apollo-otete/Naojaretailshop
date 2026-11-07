-- 0005: Catalogue context — categories, products, product_images (§5.2).
--
-- Source of truth: docs/phase-1-domain-model-erd.md §5.2. Money is integer cents (ADR-021);
-- publication state is independent of stock status (§5 workflow step 4).

CREATE TABLE catalogue.categories (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    slug        CITEXT UNIQUE NOT NULL CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
    icon        TEXT,
    parent_id   BIGINT REFERENCES catalogue.categories (id) ON DELETE SET NULL,
    is_active   BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER categories_touch_updated_at
    BEFORE UPDATE ON catalogue.categories
    FOR EACH ROW EXECUTE FUNCTION identity.touch_updated_at();

CREATE TABLE catalogue.products (
    id                       BIGSERIAL PRIMARY KEY,
    sku                      CITEXT UNIQUE NOT NULL CHECK (char_length(sku) BETWEEN 1 AND 64),
    name                     TEXT NOT NULL CHECK (char_length(name) BETWEEN 2 AND 200),
    slug                     CITEXT UNIQUE NOT NULL CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
    description              TEXT CHECK (description IS NULL OR char_length(description) <= 20000),
    brand                    TEXT,
    price_cents              BIGINT NOT NULL CHECK (price_cents >= 0 AND price_cents % 100 = 0),
    compare_at_price_cents   BIGINT NULL CHECK (compare_at_price_cents IS NULL OR (compare_at_price_cents >= 0 AND compare_at_price_cents % 100 = 0)),
    category_id              BIGINT NOT NULL REFERENCES catalogue.categories (id),
    subcategory              TEXT,
    publication_state        TEXT NOT NULL DEFAULT 'draft'
                             CHECK (publication_state IN ('draft','published','unpublished','archived')),
    is_featured              BOOLEAN NOT NULL DEFAULT false,
    specifications           JSONB NOT NULL DEFAULT '{}'::jsonb,
    views                    INTEGER NOT NULL DEFAULT 0 CHECK (views >= 0),
    rating_avg               NUMERIC(3,2) NOT NULL DEFAULT 0 CHECK (rating_avg BETWEEN 0 AND 5),
    rating_count             INTEGER NOT NULL DEFAULT 0 CHECK (rating_count >= 0),
    low_stock_threshold      INTEGER,
    critical_stock_threshold  INTEGER,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at             TIMESTAMPTZ,
    archived_at              TIMESTAMPTZ
);

CREATE INDEX ON catalogue.products (category_id) WHERE publication_state = 'published';
CREATE INDEX ON catalogue.products (publication_state);
CREATE INDEX ON catalogue.products USING gin (to_tsvector('simple',
    name || ' ' || coalesce(description, '') || ' ' || coalesce(subcategory, '')));
CREATE UNIQUE INDEX ON catalogue.products (lower(sku::text));

CREATE TRIGGER products_touch_updated_at
    BEFORE UPDATE ON catalogue.products
    FOR EACH ROW EXECUTE FUNCTION identity.touch_updated_at();

CREATE TABLE catalogue.product_images (
    id            BIGSERIAL PRIMARY KEY,
    product_id    BIGINT NOT NULL REFERENCES catalogue.products (id) ON DELETE CASCADE,
    url           TEXT NOT NULL,
    alt_text      TEXT,
    position      INTEGER NOT NULL DEFAULT 0 CHECK (position >= 0),
    provider      TEXT NOT NULL DEFAULT 'local' CHECK (provider IN ('local','cloudinary')),
    provider_ref   TEXT,
    is_primary    BOOLEAN NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (product_id, url)
);

-- One primary image per product, enforced in the database.
CREATE UNIQUE INDEX ON catalogue.product_images (product_id) WHERE is_primary;

CREATE INDEX ON catalogue.product_images (product_id, position);
