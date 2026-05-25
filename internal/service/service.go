package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"restaurant/backend/internal/auth"
	"restaurant/backend/internal/config"
	"restaurant/backend/internal/domain"
	"restaurant/backend/internal/realtime"
	"restaurant/backend/internal/repository"
)

type Service struct {
	repo   *repository.Repository
	hub    *realtime.Hub
	config config.Config
}
var baseURL string = "https://qrdinehotel.netlify.app"
func New(repo *repository.Repository, hub *realtime.Hub, cfg config.Config) *Service {
	return &Service{repo: repo, hub: hub, config: cfg}
}

func (s *Service) Login(ctx context.Context, email, password string) (domain.User, string, string, error) {
	user, hash, err := s.repo.GetUserAuthByEmail(ctx, email)
	if err != nil {
		return domain.User{}, "", "", errors.New("invalid credentials")
	}
	if !user.Active || !auth.CheckPassword(hash, password) {
		return domain.User{}, "", "", errors.New("invalid credentials")
	}
	accessToken, err := auth.IssueAccessToken(s.config.JWTSecret, user)
	if err != nil {
		return domain.User{}, "", "", err
	}
	refreshToken, err := auth.IssueRefreshToken(s.config.JWTSecret, user)
	if err != nil {
		return domain.User{}, "", "", err
	}
	return user, accessToken, refreshToken, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (domain.User, string, string, error) {
	claims, err := auth.ParseTokenOfType(s.config.JWTSecret, refreshToken, auth.TokenTypeRefresh)
	if err != nil {
		return domain.User{}, "", "", errors.New("invalid refresh token")
	}
	user, _, err := s.repo.GetUserAuthByEmail(ctx, claims.Email)
	if err != nil {
		return domain.User{}, "", "", errors.New("invalid refresh token")
	}
	if !user.Active {
		return domain.User{}, "", "", errors.New("user is inactive")
	}
	accessToken, err := auth.IssueAccessToken(s.config.JWTSecret, user)
	if err != nil {
		return domain.User{}, "", "", err
	}
	nextRefreshToken, err := auth.IssueRefreshToken(s.config.JWTSecret, user)
	if err != nil {
		return domain.User{}, "", "", err
	}
	return user, accessToken, nextRefreshToken, nil
}

func (s *Service) CreateStaff(ctx context.Context, name, email, password, role string) (domain.User, error) {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(strings.ToLower(email))
	role = strings.TrimSpace(strings.ToLower(role))
	if name == "" || email == "" || len(password) < 8 {
		return domain.User{}, errors.New("name, email, and password with 8+ characters are required")
	}
	if !isValidRole(role) {
		return domain.User{}, errors.New("invalid role")
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return domain.User{}, err
	}
	return s.repo.CreateUser(ctx, name, email, hash, role)
}

func (s *Service) ListUsers(ctx context.Context) ([]domain.User, error) {
	return s.repo.ListUsers(ctx)
}

func (s *Service) ListTables(ctx context.Context, baseURL string) ([]domain.Table, error) {
	tables, err := s.repo.ListTables(ctx)
	if err != nil {
		return nil, err
	}
	for i := range tables {
		if updated, changed := s.withTableURL(baseURL, tables[i]); changed {
			tables[i] = updated
			if _, err := s.repo.SaveQRCode(ctx, tables[i].ID, tables[i].QRURL, tables[i].QRImageData); err != nil {
				return nil, err
			}
		}
	}
	return tables, nil
}

func (s *Service) Table(ctx context.Context, id int64, baseURL string) (domain.Table, error) {
	table, err := s.repo.GetTableByID(ctx, id)
	if err != nil {
		return domain.Table{}, err
	}
	if updated, changed := s.withTableURL(baseURL, table); changed {
		table = updated
		return s.repo.SaveQRCode(ctx, table.ID, table.QRURL, table.QRImageData)
	}
	return table, nil
}

func (s *Service) CreateTable(ctx context.Context, number string, seats int, baseURL string) (domain.Table, error) {
	number = strings.TrimSpace(number)
	if number == "" || seats <= 0 {
		return domain.Table{}, errors.New("table number and seats are required")
	}

	table, err := s.repo.CreateTable(ctx, number, seats, slugifyTable(number), "qr-"+randomToken(8))
	if err != nil {
		return domain.Table{}, err
	}
	table, _ = s.withTableURL(baseURL, table)
	table, err = s.repo.SaveQRCode(ctx, table.ID, table.QRURL, table.QRImageData)
	if err != nil {
		return domain.Table{}, err
	}
	s.hub.Broadcast(realtime.Event{Type: "table.created", Data: table})
	return table, nil
}

func (s *Service) SaveTableQRCode(ctx context.Context, tableID int64, baseURL, imageData string) (domain.Table, error) {
	table, err := s.repo.GetTableByID(ctx, tableID)
	if err != nil {
		return domain.Table{}, err
	}
	table, _ = s.withTableURL(baseURL, table)
	table, err = s.repo.SaveQRCode(ctx, table.ID, table.QRURL, strings.TrimSpace(imageData))
	if err == nil {
		s.hub.Broadcast(realtime.Event{Type: "table.updated", Data: table})
	}
	return table, err
}

func (s *Service) ResolveTableContext(ctx context.Context, identifier, sessionToken, customerName, baseURL string) (domain.TableContext, error) {
	table, err := s.repo.GetTableByIdentifier(ctx, strings.TrimSpace(identifier))
	if err != nil {
		return domain.TableContext{}, err
	}

	if updated, changed := s.withTableURL(baseURL, table); changed {
		table = updated
		table, err = s.repo.SaveQRCode(ctx, table.ID, table.QRURL, table.QRImageData)
		if err != nil {
			return domain.TableContext{}, err
		}
	}

	var session domain.TableSession
	if strings.TrimSpace(sessionToken) != "" {
		current, err := s.repo.GetSessionByToken(ctx, strings.TrimSpace(sessionToken))
		if err == nil && current.TableID == table.ID && current.Status == "active" {
			session, err = s.repo.TouchSession(ctx, current.Token, customerName)
			if err != nil {
				return domain.TableContext{}, err
			}
		}
	}
	if session.ID == 0 {
		session, err = s.repo.CreateSession(ctx, repository.CreateSessionInput{
			TableID:      table.ID,
			Token:        "session-" + randomToken(10),
			CustomerName: strings.TrimSpace(customerName),
		})
		if err != nil {
			return domain.TableContext{}, err
		}
	}
	session.TableNumber = table.Number

	order, err := s.repo.GetLatestOrderForSession(ctx, session.ID)
	if err != nil {
		return domain.TableContext{}, err
	}
	settings, settingsErr := s.repo.GetSettings(ctx)
	if settingsErr != nil {
		settings = domain.Settings{
			BusinessName:  "MauzoHub",
			BusinessType:  "Business",
			CurrencyCode:  "KES",
			ReceiptFooter: "Thank you for your purchase.",
		}
	}

	return domain.TableContext{
		Table:       table,
		Session:     session,
		Business:    settings,
		ActiveOrder: order,
	}, nil
}

func (s *Service) SessionContext(ctx context.Context, sessionToken, baseURL string) (domain.TableContext, error) {
	session, err := s.repo.GetSessionByToken(ctx, strings.TrimSpace(sessionToken))
	if err != nil {
		return domain.TableContext{}, err
	}
	table, err := s.repo.GetTableByID(ctx, session.TableID)
	if err != nil {
		return domain.TableContext{}, err
	}
	if updated, changed := s.withTableURL(baseURL, table); changed {
		table = updated
		table, err = s.repo.SaveQRCode(ctx, table.ID, table.QRURL, table.QRImageData)
		if err != nil {
			return domain.TableContext{}, err
		}
	}
	session.TableNumber = table.Number
	order, err := s.repo.GetLatestOrderForSession(ctx, session.ID)
	if err != nil {
		return domain.TableContext{}, err
	}
	settings, settingsErr := s.repo.GetSettings(ctx)
	if settingsErr != nil {
		settings = domain.Settings{
			BusinessName:  "MauzoHub",
			BusinessType:  "Business",
			CurrencyCode:  "KES",
			ReceiptFooter: "Thank you for your purchase.",
		}
	}
	return domain.TableContext{
		Table:       table,
		Session:     session,
		Business:    settings,
		ActiveOrder: order,
	}, nil
}

func (s *Service) Menu(ctx context.Context) ([]domain.Category, []domain.MenuItem, error) {
	return s.repo.ListMenu(ctx)
}

func (s *Service) AdminMenu(ctx context.Context) ([]domain.Category, []domain.MenuItem, error) {
	return s.repo.ListMenuAdmin(ctx)
}

func (s *Service) CreateCategory(ctx context.Context, name string, sortOrder int) (domain.Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Category{}, errors.New("category name is required")
	}
	return s.repo.CreateCategory(ctx, name, sortOrder)
}

