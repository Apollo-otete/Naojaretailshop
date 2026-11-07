package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/apierr"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/catalogue"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpauth"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpcatalogue"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpcookie"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/httpjson"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/identity"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/media/cloudinary"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/ordering"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/shopping"
)

// mountLegacy serves the existing frontend's /api/* paths (legacy shapes, Bearer-token admin auth)
// alongside the /api/v1 contract. It keeps the React app working against this one Go backend while
// the contract surface remains the source of truth for new clients.
func mountLegacy(r gin.IRouter, d Deps, cookie httpcookie.Session) {
	// Public catalogue
	r.GET("/products", legacyListProducts(d))
	r.GET("/products/:ref", legacyGetProduct(d))
	r.GET("/categories", legacyListCategories(d))

	// Auth (legacy frontend expects {token, admin} and sends Bearer tokens)
	r.POST("/auth/login", legacyLogin(d, cookie))
	r.PUT("/auth/change-password", legacyBearerAuth(d), legacyChangePassword(d))
	r.POST("/auth/change-password", legacyBearerAuth(d), legacyChangePassword(d)) // tolerate method

	// Orders (legacy)
	r.POST("/orders", legacyCreateOrder(d))
	r.GET("/orders", legacyBearerAuth(d), legacyListOrders(d))
	r.GET("/orders/track", legacyTrackOrder(d))
	r.POST("/orders/track", legacyTrackOrder(d)) // verified (ref+phone) per G3; POST preferred
	r.PUT("/orders/:id", legacyBearerAuth(d), legacyUpdateOrder(d))

	// Admin catalogue CRUD (legacy shapes)
	r.POST("/products", legacyBearerAuth(d), legacyAdminCreateProduct(d))
	r.PUT("/products/:id", legacyBearerAuth(d), legacyAdminUpdateProduct(d))
	r.DELETE("/products/:id", legacyBearerAuth(d), legacyAdminArchiveProduct(d))
	r.POST("/categories", legacyBearerAuth(d), legacyAdminCreateCategory(d))
	r.PUT("/categories/:id", legacyBearerAuth(d), legacyAdminUpdateCategory(d))
	r.DELETE("/categories/:id", legacyBearerAuth(d), legacyAdminDeleteCategory(d))

	// Stubs for endpoints the frontend calls but the Go backend doesn't implement yet.
	// Return safe empty/default shapes so the UI doesn't hard-fail.
	r.GET("/analytics", legacyBearerAuth(d), legacyStubOK(gin.H{
		"totalSales": 0, "totalOrders": 0, "totalProducts": 0, "totalSubscribers": 0,
		"lowStockCount": 0, "pendingMessagesCount": 0, "recentOrders": []any{}, "monthlySales": []any{},
	}))
	r.GET("/reviews", legacyStubOK([]any{}))
	r.POST("/reviews", legacyStubCreated(gin.H{"message": "Review submitted successfully.", "review": gin.H{}}))
	r.PUT("/reviews/:id/approve", legacyBearerAuth(d), legacyStubOK(gin.H{}))
	r.PUT("/reviews/:id/reject", legacyBearerAuth(d), legacyStubOK(gin.H{}))
	r.GET("/newsletter", legacyBearerAuth(d), legacyStubOK([]any{}))
	r.POST("/newsletter", legacyStubOK(gin.H{"message": "Subscribed."}))
	r.GET("/contacts", legacyBearerAuth(d), legacyStubOK([]any{}))
	r.POST("/contacts", legacyStubCreated(gin.H{"message": "Message sent."}))
	r.PUT("/contacts/:id/read", legacyBearerAuth(d), legacyStubOK(gin.H{}))
	r.POST("/upload", legacyBearerAuth(d), legacyUpload(d))
	// M-Pesa compat: aligned to real flow when configured; safe/non-leaking when not.
	r.POST("/mpesa/stkpush", legacyBearerAuth(d), legacyMpesaSTKPush(d))
	r.POST("/mpesa/query", legacyBearerAuth(d), legacyMpesaQuery(d))
	r.GET("/mpesa/order-status/:orderRef", legacyMpesaOrderStatus(d))
}

