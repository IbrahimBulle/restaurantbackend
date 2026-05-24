package httptransport

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"restaurant/backend/internal/auth"
	"restaurant/backend/internal/config"
	"restaurant/backend/internal/domain"
	"restaurant/backend/internal/realtime"
	"restaurant/backend/internal/repository"
	"restaurant/backend/internal/service"
)

type Handler struct {
	service *service.Service
	hub     *realtime.Hub
	config  config.Config
}

func New(service *service.Service, hub *realtime.Hub, cfg config.Config) *Handler {
	return &Handler{service: service, hub: hub, config: cfg}
}

func (h *Handler) Routes(logMiddleware func(http.Handler) http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
	r.Use(logMiddleware)
	r.Use(RateLimit(180, 1_000_000_000))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: h.config.AllowedOrigins(),
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
	}))

	r.Get("/health", h.health)
	r.Get("/ws", h.hub.ServeHTTP)

	r.Route("/api", func(r chi.Router) {
		r.Post("/auth/login", h.login)
		r.Post("/auth/refresh", h.refresh)
		r.Post("/auth/logout", h.logout)

		r.Get("/public/menu", h.menu)
		r.Get("/public/tables/{identifier}", h.resolveTable)
		r.Get("/public/sessions/{token}", h.sessionContext)
		r.Post("/public/orders", h.placeOrder)
		r.Get("/public/orders/{id}", h.publicOrder)
		r.Post("/public/orders/{id}/payments/cash", h.publicCashPayment)
		r.Post("/public/orders/{id}/payments/mpesa", h.publicMpesaPayment)

		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(h.config.JWTSecret))

			r.Get("/me", h.me)
			r.Get("/dashboard", h.dashboard)
			r.Get("/settings", h.settings)
			r.Put("/settings", h.saveSettings)
			r.Get("/products", h.products)
			r.Post("/products", h.createProduct)
			r.Put("/products/{id}", h.updateProduct)
			r.Delete("/products/{id}", h.deleteProduct)
			r.Get("/categories", h.categories)
			r.Post("/categories", h.createCategory)
			r.Get("/inventory", h.inventory)
			r.Get("/inventory/movements", h.inventoryMovements)
			r.Post("/inventory/adjust", h.adjustInventory)
			r.Get("/orders", h.orders)
			r.Get("/orders/{id}", h.order)
			r.Patch("/orders/{id}/status", h.updateOrderStatus)
			r.Get("/tables", h.tables)
			r.Get("/tables/{id}/qrcode", h.tableQRCode)
			r.Post("/tables", h.createTable)
			r.Put("/tables/{id}/qrcode", h.saveTableQRCode)
			r.Get("/menu", h.adminMenu)
			r.Post("/menu", h.createMenuItem)
			r.Post("/menu/categories", h.createCategory)
			r.Post("/menu/items", h.createMenuItem)
			r.Patch("/menu/items/{id}", h.updateMenuItem)
			r.Get("/analytics", h.analytics)
			r.Get("/reports/summary", h.analytics)
			r.Get("/reports/sales.csv", h.salesCSV)
			r.Get("/users", h.users)
			r.Post("/users", h.createUser)
			r.Post("/payments/cash", h.cashPayment)
			r.Post("/payments/mpesa", h.mpesaPayment)
			r.Post("/payments/{id}/confirm", h.confirmPayment)
			r.Post("/payments/mpesa/callback", h.mpesaCallback)
			r.Get("/receipts/{orderID}", h.receipt)
		})
	})

	return r
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}
	user, accessToken, refreshToken, err := h.service.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user":          user,
		"token":         accessToken,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decode(w, r, &input) {
		return
	}
	user, accessToken, refreshToken, err := h.service.Refresh(r.Context(), input.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user":          user,
		"token":         accessToken,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}
	name := strings.TrimSpace(claims.Name)
	if name == "" {
		if local, _, found := strings.Cut(claims.Email, "@"); found && strings.TrimSpace(local) != "" {
			name = local
		} else {
			name = claims.Email
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":     claims.UserID,
		"name":   name,
		"email":  claims.Email,
		"role":   claims.Role,
		"status": "authenticated",
	})
}