func (s *Service) CreateMenuItem(ctx context.Context, input repository.SaveMenuItemInput) (domain.MenuItem, error) {
	if input.CategoryID <= 0 || strings.TrimSpace(input.Name) == "" {
		return domain.MenuItem{}, errors.New("category and item name are required")
	}
	if input.PriceCents < 0 {
		return domain.MenuItem{}, errors.New("price must be zero or greater")
	}
	if input.CostCents < 0 {
		return domain.MenuItem{}, errors.New("cost must be zero or greater")
	}
	return s.repo.CreateMenuItem(ctx, input)
}

func (s *Service) UpdateMenuItem(ctx context.Context, id int64, input repository.SaveMenuItemInput) (domain.MenuItem, error) {
	if id <= 0 || input.CategoryID <= 0 || strings.TrimSpace(input.Name) == "" {
		return domain.MenuItem{}, errors.New("category and item name are required")
	}
	if input.PriceCents < 0 {
		return domain.MenuItem{}, errors.New("price must be zero or greater")
	}
	if input.CostCents < 0 {
		return domain.MenuItem{}, errors.New("cost must be zero or greater")
	}
	return s.repo.UpdateMenuItem(ctx, id, input)
}

func (s *Service) PlaceOrder(ctx context.Context, input repository.CreateOrderInput) (domain.Order, error) {
	if strings.TrimSpace(input.CustomerName) == "" {
		input.CustomerName = "Guest"
	}
	if strings.TrimSpace(input.Source) == "" {
		input.Source = "qr"
	}
	order, err := s.repo.CreateOrder(ctx, input)
	if err == nil {
		s.hub.Broadcast(realtime.Event{Type: "order.created", Data: order})
	}
	return order, err
}