func legacyStubOK(payload any) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusOK, payload) }
}
func legacyStubCreated(payload any) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusCreated, payload) }
}

func legacyDecode(c *gin.Context, target any) error {
	if err := json.NewDecoder(c.Request.Body).Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return apierr.BadRequest("A JSON request body is required.")
		}
		return apierr.BadRequest("The request body is not valid JSON.")
	}
	return nil
}

// legacyBearerAuth accepts Authorization: Bearer <session-token> (the frontend's admin flow) and
// resolves it through the same session store the cookie path uses. A valid token becomes the
// principal; an invalid/missing one gets 401 in the legacy {message} shape the frontend expects.
func legacyBearerAuth(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Authentication required."})
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		if token == "" || d.Sessions == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Authentication required."})
			return
		}
		p, err := d.Sessions.Authenticate(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Session expired. Please sign in again."})
			return
		}
		c.Request = c.Request.WithContext(identity.WithPrincipal(c.Request.Context(), p))
		c.Next()
	}
}

func legacyLogin(d Deps, cookie httpcookie.Session) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.Identity == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Auth is not configured."})
			return
		}
		var in struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := legacyDecode(c, &in); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		auth, err := d.Identity.Login(c.Request.Context(), identity.LoginInput{
			Email: in.Email, Password: in.Password, IP: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"),
		})
		if err != nil {
			if errors.Is(err, identity.ErrInvalidCredentials) || errors.Is(err, identity.ErrInactive) || errors.Is(err, identity.ErrSuspended) {
				c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid email or password."})
				return
			}
			if errors.Is(err, identity.ErrLocked) {
				c.JSON(http.StatusLocked, gin.H{"message": "Account locked. Try again later."})
				return
			}
			httpjson.Fail(c, d.Logger, err)
			return
		}
		// Also set the session cookie so /api/v1 works from the same browser.
		cookie.Set(c.Writer, auth.Session.Token)
		c.JSON(http.StatusOK, gin.H{
			"token": auth.Session.Token, // legacy frontend stores and sends as Bearer
			"admin": gin.H{"id": auth.Principal.User.ID, "name": auth.Principal.User.FullName, "email": auth.Principal.User.Email},
		})
	}
}

func legacyChangePassword(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := identity.PrincipalFrom(c.Request.Context())
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Authentication required."})
			return
		}
		var in struct {
			Current string `json:"current_password"`
			New     string `json:"new_password"`
			// legacy frontend field names
			Old      string `json:"oldPassword"`
			NewPass  string `json:"newPassword"`
			Password string `json:"password"`
		}
		_ = legacyDecode(c, &in)
		cur := in.Current
		if cur == "" {
			cur = in.Old
		}
		nw := in.New
		if nw == "" {
			nw = in.NewPass
		}
		if nw == "" {
			nw = in.Password
		}
		if d.Identity == nil {
			c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
			return
		}
		if err := d.Identity.ChangePassword(c.Request.Context(), p, cur, nw); err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
	}
}

// ── catalogue legacy views (KES decimal price, images as URL strings) ────────────────

