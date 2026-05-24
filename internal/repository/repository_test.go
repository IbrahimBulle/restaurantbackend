package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	internaldb "restaurant/backend/internal/database"
)

func TestListMenuReleasesSingleSQLiteConnection(t *testing.T) {
	repo := newTestRepository(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	categories, items, err := repo.ListMenu(ctx)
	if err != nil {
		t.Fatalf("ListMenu returned error: %v", err)
	}
	if len(categories) == 0 {
		t.Fatal("expected seeded categories")
	}
	if len(items) == 0 {
		t.Fatal("expected seeded menu items")
	}
}

func TestListMethodsReturnEmptySlicesWhenNoRows(t *testing.T) {
	repo := newTestRepository(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	orders, err := repo.ListOrders(ctx, 10)
	if err != nil {
		t.Fatalf("ListOrders returned error: %v", err)
	}
	if orders == nil {
		t.Fatal("expected empty orders slice, got nil")
	}
	if len(orders) != 0 {
		t.Fatalf("expected no orders, got %d", len(orders))
	}

	items, err := repo.ListOrderItems(ctx, 99999)
	if err != nil {
		t.Fatalf("ListOrderItems returned error: %v", err)
	}
	if items == nil {
		t.Fatal("expected empty order items slice, got nil")
	}
	if len(items) != 0 {
		t.Fatalf("expected no order items, got %d", len(items))
	}

	sales, best, lowStock, err := repo.Analytics(ctx)
	if err != nil {
		t.Fatalf("Analytics returned error: %v", err)
	}
	if sales == nil || best == nil || lowStock == nil {
		t.Fatal("expected analytics slices to be non-nil")
	}
	if len(sales) != 0 || len(best) != 0 || len(lowStock) != 0 {
		t.Fatalf("expected empty analytics slices, got sales=%d best=%d lowStock=%d", len(sales), len(best), len(lowStock))
	}
}

func TestListOrdersReleasesSingleSQLiteConnection(t *testing.T) {
	repo := newTestRepository(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	tableID := int64(1)
	if _, err := repo.CreateOrder(ctx, CreateOrderInput{
		TableID:      &tableID,
		CustomerName: "Regression Test",
		Items: []CreateOrderItemInput{
			{MenuItemID: 1, Quantity: 2},
		},
	}); err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}

	orders, err := repo.ListOrders(ctx, 10)
	if err != nil {
		t.Fatalf("ListOrders returned error: %v", err)
	}
	if len(orders) == 0 {
		t.Fatal("expected at least one order")
	}
	if len(orders[0].Items) == 0 {
		t.Fatal("expected order items to be loaded")
	}
}

func TestAnalyticsReleasesSingleSQLiteConnection(t *testing.T) {
	repo := newTestRepository(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	tableID := int64(1)
	order, err := repo.CreateOrder(ctx, CreateOrderInput{
		TableID:      &tableID,
		CustomerName: "Analytics Test",
		Items: []CreateOrderItemInput{
			{MenuItemID: 2, Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if _, err := repo.UpdateOrderStatus(ctx, order.ID, "paid"); err != nil {
		t.Fatalf("UpdateOrderStatus returned error: %v", err)
	}

	sales, best, _, err := repo.Analytics(ctx)
	if err != nil {
		t.Fatalf("Analytics returned error: %v", err)
	}
	if len(sales) == 0 {
		t.Fatal("expected at least one sales data point")
	}
	if len(best) == 0 {
		t.Fatal("expected at least one best seller")
	}
}

func newTestRepository(t *testing.T) *Repository {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "restaurant-test.db")
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_busy_timeout=5000", dbPath)

	db, err := internaldb.Open(dsn)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("SetDialect returned error: %v", err)
	}
	if err := goose.Up(db, migrationsDir(t)); err != nil {
		t.Fatalf("goose.Up returned error: %v", err)
	}

	return New(db)
}

func migrationsDir(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "db", "migrations")
}
