package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/catalogue"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/database"
)

type Store struct {
	pool *pgxpool.Pool
}

// need import
var _ = pgx.ErrNoRows

func New(pool *pgxpool.Pool) (*Store, error) {
	if pool == nil {
		return nil, database.ErrNoPool
	}
	return &Store{pool: pool}, nil
}

// compile-time checks
var _ catalogue.CategoryStore = (*Store)(nil)
var _ catalogue.ProductStore = (*Store)(nil)
var _ catalogue.Transactor = (*Tx)(nil)

type Tx struct{ runner *database.TxRunner }

func NewTx(pool *pgxpool.Pool) *Tx { return &Tx{runner: database.NewTxRunner(pool)} }

func (t *Tx) RunInTx(ctx context.Context, work func(ctx context.Context) error) error {
	return t.runner.RunInTx(ctx, work)
}

func (s *Store) exec(ctx context.Context) database.Execer { return database.DB(ctx, s.pool) }

// ── categories ────────────────────────────────────────────────────────────────

const categoryColumns = `id, name, slug, icon, parent_id, is_active, created_at, updated_at`

func (s *Store) scanCategory(row pgx.Row) (catalogue.Category, error) {
	var c catalogue.Category
	var icon *string
	var parentID *int64
	err := row.Scan(&c.ID, &c.Name, &c.Slug, &icon, &parentID, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return catalogue.Category{}, err
	}
	c.Icon = icon
	c.ParentID = parentID
	return c, nil
}

func (s *Store) CreateCategory(ctx context.Context, c *catalogue.Category) error {
	row := s.exec(ctx).QueryRow(ctx, `
		INSERT INTO catalogue.categories (name, slug, icon, parent_id, is_active)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`,
		c.Name, c.Slug, c.Icon, c.ParentID, c.IsActive)
	var created, updated time.Time
	if err := row.Scan(&c.ID, &created, &updated); err != nil {
		return wrapUnique(err)
	}
	c.CreatedAt, c.UpdatedAt = created, updated
	return nil
}

func (s *Store) CategoryByID(ctx context.Context, id int64) (catalogue.Category, error) {
	row := s.exec(ctx).QueryRow(ctx, `SELECT `+categoryColumns+` FROM catalogue.categories WHERE id = $1`, id)
	c, err := s.scanCategory(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalogue.Category{}, catalogue.ErrNoCategory
	}
	if err != nil {
		return catalogue.Category{}, fmt.Errorf("reading category: %w", err)
	}
	return c, nil
}

func (s *Store) CategoryBySlug(ctx context.Context, slug string) (catalogue.Category, error) {
	row := s.exec(ctx).QueryRow(ctx, `SELECT `+categoryColumns+` FROM catalogue.categories WHERE slug = $1`, slug)
	c, err := s.scanCategory(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalogue.Category{}, catalogue.ErrNoCategory
	}
	if err != nil {
		return catalogue.Category{}, fmt.Errorf("reading category: %w", err)
	}
	return c, nil
}

