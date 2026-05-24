package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"

	"restaurant/backend/internal/auth"
	"restaurant/backend/internal/domain"
	"restaurant/backend/internal/realtime"
	"restaurant/backend/internal/repository"
)

type Service struct {
	repo      *repository.Repository
	hub       *realtime.Hub
	jwtSecret string
}

func New(repo *repository.Repository, hub *realtime.Hub, jwtSecret string) *Service {
	return &Service{repo: repo, hub: hub, jwtSecret: jwtSecret}
}

func (s *Service) Login(ctx context.Context, email, password string) (domain.User, string, error) {
	user, hash, err := s.repo.GetUserAuthByEmail(ctx, email)
	if err != nil {
		return domain.User{}, "", errors.New("invalid credentials")
	}
	if !user.Active || !auth.CheckPassword(hash, password) {
		return domain.User{}, "", errors.New("invalid credentials")
	}
	token, err := auth.IssueToken(s.jwtSecret, user)
	return user, token, err
}

func (s *Service) CreateStaff(ctx context.Context, name, email, password, role string) (domain.User, error) {
	if name == "" || email == "" || len(password) < 8 {
		return domain.User{}, errors.New("name, valid email, and password with 8+ characters are required")
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
	baseURL = strings.TrimRight(baseURL, "/")
	for i := range tables {
		tables[i].QRURL = baseURL + "/order/" + tables[i].QRToken
	}
	return tables, nil
}

func (s *Service) CreateTable(ctx context.Context, number string, seats int) (domain.Table, error) {
	number = strings.TrimSpace(number)
	if number == "" || seats <= 0 {
		return domain.Table{}, errors.New("table number and seats are required")
	}
	table, err := s.repo.CreateTable(ctx, number, seats, "table-"+randomToken(8))
	if err == nil {
		s.hub.Broadcast(realtime.Event{Type: "table.created", Data: table})
	}
	return table, err
}

func (s *Service) ResolveTable(ctx context.Context, token string) (domain.Table, error) {
	return s.repo.GetTableByToken(ctx, token)
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
	return s.repo.CreateMenuItem(ctx, input)
}

func (s *Service) UpdateMenuItem(ctx context.Context, id int64, input repository.SaveMenuItemInput) (domain.MenuItem, error) {
	if id <= 0 || input.CategoryID <= 0 || strings.TrimSpace(input.Name) == "" {
		return domain.MenuItem{}, errors.New("category and item name are required")
	}
	if input.PriceCents < 0 {
		return domain.MenuItem{}, errors.New("price must be zero or greater")
	}
	return s.repo.UpdateMenuItem(ctx, id, input)
}

func (s *Service) PlaceOrder(ctx context.Context, input repository.CreateOrderInput) (domain.Order, error) {
	order, err := s.repo.CreateOrder(ctx, input)
	if err == nil {
		s.hub.Broadcast(realtime.Event{Type: "order.created", Data: order})
	}
	return order, err
}

func (s *Service) Orders(ctx context.Context, limit int) ([]domain.Order, error) {
	return s.repo.ListOrders(ctx, limit)
}

func (s *Service) UpdateOrderStatus(ctx context.Context, id int64, status string) (domain.Order, error) {
	order, err := s.repo.UpdateOrderStatus(ctx, id, status)
	if err == nil {
		s.hub.Broadcast(realtime.Event{Type: "order.updated", Data: order})
	}
	return order, err
}

func (s *Service) Pay(ctx context.Context, orderID int64, method string, amount int64, reference string) (domain.Payment, error) {
	status := "paid"
	if method == "mpesa" && reference == "" {
		status = "pending"
		reference = "mpesa-stk-placeholder"
	}
	payment, err := s.repo.CreatePayment(ctx, orderID, method, amount, status, reference)
	if err == nil {
		s.hub.Broadcast(realtime.Event{Type: "payment.updated", Data: payment})
	}
	return payment, err
}

func (s *Service) Analytics(ctx context.Context) ([]domain.SalesPoint, []domain.BestSeller, []domain.Ingredient, error) {
	return s.repo.Analytics(ctx)
}

func randomToken(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "fallback"
	}
	return hex.EncodeToString(buf)
}