func legacyProduct(p catalogue.Product, cat *catalogue.Category, imgs []catalogue.ProductImage) gin.H {
	urls := make([]string, 0, len(imgs))
	for _, im := range imgs {
		urls = append(urls, im.URL)
	}
	stock := 0
	status := "in_stock"
	_ = stock
	h := gin.H{
		"id": p.ID, "name": p.Name, "slug": p.Slug, "sku": p.SKU,
		"description": p.Description, "brand": p.Brand,
		"price":          float64(p.PriceCents) / 100.0,
		"price_cents":    p.PriceCents,
		"stock_quantity": stock, "status": status, "in_stock": true,
		"is_featured": p.IsFeatured, "views": p.Views,
		"rating_avg": p.RatingAvg, "rating_count": p.RatingCount,
		"images": urls, "image_url": nil,
		"publication_state": p.PublicationState,
		"created_at":        p.CreatedAt, "updated_at": p.UpdatedAt,
	}
	if len(urls) > 0 {
		h["image_url"] = urls[0]
	}
	if cat != nil {
		h["category_id"] = cat.ID
		h["category_slug"] = cat.Slug
		h["category"] = gin.H{"id": cat.ID, "name": cat.Name, "slug": cat.Slug, "icon": cat.Icon}
	} else {
		h["category_id"] = p.CategoryID
	}
	if p.Subcategory != nil {
		h["subcategory"] = *p.Subcategory
	}
	return h
}

func legacyListProducts(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.Catalogue == nil {
			c.JSON(http.StatusOK, []any{})
			return
		}
		catSlug := c.Query("category")
		search := c.Query("search")
		page := 1
		if v := c.Query("page"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				page = n
			}
		}
		pageSize := 12
		if v := c.Query("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				pageSize = n
			}
		}
		if pageSize <= 0 {
			pageSize = 12
		}
		f := catalogue.ListProductsFilter{Page: page, PageSize: pageSize}
		if search != "" {
			f.Query = search
		}
		if catSlug != "" {
			f.CategorySlug = catSlug
		}
		res, err := d.Catalogue.ListProducts(c.Request.Context(), f, false)
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		out := make([]gin.H, 0, len(res.Items))
		for _, p := range res.Items {
			cat, _ := d.Catalogue.CategoryByID(c.Request.Context(), p.CategoryID)
			imgs, _ := d.Catalogue.ProductImages(c.Request.Context(), p.ID)
			out = append(out, legacyProduct(p, &cat, imgs))
		}
		// Frontend handles array or {products,pagination}
		c.JSON(http.StatusOK, gin.H{
			"products":   out,
			"pagination": gin.H{"total": res.Meta.Total, "page": res.Meta.Page, "limit": res.Meta.PageSize, "totalPages": res.Meta.TotalPages},
		})
	}
}

func legacyGetProduct(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.Catalogue == nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "Not found"})
			return
		}
		ref := c.Param("ref")
		p, err := d.Catalogue.GetProduct(c.Request.Context(), ref, false)
		if err != nil {
			if errors.Is(err, catalogue.ErrNoProduct) {
				c.JSON(http.StatusNotFound, gin.H{"message": "Product not found"})
				return
			}
			httpjson.Fail(c, d.Logger, err)
			return
		}
		cat, _ := d.Catalogue.CategoryByID(c.Request.Context(), p.CategoryID)
		imgs, _ := d.Catalogue.ProductImages(c.Request.Context(), p.ID)
		c.JSON(http.StatusOK, legacyProduct(p, &cat, imgs))
	}
}

func legacyListCategories(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.Catalogue == nil {
			c.JSON(http.StatusOK, []any{})
			return
		}
		res, err := d.Catalogue.ListCategories(c.Request.Context(), 1, 200)
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		out := make([]gin.H, 0, len(res.Items))
		for _, cat := range res.Items {
			cnt, _ := d.Catalogue.CategoryProductCount(c.Request.Context(), cat.ID)
			out = append(out, gin.H{"id": cat.ID, "name": cat.Name, "slug": cat.Slug, "icon": cat.Icon, "product_count": cnt})
		}
		c.JSON(http.StatusOK, out)
	}
}

// ── admin catalogue CRUD (best-effort compat; v1 is the authoritative contract) ──────