func (s *Service) Orders(ctx context.Context, limit int) ([]domain.Order, error) {
	return s.repo.ListOrders(ctx, limit)
}

func (s *Service) Order(ctx context.Context, id int64) (domain.Order, error) {
	return s.repo.GetOrder(ctx, id)
}

func (s *Service) PublicOrder(ctx context.Context, id int64, sessionToken string) (domain.Order, error) {
	order, err := s.repo.GetOrder(ctx, id)
	if err != nil {
		return domain.Order{}, err
	}
	if strings.TrimSpace(sessionToken) == "" {
		return domain.Order{}, errors.New("session token is required")
	}
	session, err := s.repo.GetSessionByToken(ctx, sessionToken)
	if err != nil {
		return domain.Order{}, errors.New("invalid session token")
	}
	if order.SessionID == nil || *order.SessionID != session.ID {
		return domain.Order{}, errors.New("order does not belong to this table session")
	}
	return order, nil
}

func (s *Service) UpdateOrderStatus(ctx context.Context, id int64, status string) (domain.Order, error) {
	status = normalizeOrderStatus(status)
	if !validOrderStatus(status) {
		return domain.Order{}, errors.New("invalid order status")
	}
	order, err := s.repo.UpdateOrderStatus(ctx, id, status)
	if err != nil {
		return domain.Order{}, err
	}
	if status == "paid" {
		if err := s.finalizeSessionIfComplete(ctx, order); err != nil {
			return domain.Order{}, err
		}
		order, err = s.repo.GetOrder(ctx, id)
		if err != nil {
			return domain.Order{}, err
		}
	}
	s.hub.Broadcast(realtime.Event{Type: "order.updated", Data: order})
	return order, nil
}