func (h *Handler) menu(w http.ResponseWriter, r *http.Request) {
	categories, items, err := h.service.Menu(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": categories, "items": items})
}

func (h *Handler) adminMenu(w http.ResponseWriter, r *http.Request) {
	categories, items, err := h.service.AdminMenu(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": categories, "items": items})
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.service.DashboardSnapshot(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (h *Handler) settings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.Settings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (h *Handler) saveSettings(w http.ResponseWriter, r *http.Request) {
	var input domain.Settings
	if !decode(w, r, &input) {
		return
	}
	settings, err := h.service.SaveSettings(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (h *Handler) categories(w http.ResponseWriter, r *http.Request) {
	categories, _, err := h.service.AdminMenu(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, categories)
}

func (h *Handler) products(w http.ResponseWriter, r *http.Request) {
	products, err := h.service.Products(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, products)
}

func (h *Handler) createProduct(w http.ResponseWriter, r *http.Request) {
	item, ok := h.decodeMenuItem(w, r)
	if !ok {
		return
	}
	created, err := h.service.CreateMenuItem(r.Context(), item)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	product, err := h.service.Product(r.Context(), created.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, product)
}

func (h *Handler) updateProduct(w http.ResponseWriter, r *http.Request) {
	item, ok := h.decodeMenuItem(w, r)
	if !ok {
		return
	}
	updated, err := h.service.UpdateMenuItem(r.Context(), pathID(r), item)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	product, err := h.service.Product(r.Context(), updated.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, product)
}

func (h *Handler) deleteProduct(w http.ResponseWriter, r *http.Request) {
	if err := h.service.ArchiveProduct(r.Context(), pathID(r)); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) inventory(w http.ResponseWriter, r *http.Request) {
	products, err := h.service.Products(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	filtered := make([]domain.Product, 0, len(products))
	for _, product := range products {
		if product.TrackStock {
			filtered = append(filtered, product)
		}
	}
	writeJSON(w, http.StatusOK, filtered)
}

func (h *Handler) inventoryMovements(w http.ResponseWriter, r *http.Request) {
	movements, err := h.service.InventoryMovements(r.Context(), queryInt(r, "limit", 40))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, movements)
}

func (h *Handler) adjustInventory(w http.ResponseWriter, r *http.Request) {
	var input domain.InventoryAdjustment
	if !decode(w, r, &input) {
		return
	}
	product, err := h.service.AdjustInventory(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, product)
}

func (h *Handler) resolveTable(w http.ResponseWriter, r *http.Request) {
	context, err := h.service.ResolveTableContext(
		r.Context(),
		chi.URLParam(r, "identifier"),
		r.URL.Query().Get("session_token"),
		r.URL.Query().Get("customer_name"),
		h.frontendBase(r),
	)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, context)
}

func (h *Handler) sessionContext(w http.ResponseWriter, r *http.Request) {
	context, err := h.service.SessionContext(r.Context(), chi.URLParam(r, "token"), h.frontendBase(r))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, context)
}

func (h *Handler) placeOrder(w http.ResponseWriter, r *http.Request) {
	var input repository.CreateOrderInput
	if !decode(w, r, &input) {
		return
	}
	order, err := h.service.PlaceOrder(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, order)
}

func (h *Handler) publicOrder(w http.ResponseWriter, r *http.Request) {
	order, err := h.service.PublicOrder(r.Context(), pathID(r), queryStr(r, "session_token", ""))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *Handler) orders(w http.ResponseWriter, r *http.Request) {
	orders, err := h.service.Orders(r.Context(), queryInt(r, "limit", 100))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

func (h *Handler) order(w http.ResponseWriter, r *http.Request) {
	order, err := h.service.Order(r.Context(), pathID(r))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *Handler) updateOrderStatus(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &input) {
		return
	}
	order, err := h.service.UpdateOrderStatus(r.Context(), pathID(r), input.Status)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *Handler) tables(w http.ResponseWriter, r *http.Request) {
	tables, err := h.service.ListTables(r.Context(), h.frontendBase(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, tables)
}

func (h *Handler) createTable(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Number string `json:"number"`
		Seats  int    `json:"seats"`
	}
	if !decode(w, r, &input) {
		return
	}
	table, err := h.service.CreateTable(r.Context(), input.Number, input.Seats, h.frontendBase(r))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, table)
}

func (h *Handler) tableQRCode(w http.ResponseWriter, r *http.Request) {
	table, err := h.service.Table(r.Context(), pathID(r), h.frontendBase(r))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"table_id":      table.ID,
		"slug":          table.Slug,
		"qr_url":        table.QRURL,
		"qr_image_data": table.QRImageData,
		"table_number":  table.Number,
		"table_status":  table.Status,
		"table_seats":   table.Seats,
	})
}

func (h *Handler) saveTableQRCode(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ImageData string `json:"image_data"`
	}
	if !decode(w, r, &input) {
		return
	}
	table, err := h.service.SaveTableQRCode(r.Context(), pathID(r), h.frontendBase(r), input.ImageData)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, table)
}