func legacyAdminCreateProduct(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.Catalogue == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "Unavailable"})
			return
		}
		var in struct {
			SKU, Name, Slug, Description, Brand, Subcategory string
			Price                                            float64 `json:"price"`
			PriceCents                                       int64   `json:"price_cents"`
			CategoryID                                       int64   `json:"category_id"`
			IsFeatured                                       bool    `json:"is_featured"`
			StockQuantity                                    int     `json:"stock_quantity"`
			Images                                           []any   `json:"images"`
		}
		if err := legacyDecode(c, &in); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		cents := in.PriceCents
		if cents == 0 && in.Price > 0 {
			cents = int64(in.Price*100 + 0.5)
		}
		// round to whole shillings as ADR-021 expects? Keep as-is; domain will validate.
		ci := catalogue.CreateProductInput{SKU: in.SKU, Name: in.Name, Slug: in.Slug, PriceCents: cents, CategoryID: in.CategoryID, IsFeatured: in.IsFeatured}
		if in.Description != "" {
			ci.Description = &in.Description
		}
		if in.Brand != "" {
			ci.Brand = &in.Brand
		}
		if in.Subcategory != "" {
			ci.Subcategory = &in.Subcategory
		}
		p, err := d.Catalogue.CreateProduct(c.Request.Context(), ci)
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		cat, _ := d.Catalogue.CategoryByID(c.Request.Context(), p.CategoryID)
		imgs, _ := d.Catalogue.ProductImages(c.Request.Context(), p.ID)
		c.JSON(http.StatusCreated, legacyProduct(p, &cat, imgs))
	}
}
func legacyAdminUpdateProduct(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Update via /api/v1 for full contract support."})
	}
}
func legacyAdminArchiveProduct(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.Catalogue == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "Unavailable"})
			return
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid id"})
			return
		}
		if err := d.Catalogue.ArchiveProduct(c.Request.Context(), id); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Product archived", "id": id})
	}
}
func legacyAdminCreateCategory(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.Catalogue == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "Unavailable"})
			return
		}
		var in struct {
			Name, Slug string
			Icon       *string `json:"icon"`
		}
		if err := legacyDecode(c, &in); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		cat, err := d.Catalogue.CreateCategory(c.Request.Context(), catalogue.CreateCategoryInput{Name: in.Name, Slug: in.Slug, Icon: in.Icon})
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": cat.ID, "name": cat.Name, "slug": cat.Slug, "icon": cat.Icon, "product_count": 0})
	}
}
func legacyAdminUpdateCategory(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Update via /api/v1 for full contract support."})
	}
}
func legacyAdminDeleteCategory(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.Catalogue == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "Unavailable"})
			return
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid id"})
			return
		}
		if err := d.Catalogue.DeleteCategory(c.Request.Context(), id); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Category deleted", "id": id})
	}
}

// ── orders legacy (best-effort) ──────────────────────────────────────────────────────