func (s *Service) CreateCashPayment(ctx context.Context, orderID int64, amount int64, reference string, confirmNow bool) (domain.Order, domain.Payment, *domain.Receipt, error) {
	order, err := s.repo.GetOrder(ctx, orderID)
	if err != nil {
		return domain.Order{}, domain.Payment{}, nil, err
	}
	if amount <= 0 {
		amount = order.TotalCents
	}
	status := "pending"
	if confirmNow {
		status = "paid"
	}
	payment, err := s.repo.CreatePayment(ctx, repository.CreatePaymentInput{
		OrderID:      orderID,
		Method:       "cash",
		AmountCents:  amount,
		Status:       status,
		Reference:    strings.TrimSpace(reference),
		Provider:     "cash",
		MetadataJSON: `{"channel":"cash"}`,
	})
	if err != nil {
		return domain.Order{}, domain.Payment{}, nil, err
	}
	s.hub.Broadcast(realtime.Event{Type: "payment.updated", Data: payment})
	if !confirmNow {
		order, err = s.repo.GetOrder(ctx, orderID)
		return order, payment, order.Receipt, err
	}
	return s.confirmSettledPayment(ctx, payment)
}

func (s *Service) CreateMpesaPayment(ctx context.Context, orderID int64, phoneNumber string, amount int64) (domain.Order, domain.Payment, *domain.Receipt, error) {
	order, err := s.repo.GetOrder(ctx, orderID)
	if err != nil {
		return domain.Order{}, domain.Payment{}, nil, err
	}
	normalizedPhone, err := normalizeKenyanPhone(phoneNumber)
	if err != nil {
		return domain.Order{}, domain.Payment{}, nil, err
	}
	if amount <= 0 {
		amount = order.TotalCents
	}
	reference := "MPESA-" + strings.ToUpper(randomToken(5))
	metadata, _ := json.Marshal(map[string]any{
		"short_code":   s.config.MPesaShortCode,
		"callback_url": s.config.MPesaCallbackURL,
		"phone_number": normalizedPhone,
		"channel":      "stk_push",
	})
	status := "pending"
	if s.config.MPesaAutoApprove {
		status = "paid"
	}
	payment, err := s.repo.CreatePayment(ctx, repository.CreatePaymentInput{
		OrderID:      orderID,
		Method:       "mpesa",
		AmountCents:  amount,
		Status:       status,
		Reference:    reference,
		PhoneNumber:  normalizedPhone,
		Provider:     "mpesa",
		MetadataJSON: string(metadata),
	})
	if err != nil {
		return domain.Order{}, domain.Payment{}, nil, err
	}
	s.hub.Broadcast(realtime.Event{Type: "payment.updated", Data: payment})
	if !s.config.MPesaAutoApprove {
		order, err = s.repo.GetOrder(ctx, orderID)
		return order, payment, order.Receipt, err
	}
	return s.confirmSettledPayment(ctx, payment)
}

func (s *Service) ConfirmPayment(ctx context.Context, paymentID int64, reference string) (domain.Order, domain.Payment, *domain.Receipt, error) {
	payment, err := s.repo.ConfirmPayment(ctx, paymentID, reference)
	if err != nil {
		return domain.Order{}, domain.Payment{}, nil, err
	}
	return s.confirmSettledPayment(ctx, payment)
}

func (s *Service) Receipt(ctx context.Context, orderID int64) (*domain.Receipt, error) {
	return s.repo.GetReceiptByOrderID(ctx, orderID)
}