func (h *Handler) createCategory(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name      string `json:"name"`
		SortOrder int    `json:"sort_order"`
	}
	if !decode(w, r, &input) {
		return
	}
	category, err := h.service.CreateCategory(r.Context(), input.Name, input.SortOrder)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, category)
}

func (h *Handler) createMenuItem(w http.ResponseWriter, r *http.Request) {
	item, ok := h.decodeMenuItem(w, r)
	if !ok {
		return
	}
	created, err := h.service.CreateMenuItem(r.Context(), item)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) updateMenuItem(w http.ResponseWriter, r *http.Request) {
	item, ok := h.decodeMenuItem(w, r)
	if !ok {
		return
	}
	updated, err := h.service.UpdateMenuItem(r.Context(), pathID(r), item)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) publicCashPayment(w http.ResponseWriter, r *http.Request) {
	var input struct {
		AmountCents int64  `json:"amount_cents"`
		Reference   string `json:"reference"`
	}
	if !decode(w, r, &input) {
		return
	}
	order, err := h.service.PublicOrder(r.Context(), pathID(r), queryStr(r, "session_token", ""))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	order, payment, receipt, err := h.service.CreateCashPayment(r.Context(), order.ID, input.AmountCents, input.Reference, false)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"order": order, "payment": payment, "receipt": receipt})
}

func (h *Handler) publicMpesaPayment(w http.ResponseWriter, r *http.Request) {
	var input struct {
		AmountCents int64  `json:"amount_cents"`
		PhoneNumber string `json:"phone_number"`
	}
	if !decode(w, r, &input) {
		return
	}
	order, err := h.service.PublicOrder(r.Context(), pathID(r), queryStr(r, "session_token", ""))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	order, payment, receipt, err := h.service.CreateMpesaPayment(r.Context(), order.ID, input.PhoneNumber, input.AmountCents)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"order": order, "payment": payment, "receipt": receipt})
}

func (h *Handler) cashPayment(w http.ResponseWriter, r *http.Request) {
	var input struct {
		OrderID     int64  `json:"order_id"`
		AmountCents int64  `json:"amount_cents"`
		Reference   string `json:"reference"`
	}
	if !decode(w, r, &input) {
		return
	}
	order, payment, receipt, err := h.service.CreateCashPayment(r.Context(), input.OrderID, input.AmountCents, input.Reference, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"order": order, "payment": payment, "receipt": receipt})
}

func (h *Handler) mpesaPayment(w http.ResponseWriter, r *http.Request) {
	var input struct {
		OrderID     int64  `json:"order_id"`
		AmountCents int64  `json:"amount_cents"`
		PhoneNumber string `json:"phone_number"`
	}
	if !decode(w, r, &input) {
		return
	}
	order, payment, receipt, err := h.service.CreateMpesaPayment(r.Context(), input.OrderID, input.PhoneNumber, input.AmountCents)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"order": order, "payment": payment, "receipt": receipt})
}