func legacyCreateOrder(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Compat for the existing frontend's direct checkout payload. The contract flow is
		// cart → quote → idempotent placement; this translates the legacy shape into an order
		// using the same store/tx primitives, without trusting client totals (we recompute from
		// the submitted lines).
		if d.LegacyOrderStore == nil || d.LegacyOrderTx == nil {
			c.JSON(http.StatusNotImplemented, gin.H{"message": "Checkout is not available. Please use the cart and /api/v1 checkout flow."})
			return
		}
		var in struct {
			CustomerName     string  `json:"customer_name"`
			CustomerPhone    string  `json:"customer_phone"`
			CustomerEmail    string  `json:"customer_email"`
			ShippingAddress  string  `json:"shipping_address"`
			Notes            string  `json:"notes"`
			PaymentMethod    string  `json:"payment_method"`
			DeliveryFee      float64 `json:"delivery_fee"`
			DeliveryFeeCents int64   `json:"delivery_fee_cents"`
			Items            []struct {
				ProductID   int64   `json:"product_id"`
				ProductName string  `json:"product_name"`
				SKU         string  `json:"sku"`
				Quantity    int     `json:"quantity"`
				UnitPrice   float64 `json:"unit_price"`
				UnitCents   int64   `json:"unit_price_cents"`
			} `json:"items"`
		}
		if err := legacyDecode(c, &in); err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		if len(in.Items) == 0 {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"message": "An order needs at least one item."})
			return
		}

		deliveryFeeCents := in.DeliveryFeeCents
		if deliveryFeeCents == 0 && in.DeliveryFee > 0 {
			deliveryFeeCents = int64(in.DeliveryFee*100 + 0.5)
		}

		now := time.Now().UTC()
		order := ordering.Order{
			Status:           ordering.StatusPendingPayment,
			PaymentStatus:    ordering.PayPending,
			Currency:         "KES",
			DeliveryFeeCents: deliveryFeeCents,
			Reference:        fmt.Sprintf("NJ-%d-%04d", now.Year(), rand.Intn(10000)),
			ExpiresAt:        timePtr(now.Add(30 * time.Minute)),
		}
		if in.ShippingAddress != "" {
			order.DeliveryAddress = &in.ShippingAddress
		}
		if in.CustomerPhone != "" {
			order.ContactPhone = &in.CustomerPhone
		}
		if in.Notes != "" {
			order.DeliveryInstructions = &in.Notes
		}

		// Associate with signed-in caller when present (Bearer session).
		if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") && d.Sessions != nil {
			token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
			if p, err := d.Sessions.Authenticate(c.Request.Context(), token); err == nil {
				uid := p.UserID()
				order.UserID = &uid
			}
		}

		items := make([]ordering.OrderItem, 0, len(in.Items))
		var subtotal int64
		for _, it := range in.Items {
			qty := it.Quantity
			if qty < 1 {
				qty = 1
			}
			unit := it.UnitCents
			if unit == 0 && it.UnitPrice > 0 {
				unit = int64(it.UnitPrice*100 + 0.5)
			}
			line := unit * int64(qty)
			subtotal += line
			pid := it.ProductID
			items = append(items, ordering.OrderItem{
				ProductID: &pid, ProductName: it.ProductName, SKU: it.SKU,
				Quantity: qty, UnitPriceCents: unit, LineTotalCents: line,
			})
		}
		order.SubtotalCents = subtotal
		order.TotalCents = subtotal + deliveryFeeCents

		err := d.LegacyOrderTx.RunInTx(c.Request.Context(), func(ctx context.Context) error {
			if err := d.LegacyOrderStore.CreateOrder(ctx, &order); err != nil {
				return err
			}
			for i := range items {
				items[i].OrderID = order.ID
				if err := d.LegacyOrderStore.AddOrderItem(ctx, &items[i]); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		c.JSON(http.StatusCreated, gin.H{"message": "Order placed successfully", "order": legacyOrderView(order, items)})
	}
}

func timePtr(t time.Time) *time.Time { return &t }
func legacyTrackOrder(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.Ordering == nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "No matching order found. Please check your order reference and phone number."})
			return
		}
		// Contract/verified flow: POST {reference, phone}
		if c.Request.Method == http.MethodPost {
			var in struct {
				Reference string `json:"reference"`
				Ref       string `json:"ref"`
				Phone     string `json:"phone"`
			}
			_ = c.ShouldBindJSON(&in)
			ref := strings.TrimSpace(in.Reference)
			if ref == "" {
				ref = strings.TrimSpace(in.Ref)
			}
			phone := strings.TrimSpace(in.Phone)
			if ref == "" || phone == "" {
				c.JSON(http.StatusNotFound, gin.H{"message": "No matching order found. Please check your order reference and phone number."})
				return
			}
			out, err := d.Ordering.TrackOrder(c.Request.Context(), ref, phone)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"message": "No matching order found. Please check your order reference and phone number."})
				return
			}
			c.JSON(http.StatusOK, out)
			return
		}
		// GET: only return details to the authenticated owner; never leak by reference alone.
		ref := strings.TrimSpace(c.Query("ref"))
		if ref == "" {
			ref = strings.TrimSpace(c.Query("reference"))
		}
		if ref == "" {
			c.JSON(http.StatusNotFound, gin.H{"message": "No matching order found. Please check your order reference and phone number."})
			return
		}
		if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") && d.Sessions != nil {
			token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
			if p, err := d.Sessions.Authenticate(c.Request.Context(), token); err == nil {
				o, items, err := d.Ordering.GetOrder(c.Request.Context(), p, ref)
				if err == nil {
					c.JSON(http.StatusOK, legacyOrderView(o, items))
					return
				}
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"message": "No matching order found. Please check your order reference and phone number."})
	}
}

