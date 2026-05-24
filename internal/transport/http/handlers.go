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
	r.Use(RateLimit(120, 1_000_000_000))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   h.config.AllowedOrigins(),
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	r.Get("/health", h.health)
	r.Get("/ws", h.hub.ServeHTTP)

	r.Route("/api", func(r chi.Router) {
		r.Post("/auth/login", h.login)
		r.Get("/public/menu", h.menu)
		r.Get("/public/tables/{token}", h.resolveTable)
		r.Post("/public/orders", h.placeOrder)

		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(h.config.JWTSecret))
			r.Get("/me", h.me)
			r.Get("/orders", h.orders)
			r.Patch("/orders/{id}/status", h.updateOrderStatus)
			r.Post("/payments", h.pay)
			r.Get("/tables", h.tables)
			r.Post("/tables", h.createTable)
			r.Get("/menu", h.adminMenu)
			r.Post("/menu/categories", h.createCategory)
			r.Post("/menu/items", h.createMenuItem)
			r.Patch("/menu/items/{id}", h.updateMenuItem)
			r.Get("/reports/summary", h.analytics)
			r.Get("/reports/sales.csv", h.salesCSV)
			r.Get("/users", h.users)
			r.Post("/users", h.createUser)
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
	user, token, err := h.service.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "token": token})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "authenticated",
		"user_id": claims.UserID,
		"email":   claims.Email,
		"role":    claims.Role,
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

func (h *Handler) resolveTable(w http.ResponseWriter, r *http.Request) {
	table, err := h.service.ResolveTable(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, table)
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

func (h *Handler) orders(w http.ResponseWriter, r *http.Request) {
	orders, err := h.service.Orders(r.Context(), queryInt(r, "limit", 50))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, orders)
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

func (h *Handler) pay(w http.ResponseWriter, r *http.Request) {
	var input struct {
		OrderID     int64  `json:"order_id"`
		Method      string `json:"method"`
		AmountCents int64  `json:"amount_cents"`
		Reference   string `json:"reference"`
	}
	if !decode(w, r, &input) {
		return
	}
	payment, err := h.service.Pay(r.Context(), input.OrderID, input.Method, input.AmountCents, input.Reference)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, payment)
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
	table, err := h.service.CreateTable(r.Context(), input.Number, input.Seats)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	table.QRURL = h.frontendBase(r) + "/order/" + table.QRToken
	writeJSON(w, http.StatusCreated, table)
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

func (h *Handler) analytics(w http.ResponseWriter, r *http.Request) {
	sales, best, lowStock, err := h.service.Analytics(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"daily_sales": sales, "best_sellers": best, "low_stock": lowStock})
}

func (h *Handler) salesCSV(w http.ResponseWriter, r *http.Request) {
	sales, _, _, err := h.service.Analytics(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=sales.csv")
	out := csv.NewWriter(w)
	_ = out.Write([]string{"day", "order_count", "revenue_cents"})
	for _, row := range sales {
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

func queryInt(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return fallback
	}
	return value
}

func frontendBase(r *http.Request) string {
	if origin := r.Header.Get("Origin"); origin != "" {
		return origin
	}
	return "http://" + r.Host
}

func (h *Handler) frontendBase(r *http.Request) string {
	if base := strings.TrimRight(h.config.FrontendURL, "/"); base != "" {
		return base
	}
	return strings.TrimRight(frontendBase(r), "/")
}

func (h *Handler) decodeMenuItem(w http.ResponseWriter, r *http.Request) (repository.SaveMenuItemInput, bool) {
	var input struct {
		CategoryID  int64  `json:"category_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		PriceCents  int64  `json:"price_cents"`
		ImageURL    string `json:"image_url"`
		Active      bool   `json:"active"`
	}
	if !decode(w, r, &input) {
		return repository.SaveMenuItemInput{}, false
	}
	return repository.SaveMenuItemInput{
		CategoryID:  input.CategoryID,
		Name:        input.Name,
		Description: input.Description,
		PriceCents:  input.PriceCents,
		ImageURL:    input.ImageURL,
		Active:      input.Active,
	}, true
}

var _ = domain.RoleAdmin