func (h *Handler) mpesaCallback(w http.ResponseWriter, r *http.Request) {
	var input struct {
		PaymentID   int64  `json:"payment_id"`
		Reference   string `json:"reference"`
		Status      string `json:"status"`
		Phone       string `json:"phone_number"`
		AmountCents int64  `json:"amount_cents"`
	}
	if !decode(w, r, &input) {
		return
	}
	if strings.ToLower(strings.TrimSpace(input.Status)) != "paid" {
		writeJSON(w, http.StatusOK, map[string]any{
			"received": true,
			"status":   "ignored",
		})
		return
	}
	order, payment, receipt, err := h.service.ConfirmPayment(r.Context(), input.PaymentID, input.Reference)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"order": order, "payment": payment, "receipt": receipt, "received": true})
}

func (h *Handler) confirmPayment(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Reference string `json:"reference"`
	}
	if !decode(w, r, &input) {
		return
	}
	order, payment, receipt, err := h.service.ConfirmPayment(r.Context(), pathID(r), input.Reference)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"order": order, "payment": payment, "receipt": receipt})
}

func (h *Handler) receipt(w http.ResponseWriter, r *http.Request) {
	receipt, err := h.service.Receipt(r.Context(), pathOrderID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if receipt == nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("receipt not found"))
		return
	}
	writeJSON(w, http.StatusOK, receipt)
}

func (h *Handler) analytics(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.service.Analytics(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (h *Handler) salesCSV(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.service.Analytics(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=sales.csv")
	out := csv.NewWriter(w)
	_ = out.Write([]string{"day", "order_count", "revenue_cents"})
	for _, row := range snapshot.DailySales {
		_ = out.Write([]string{row.Day, fmt.Sprint(row.OrderCount), fmt.Sprint(row.RevenueCents)})
	}
	out.Flush()
}

func (h *Handler) users(w http.ResponseWriter, r *http.Request) {
	users, err := h.service.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if !decode(w, r, &input) {
		return
	}
	user, err := h.service.CreateStaff(r.Context(), input.Name, input.Email, input.Password, input.Role)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return false
	}
	return true
}

func pathID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return id
}

func pathOrderID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, "orderID"), 10, 64)
	return id
}

func queryInt(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return fallback
	}
	return value
}

func queryStr(r *http.Request, key string, fallback string) string {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	return value
}

func frontendBase(r *http.Request) string {
	if origin := r.Header.Get("Origin"); origin != "" {
		return strings.TrimRight(origin, "/")
	}
	return "http://" + strings.TrimRight(r.Host, "/")
}

func (h *Handler) frontendBase(r *http.Request) string {
	if base := strings.TrimRight(h.config.FrontendURL, "/"); base != "" {
		return base
	}
	return frontendBase(r)
}

func (h *Handler) decodeMenuItem(w http.ResponseWriter, r *http.Request) (repository.SaveMenuItemInput, bool) {
	var input struct {
		CategoryID   int64   `json:"category_id"`
		Name         string  `json:"name"`
		Description  string  `json:"description"`
		PriceCents   int64   `json:"price_cents"`
		ImageURL     string  `json:"image_url"`
		SKU          string  `json:"sku"`
		ItemType     string  `json:"item_type"`
		CostCents    int64   `json:"cost_cents"`
		SortOrder    int     `json:"sort_order"`
		StockQty     float64 `json:"stock_qty"`
		ReorderLevel float64 `json:"reorder_level"`
		Unit         string  `json:"unit"`
		TrackStock   *bool   `json:"track_stock"`
		Active       bool    `json:"active"`
	}
	if !decode(w, r, &input) {
		return repository.SaveMenuItemInput{}, false
	}
	trackStock := true
	if input.TrackStock != nil {
		trackStock = *input.TrackStock
	}
	return repository.SaveMenuItemInput{
		CategoryID:   input.CategoryID,
		Name:         input.Name,
		Description:  input.Description,
		PriceCents:   input.PriceCents,
		ImageURL:     input.ImageURL,
		SKU:          input.SKU,
		ItemType:     input.ItemType,
		CostCents:    input.CostCents,
		SortOrder:    input.SortOrder,
		StockQty:     input.StockQty,
		ReorderLevel: input.ReorderLevel,
		Unit:         input.Unit,
		TrackStock:   trackStock,
		Active:       input.Active,
	}, true
}