func (s *Service) Analytics(ctx context.Context) (domain.AnalyticsSnapshot, error) {
	return s.repo.Analytics(ctx)
}

func (s *Service) Products(ctx context.Context) ([]domain.Product, error) {
	return s.repo.ListProducts(ctx)
}

func (s *Service) Product(ctx context.Context, id int64) (domain.Product, error) {
	return s.repo.GetProduct(ctx, id)
}

func (s *Service) ArchiveProduct(ctx context.Context, id int64) error {
	if id <= 0 {
		return errors.New("invalid product id")
	}
	return s.repo.ArchiveProduct(ctx, id)
}

func (s *Service) AdjustInventory(ctx context.Context, input domain.InventoryAdjustment) (domain.Product, error) {
	if input.ProductID <= 0 {
		return domain.Product{}, errors.New("product is required")
	}
	if input.ChangeQty == 0 {
		return domain.Product{}, errors.New("stock change cannot be zero")
	}
	return s.repo.AdjustInventory(ctx, input)
}

func (s *Service) InventoryMovements(ctx context.Context, limit int) ([]domain.InventoryMovement, error) {
	return s.repo.ListInventoryMovements(ctx, limit)
}

func (s *Service) Settings(ctx context.Context) (domain.Settings, error) {
	return s.repo.GetSettings(ctx)
}

func (s *Service) SaveSettings(ctx context.Context, input domain.Settings) (domain.Settings, error) {
	input.BusinessName = strings.TrimSpace(input.BusinessName)
	if input.BusinessName == "" {
		return domain.Settings{}, errors.New("business name is required")
	}
	input.BusinessType = strings.TrimSpace(input.BusinessType)
	if input.BusinessType == "" {
		input.BusinessType = "Business"
	}
	input.Phone = strings.TrimSpace(input.Phone)
	input.MPesaTill = strings.TrimSpace(input.MPesaTill)
	input.CurrencyCode = strings.ToUpper(strings.TrimSpace(input.CurrencyCode))
	if input.CurrencyCode == "" {
		input.CurrencyCode = "KES"
	}
	input.ReceiptFooter = strings.TrimSpace(input.ReceiptFooter)
	if strings.TrimSpace(input.ReceiptFooter) == "" {
		input.ReceiptFooter = "Thank you for your purchase."
	}
	return s.repo.SaveSettings(ctx, input)
}

func (s *Service) DashboardSnapshot(ctx context.Context) (domain.DashboardSnapshot, error) {
	return s.repo.DashboardSnapshot(ctx)
}

func (s *Service) confirmSettledPayment(ctx context.Context, payment domain.Payment) (domain.Order, domain.Payment, *domain.Receipt, error) {
	order, err := s.repo.UpdateOrderPaymentStatus(ctx, payment.OrderID, "paid", true)
	if err != nil {
		return domain.Order{}, domain.Payment{}, nil, err
	}
	receipt, err := s.ensureReceipt(ctx, order, payment)
	if err != nil {
		return domain.Order{}, domain.Payment{}, nil, err
	}
	order.Receipt = receipt
	if err := s.finalizeSessionIfComplete(ctx, order); err != nil {
		return domain.Order{}, domain.Payment{}, nil, err
	}
	order, err = s.repo.GetOrder(ctx, order.ID)
	if err != nil {
		return domain.Order{}, domain.Payment{}, nil, err
	}
	s.hub.Broadcast(realtime.Event{Type: "payment.updated", Data: payment})
	s.hub.Broadcast(realtime.Event{Type: "order.updated", Data: order})
	if receipt != nil {
		s.hub.Broadcast(realtime.Event{Type: "receipt.created", Data: receipt})
	}
	return order, payment, receipt, nil
}