func (s *Store) ListCategories(ctx context.Context, page, pageSize int) (catalogue.CategoryList, error) {
	// total
	var total int64
	if err := s.exec(ctx).QueryRow(ctx, `SELECT count(*) FROM catalogue.categories WHERE is_active = true`).Scan(&total); err != nil {
		return catalogue.CategoryList{}, err
	}
	offset := (page - 1) * pageSize
	rows, err := s.exec(ctx).Query(ctx, `SELECT `+categoryColumns+` FROM catalogue.categories WHERE is_active = true ORDER BY name ASC LIMIT $1 OFFSET $2`, pageSize, offset)
	if err != nil {
		return catalogue.CategoryList{}, err
	}
	defer rows.Close()
	items := []catalogue.Category{}
	for rows.Next() {
		c, err := s.scanCategory(rows)
		if err != nil {
			return catalogue.CategoryList{}, err
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		return catalogue.CategoryList{}, err
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if total == 0 {
		totalPages = 0
	}
	return catalogue.CategoryList{
		Items: items,
		Meta:  catalogue.PageMeta{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages},
	}, nil
}

func (s *Store) ActiveCategories(ctx context.Context) ([]catalogue.Category, error) {
	rows, err := s.exec(ctx).Query(ctx, `SELECT `+categoryColumns+` FROM catalogue.categories WHERE is_active = true ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []catalogue.Category{}
	for rows.Next() {
		c, err := s.scanCategory(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (s *Store) UpdateCategory(ctx context.Context, c catalogue.Category) error {
	tag, err := s.exec(ctx).Exec(ctx, `
		UPDATE catalogue.categories
		SET name=$2, slug=$3, icon=$4, parent_id=$5, is_active=$6, updated_at=now()
		WHERE id=$1`,
		c.ID, c.Name, c.Slug, c.Icon, c.ParentID, c.IsActive)
	if err != nil {
		return wrapUnique(err)
	}
	if tag.RowsAffected() == 0 {
		return catalogue.ErrNoCategory
	}
	return nil
}

func (s *Store) DeleteCategory(ctx context.Context, id int64) error {
	tag, err := s.exec(ctx).Exec(ctx, `DELETE FROM catalogue.categories WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return catalogue.ErrNoCategory
	}
	return nil
}

func (s *Store) CountProductsByCategory(ctx context.Context, categoryID int64, publishedOnly bool) (int64, error) {
	q := `SELECT count(*) FROM catalogue.products WHERE category_id = $1`
	args := []any{categoryID}
	if publishedOnly {
		q += ` AND publication_state = 'published'`
	}
	var n int64
	err := s.exec(ctx).QueryRow(ctx, q, args...).Scan(&n)
	return n, err
}

// ── products ──────────────────────────────────────────────────────────────────

const productColumns = `id, sku, name, slug, description, brand, price_cents, compare_at_price_cents,
	category_id, subcategory, publication_state, is_featured, specifications, views, rating_avg,
	rating_count, low_stock_threshold, critical_stock_threshold, created_at, updated_at,
	published_at, archived_at`

func (s *Store) scanProduct(row pgx.Row) (catalogue.Product, error) {
	var p catalogue.Product
	var description, brand, subcategory *string
	var compareAt *int64
	var publishedAt, archivedAt *time.Time
	var specs []byte
	err := row.Scan(
		&p.ID, &p.SKU, &p.Name, &p.Slug, &description, &brand, &p.PriceCents, &compareAt,
		&p.CategoryID, &subcategory, &p.PublicationState, &p.IsFeatured, &specs, &p.Views,
		&p.RatingAvg, &p.RatingCount, &p.LowStockThreshold, &p.CriticalStockThreshold,
		&p.CreatedAt, &p.UpdatedAt, &publishedAt, &archivedAt,
	)
	if err != nil {
		return catalogue.Product{}, err
	}
	p.Description = description
	p.Brand = brand
	p.Subcategory = subcategory
	p.CompareAtPriceCents = compareAt
	p.PublishedAt = publishedAt
	p.ArchivedAt = archivedAt
	if len(specs) > 0 {
		_ = json.Unmarshal(specs, &p.Specifications)
	}
	if p.Specifications == nil {
		p.Specifications = map[string]any{}
	}
	return p, nil
}

func (s *Store) CreateProduct(ctx context.Context, p *catalogue.Product) error {
	specs := p.Specifications
	if specs == nil {
		specs = map[string]any{}
	}
	sb, _ := json.Marshal(specs)
	row := s.exec(ctx).QueryRow(ctx, `
		INSERT INTO catalogue.products
			(sku, name, slug, description, brand, price_cents, compare_at_price_cents,
			 category_id, subcategory, publication_state, is_featured, specifications)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb)
		RETURNING id, created_at, updated_at`,
		p.SKU, p.Name, p.Slug, p.Description, p.Brand, p.PriceCents, p.CompareAtPriceCents,
		p.CategoryID, p.Subcategory, string(p.PublicationState), p.IsFeatured, string(sb))
	var created, updated time.Time
	if err := row.Scan(&p.ID, &created, &updated); err != nil {
		return wrapUnique(err)
	}
	p.CreatedAt, p.UpdatedAt = created, updated
	return nil
}

func (s *Store) ProductByID(ctx context.Context, id int64) (catalogue.Product, error) {
	row := s.exec(ctx).QueryRow(ctx, `SELECT `+productColumns+` FROM catalogue.products WHERE id = $1`, id)
	p, err := s.scanProduct(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalogue.Product{}, catalogue.ErrNoProduct
	}
	if err != nil {
		return catalogue.Product{}, fmt.Errorf("reading product: %w", err)
	}
	return p, nil
}

func (s *Store) ProductBySlug(ctx context.Context, slug string) (catalogue.Product, error) {
	row := s.exec(ctx).QueryRow(ctx, `SELECT `+productColumns+` FROM catalogue.products WHERE slug = $1`, slug)
	p, err := s.scanProduct(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalogue.Product{}, catalogue.ErrNoProduct
	}
	if err != nil {
		return catalogue.Product{}, fmt.Errorf("reading product: %w", err)
	}
	return p, nil
}

func (s *Store) ProductByReference(ctx context.Context, ref string) (catalogue.Product, error) {
	// try numeric id first
	if id, err := parseID(ref); err == nil {
		return s.ProductByID(ctx, id)
	}
	return s.ProductBySlug(ctx, ref)
}

func (s *Store) ListProducts(ctx context.Context, f catalogue.ListProductsFilter) (catalogue.ProductList, []catalogue.Category, error) {
	// build where
	conds := []string{}
	args := []any{}
	argN := 1
	add := func(cond string, val any) {
		conds = append(conds, fmt.Sprintf(cond, argN))
		args = append(args, val)
		argN++
	}
	if f.Publication != nil {
		add("publication_state = $%d", string(*f.Publication))
	}
	if f.CategorySlug != "" {
		// join categories
		conds = append(conds, fmt.Sprintf("p.category_id = (SELECT id FROM catalogue.categories WHERE slug = $%d)", argN))
		args = append(args, f.CategorySlug)
		argN++
	}
	if f.Featured != nil {
		add("is_featured = $%d", *f.Featured)
	}
	if f.MinPriceCents != nil {
		add("price_cents >= $%d", *f.MinPriceCents)
	}
	if f.MaxPriceCents != nil {
		add("price_cents <= $%d", *f.MaxPriceCents)
	}
	if strings.TrimSpace(f.Query) != "" {
		conds = append(conds, fmt.Sprintf("to_tsvector('simple', name || ' ' || coalesce(description,'') || ' ' || coalesce(subcategory,'')) @@ plainto_tsquery('simple', $%d)", argN))
		args = append(args, f.Query)
		argN++
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	// count
	countQ := "SELECT count(*) FROM catalogue.products p " + where
	// where references columns without alias except category subquery; adjust: replace bare columns? We used bare column names in conds (publication_state, is_featured, etc.) which exist on products. Fine, but table alias p. The FROM is products p, so bare columns resolve. OK.
	var total int64
	if err := s.exec(ctx).QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return catalogue.ProductList{}, nil, err
	}

	// sort
	orderBy := "p.id DESC"
	switch f.Sort {
	case "price_asc":
		orderBy = "p.price_cents ASC"
	case "price_desc":
		orderBy = "p.price_cents DESC"
	case "name_asc":
		orderBy = "p.name ASC"
	case "newest":
		orderBy = "p.id DESC"
	}

	offset := (f.Page - 1) * f.PageSize
	listQ := fmt.Sprintf(`SELECT %s FROM catalogue.products p %s ORDER BY %s LIMIT $%d OFFSET $%d`,
		strings.Replace(productColumns, "", "p.", -1), // hmm need alias prefix
		where, orderBy, argN, argN+1)
	// The above replace is wrong; do manually.
	_ = listQ

	// rebuild productColumns with p. prefix
	prefixed := prefixColumns(productColumns, "p.")
	listQ = fmt.Sprintf(`SELECT %s FROM catalogue.products p %s ORDER BY %s LIMIT $%d OFFSET $%d`,
		prefixed, where, orderBy, argN, argN+1)
	args2 := append([]any{}, args...)
	args2 = append(args2, f.PageSize, offset)

	rows, err := s.exec(ctx).Query(ctx, listQ, args2...)
	if err != nil {
		return catalogue.ProductList{}, nil, err
	}
	defer rows.Close()
	items := []catalogue.Product{}
	for rows.Next() {
		p, err := s.scanProduct(rows)
		if err != nil {
			return catalogue.ProductList{}, nil, err
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		return catalogue.ProductList{}, nil, err
	}

	// categories
	cats, err := s.ActiveCategories(ctx)
	if err != nil {
		return catalogue.ProductList{}, nil, err
	}

	totalPages := int((total + int64(f.PageSize) - 1) / int64(f.PageSize))
	if total == 0 {
		totalPages = 0
	}
	return catalogue.ProductList{
		Items: items,
		Meta:  catalogue.PageMeta{Page: f.Page, PageSize: f.PageSize, Total: total, TotalPages: totalPages},
	}, cats, nil
}

func (s *Store) UpdateProduct(ctx context.Context, p catalogue.Product) error {
	specs := p.Specifications
	if specs == nil {
		specs = map[string]any{}
	}
	sb, _ := json.Marshal(specs)
	tag, err := s.exec(ctx).Exec(ctx, `
		UPDATE catalogue.products
		SET name=$2, description=$3, brand=$4, category_id=$5, subcategory=$6,
		    is_featured=$7, specifications=$8::jsonb, updated_at=now()
		WHERE id=$1`,
		p.ID, p.Name, p.Description, p.Brand, p.CategoryID, p.Subcategory, p.IsFeatured, string(sb))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return catalogue.ErrNoProduct
	}
	return nil
}

func (s *Store) UpdatePrice(ctx context.Context, id int64, priceCents int64, compareAt *int64) error {
	tag, err := s.exec(ctx).Exec(ctx, `
		UPDATE catalogue.products
		SET price_cents=$2, compare_at_price_cents=$3, updated_at=now()
		WHERE id=$1`,
		id, priceCents, compareAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return catalogue.ErrNoProduct
	}
	return nil
}

func (s *Store) SetPublication(ctx context.Context, id int64, state catalogue.PublicationState, publishedAt, archivedAt *time.Time) error {
	tag, err := s.exec(ctx).Exec(ctx, `
		UPDATE catalogue.products
		SET publication_state=$2, published_at=COALESCE($3, published_at), archived_at=COALESCE($4, archived_at), updated_at=now()
		WHERE id=$1`,
		id, string(state), publishedAt, archivedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return catalogue.ErrNoProduct
	}
	return nil
}

func (s *Store) ArchiveProduct(ctx context.Context, id int64) error {
	return s.DeleteProduct(ctx, id)
}

func (s *Store) DeleteProduct(ctx context.Context, id int64) error {
	tag, err := s.exec(ctx).Exec(ctx, `DELETE FROM catalogue.products WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return catalogue.ErrNoProduct
	}
	return nil
}

func (s *Store) HasOrderHistory(ctx context.Context, productID int64) (bool, error) {
	// commerce.order_lines may not exist yet; check table existence
	var exists bool
	err := s.exec(ctx).QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='commerce' AND table_name='order_lines')`).Scan(&exists)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	var n int64
	err = s.exec(ctx).QueryRow(ctx, `SELECT count(*) FROM commerce.order_lines WHERE product_id = $1`, productID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ── images ────────────────────────────────────────────────────────────────────

func (s *Store) CreateImage(ctx context.Context, img *catalogue.ProductImage) error {
	row := s.exec(ctx).QueryRow(ctx, `
		INSERT INTO catalogue.product_images (product_id, url, alt_text, position, provider, provider_ref, is_primary)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, created_at`,
		img.ProductID, img.URL, img.AltText, img.Position, img.Provider, img.ProviderRef, img.IsPrimary)
	var created time.Time
	if err := row.Scan(&img.ID, &created); err != nil {
		return wrapUnique(err)
	}
	img.CreatedAt = created
	if img.Provider == "" {
		img.Provider = "local"
	}
	return nil
}

func (s *Store) ImagesByProduct(ctx context.Context, productID int64) ([]catalogue.ProductImage, error) {
	rows, err := s.exec(ctx).Query(ctx, `
		SELECT id, product_id, url, alt_text, position, provider, provider_ref, is_primary, created_at
		FROM catalogue.product_images WHERE product_id = $1 ORDER BY is_primary DESC, position ASC, id ASC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []catalogue.ProductImage{}
	for rows.Next() {
		var img catalogue.ProductImage
		var alt, pref *string
		var provider string
		if err := rows.Scan(&img.ID, &img.ProductID, &img.URL, &alt, &img.Position, &provider, &pref, &img.IsPrimary, &img.CreatedAt); err != nil {
			return nil, err
		}
		img.AltText = alt
		img.ProviderRef = pref
		img.Provider = provider
		items = append(items, img)
	}
	return items, rows.Err()
}

func (s *Store) DeleteImagesByProduct(ctx context.Context, productID int64) error {
	_, err := s.exec(ctx).Exec(ctx, `DELETE FROM catalogue.product_images WHERE product_id = $1`, productID)
	return err
}

func (s *Store) SetPrimaryImage(ctx context.Context, productID, imageID int64) error {
	// unset all primary first
	if _, err := s.exec(ctx).Exec(ctx, `UPDATE catalogue.product_images SET is_primary=false WHERE product_id=$1`, productID); err != nil {
		return err
	}
	_, err := s.exec(ctx).Exec(ctx, `UPDATE catalogue.product_images SET is_primary=true WHERE id=$1 AND product_id=$2`, imageID, productID)
	return err
}

// ── helpers ───────────────────────────────────────────────────────────────────

func wrapUnique(err error) error {
	if err == nil {
		return nil
	}
	if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
		return err // let service decide which taken error; but service already checked some. We'll map in service by err string?
	}
	return err
}

func parseID(s string) (int64, error) {
	// only pure digits
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not numeric")
		}
	}
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func prefixColumns(cols, prefix string) string {
	// prefix each column name (handle multi-line, commas)
	parts := strings.Split(cols, ",")
	out := []string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// if already has dot or function, skip
		if strings.Contains(p, ".") || strings.Contains(p, "(") {
			out = append(out, p)
			continue
		}
		out = append(out, prefix+p)
	}
	return strings.Join(out, ", ")
}