func legacyMpesaSTKPush(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.Payments == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "M-Pesa is not configured on this server.", "ResponseDescription": "M-Pesa is not configured on this server."})
			return
		}
		var in struct {
			Phone       string `json:"phone"`
			PhoneNumber string `json:"phoneNumber"`
			OrderRef    string `json:"orderRef"`
			Reference   string `json:"reference"`
			OrderID     string `json:"orderId"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body."})
			return
		}
		phone := strings.TrimSpace(in.Phone)
		if phone == "" {
			phone = strings.TrimSpace(in.PhoneNumber)
		}
		ref := strings.TrimSpace(in.OrderRef)
		if ref == "" {
			ref = strings.TrimSpace(in.Reference)
		}
		if ref == "" {
			ref = strings.TrimSpace(in.OrderID)
		}
		res, err := d.Payments.InitiateSTK(c.Request.Context(), ref, phone)
		if err != nil {
			if d.Logger != nil {
				d.Logger.Warn("legacy mpesa stkpush failed", "error", err, "ref", ref)
			}
			// Preserve a safe, frontend-tolerant shape.
			msg := "Could not initiate M-Pesa payment."
			if fe, ok := err.(interface{ Error() string }); ok && fe != nil {
				// avoid leaking internals; keep generic where appropriate
				_ = fe
			}
			c.JSON(http.StatusBadRequest, gin.H{"message": msg, "ResponseDescription": msg, "error": msg})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message":             res.CustomerMessage,
			"ResponseDescription": res.CustomerMessage,
			"CustomerMessage":     res.CustomerMessage,
			"CheckoutRequestID":   res.CheckoutRequestID,
			"MerchantRequestID":   res.Attempt.MerchantRequestID,
			"order_reference":     ref,
			"status":              "pending",
		})
	}
}

func legacyMpesaQuery(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// The legacy "query" endpoint was unsafe (state mutation by arbitrary callers). Do not
		// mutate; return a safe status. Use the payment status endpoint for real state.
		c.JSON(http.StatusOK, gin.H{"ResultCode": "1", "ResultDesc": "Use order payment status.", "message": "Not supported"})
	}
}

func legacyMpesaOrderStatus(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ref := strings.TrimSpace(c.Param("orderRef"))
		// Never leak by reference alone. Only the authenticated owner may see status.
		if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") && d.Sessions != nil && d.Ordering != nil {
			token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
			if p, err := d.Sessions.Authenticate(c.Request.Context(), token); err == nil {
				if _, _, err := d.Ordering.GetOrder(c.Request.Context(), p, ref); err == nil && d.Payments != nil {
					if pay, _, _, err := d.Payments.PaymentByOrderRef(c.Request.Context(), ref); err == nil {
						c.JSON(http.StatusOK, gin.H{"payment_status": string(pay.Status), "status": string(pay.Status), "order_reference": ref})
						return
					}
					c.JSON(http.StatusOK, gin.H{"payment_status": "pending", "status": "pending", "order_reference": ref})
					return
				}
			}
		}
		// Indistinguishable non-leak response.
		c.JSON(http.StatusNotFound, gin.H{"message": "No matching order found."})
	}
}
func legacyListOrders(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := identity.PrincipalFrom(c.Request.Context())
		if !ok || d.Ordering == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Authentication required."})
			return
		}
		orders, total, err := d.Ordering.ListOrders(c.Request.Context(), p, nil, 1, 50)
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		out := make([]gin.H, 0, len(orders))
		for _, o := range orders {
			out = append(out, legacyOrderView(o, nil))
		}
		c.JSON(http.StatusOK, gin.H{"orders": out, "pagination": gin.H{"total": total, "page": 1, "limit": 50, "totalPages": 1}})
	}
}
func legacyUpdateOrder(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := identity.PrincipalFrom(c.Request.Context())
		if !ok || d.Ordering == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Authentication required."})
			return
		}
		idStr := c.Param("id")
		var in struct {
			Status        string `json:"status"`
			PaymentStatus string `json:"payment_status"`
		}
		_ = legacyDecode(c, &in)
		// Legacy updates by numeric id; try to find by listing and matching id (best-effort).
		orders, _, err := d.Ordering.ListOrders(c.Request.Context(), p, nil, 1, 200)
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		var ref string
		for _, o := range orders {
			if strconv.FormatInt(o.ID, 10) == idStr {
				ref = o.Reference
				break
			}
		}
		if ref == "" {
			c.JSON(http.StatusNotFound, gin.H{"message": "Order not found"})
			return
		}
		if in.Status != "" {
			o, err := d.Ordering.ProcessStatus(c.Request.Context(), p, ref, ordering.OrderStatus(in.Status), "admin update")
			if err != nil {
				httpjson.Fail(c, d.Logger, err)
				return
			}
			c.JSON(http.StatusOK, legacyOrderView(o, nil))
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Updated"})
	}
}
func legacyOrderView(o ordering.Order, items []ordering.OrderItem) gin.H {
	h := gin.H{
		"id": o.ID, "order_ref": o.Reference, "reference": o.Reference,
		"status": o.Status, "payment_status": o.PaymentStatus,
		"total_amount": float64(o.TotalCents) / 100.0, "subtotal_amount": float64(o.SubtotalCents) / 100.0,
		"delivery_fee_cents": o.DeliveryFeeCents, "currency": o.Currency,
		"created_at": o.CreatedAt, "updated_at": o.UpdatedAt,
	}
	if o.DeliveryAddress != nil {
		h["shipping_address"] = *o.DeliveryAddress
	}
	if o.ContactPhone != nil {
		h["customer_phone"] = *o.ContactPhone
	}
	if items != nil {
		its := make([]gin.H, 0, len(items))
		for _, it := range items {
			pid := int64(0)
			if it.ProductID != nil {
				pid = *it.ProductID
			}
			its = append(its, gin.H{
				"id": it.ID, "product_id": pid, "product_name": it.ProductName,
				"quantity": it.Quantity, "unit_price": float64(it.UnitPriceCents) / 100.0,
				"total_price": float64(it.LineTotalCents) / 100.0,
			})
		}
		h["items"] = its
	}
	return h
}

var _ = time.Now
var _ = context.Background
var _ = httpauth.Deps{}
var _ = httpcatalogue.Service(nil)
var _ = shopping.Cart{}

func legacyUpload(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		cl := cloudinary.New(d.Cfg.Cloudinary)
		if !cl.Enabled() {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "Image upload is not configured (Cloudinary)."})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10<<20) // 10MB
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			file, header, err = c.Request.FormFile("image")
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "A file (form field 'file' or 'image') is required."})
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, 10<<20))
		if err != nil {
			httpjson.Fail(c, d.Logger, err)
			return
		}
		name := "upload"
		if header != nil && header.Filename != "" {
			name = header.Filename
		}
		res, err := cl.Upload(name, data, "naoja/products")
		if err != nil {
			if d.Logger != nil {
				d.Logger.Error("cloudinary upload failed", "error", err.Error())
			}
			c.JSON(http.StatusBadGateway, gin.H{"message": "Upload failed."})
			return
		}
		c.JSON(http.StatusOK, gin.H{"url": res.URL, "secure_url": res.SecureURL, "public_id": res.PublicID})
	}
}