func (s *Service) ensureReceipt(ctx context.Context, order domain.Order, payment domain.Payment) (*domain.Receipt, error) {
	existing, err := s.repo.GetReceiptByOrderID(ctx, order.ID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	settings, settingsErr := s.repo.GetSettings(ctx)
	if settingsErr != nil {
		settings = domain.Settings{
			BusinessName:  "MauzoHub",
			ReceiptFooter: "Thank you for your purchase.",
		}
	}
	payload, err := json.Marshal(map[string]any{
		"order_id":       order.ID,
		"table_number":   order.TableNumber,
		"customer_name":  order.CustomerName,
		"status":         order.Status,
		"payment_status": order.PaymentStatus,
		"total_cents":    order.TotalCents,
		"vat_cents":      order.VATCents,
		"payment": map[string]any{
			"id":           payment.ID,
			"method":       payment.Method,
			"reference":    payment.Reference,
			"amount_cents": payment.AmountCents,
			"phone_number": payment.PhoneNumber,
			"provider":     payment.Provider,
			"confirmed_at": payment.ConfirmedAt,
		},
		"items":     order.Items,
		"issued_at": time.Now().UTC(),
		"business": map[string]any{
			"name":     settings.BusinessName,
			"footer":   settings.ReceiptFooter,
			"phone":    settings.Phone,
			"mpesa":    settings.MPesaTill,
			"currency": settings.CurrencyCode,
		},
	})
	if err != nil {
		return nil, err
	}
	receipt, err := s.repo.CreateReceipt(ctx, order.ID, &payment.ID, fmt.Sprintf("RCPT-%s-%04d", time.Now().Format("20060102"), order.ID), string(payload))
	if err != nil {
		return nil, err
	}
	return &receipt, nil
}

func (s *Service) finalizeSessionIfComplete(ctx context.Context, order domain.Order) error {
	if order.SessionID == nil || order.TableID == nil {
		return nil
	}
	hasOpenOrders, err := s.repo.SessionHasOpenOrders(ctx, *order.SessionID)
	if err != nil {
		return err
	}
	if hasOpenOrders {
		return nil
	}
	if err := s.repo.CloseSession(ctx, *order.SessionID); err != nil {
		return err
	}
	_, err = s.repo.UpdateTableStatus(ctx, *order.TableID, "available")
	return err
}

func (s *Service) withTableURL(baseURL string, table domain.Table) (domain.Table, bool) {
	slug := table.Slug
	if slug == "" {
		slug = table.QRToken
	}
	qrURL := strings.TrimRight(baseURL, "/") + "/order/" + slug
	changed := table.QRURL != qrURL
	table.Slug = slug
	table.QRURL = qrURL
	return table, changed
}

func normalizeOrderStatus(status string) string {
	status = strings.TrimSpace(strings.ToLower(status))
	if status == "pending" {
		return "new"
	}
	if status == "completed" {
		return "served"
	}
	return status
}

func validOrderStatus(status string) bool {
	switch status {
	case "new", "accepted", "preparing", "ready", "served", "paid", "cancelled":
		return true
	default:
		return false
	}
}

func isValidRole(role string) bool {
	switch domain.Role(role) {
	case domain.RoleAdmin, domain.RoleChef, domain.RoleWaiter, domain.RoleCashier, domain.RoleManager:
		return true
	default:
		return false
	}
}

func slugifyTable(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "table-" + randomToken(4)
	}
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				builder.WriteRune('-')
				lastDash = true
			}
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		slug = "table-" + randomToken(4)
	}
	if !strings.HasPrefix(slug, "table-") {
		slug = "table-" + slug
	}
	return slug
}

func randomToken(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "fallback"
	}
	return hex.EncodeToString(buf)
}

func normalizeKenyanPhone(value string) (string, error) {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, strings.TrimSpace(value))

	switch {
	case len(digits) == 9 && (digits[0] == '7' || digits[0] == '1'):
		return "+254" + digits, nil
	case len(digits) == 10 && digits[0] == '0' && (digits[1] == '7' || digits[1] == '1'):
		return "+254" + digits[1:], nil
	case len(digits) == 12 && strings.HasPrefix(digits, "254") && (digits[3] == '7' || digits[3] == '1'):
		return "+" + digits, nil
	default:
		return "", errors.New("enter a valid Kenyan M-Pesa phone number")
	}
}
