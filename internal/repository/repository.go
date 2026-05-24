package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"restaurant/backend/internal/domain"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DB() *sql.DB { return r.db }

func (r *Repository) GetUserAuthByEmail(ctx context.Context, email string) (domain.User, string, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, name, email, password_hash, role, active, created_at FROM users WHERE email = ? LIMIT 1`, email)
	var user domain.User
	var hash string
	var role string
	var active int
	if err := row.Scan(&user.ID, &user.Name, &user.Email, &hash, &role, &active, &user.CreatedAt); err != nil {
		return user, "", err
	}
	user.Role = domain.Role(role)
	user.Active = active == 1
	return user, hash, nil
}

func (r *Repository) ListUsers(ctx context.Context) ([]domain.User, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, email, role, active, created_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []domain.User{}
	for rows.Next() {
		var user domain.User
		var role string
		var active int
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &role, &active, &user.CreatedAt); err != nil {
			return nil, err
		}
		user.Role = domain.Role(role)
		user.Active = active == 1
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *Repository) CreateUser(ctx context.Context, name, email, passwordHash, role string) (domain.User, error) {
	row := r.db.QueryRowContext(ctx, `INSERT INTO users (name, email, password_hash, role) VALUES (?, ?, ?, ?) RETURNING id, name, email, role, active, created_at`, name, email, passwordHash, role)
	var user domain.User
	var roleValue string
	var active int
	err := row.Scan(&user.ID, &user.Name, &user.Email, &roleValue, &active, &user.CreatedAt)
	user.Role = domain.Role(roleValue)
	user.Active = active == 1
	return user, err
}

func (r *Repository) ListTables(ctx context.Context) ([]domain.Table, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, number, seats, status, qr_token, created_at FROM restaurant_tables ORDER BY CAST(number AS INTEGER), number`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tables := []domain.Table{}
	for rows.Next() {
		var table domain.Table
		if err := rows.Scan(&table.ID, &table.Number, &table.Seats, &table.Status, &table.QRToken, &table.CreatedAt); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, rows.Err()
}

func (r *Repository) GetTableByToken(ctx context.Context, token string) (domain.Table, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, number, seats, status, qr_token, created_at FROM restaurant_tables WHERE qr_token = ? LIMIT 1`, token)
	var table domain.Table
	err := row.Scan(&table.ID, &table.Number, &table.Seats, &table.Status, &table.QRToken, &table.CreatedAt)
	return table, err
}

func (r *Repository) CreateTable(ctx context.Context, number string, seats int, token string) (domain.Table, error) {
	row := r.db.QueryRowContext(ctx, `INSERT INTO restaurant_tables (number, seats, qr_token) VALUES (?, ?, ?) RETURNING id, number, seats, status, qr_token, created_at`, number, seats, token)
	var table domain.Table
	err := row.Scan(&table.ID, &table.Number, &table.Seats, &table.Status, &table.QRToken, &table.CreatedAt)
	return table, err
}

func (r *Repository) UpdateTableStatus(ctx context.Context, id int64, status string) (domain.Table, error) {
	row := r.db.QueryRowContext(ctx, `UPDATE restaurant_tables SET status = ? WHERE id = ? RETURNING id, number, seats, status, qr_token, created_at`, status, id)
	var table domain.Table
	err := row.Scan(&table.ID, &table.Number, &table.Seats, &table.Status, &table.QRToken, &table.CreatedAt)
	return table, err
}

func (r *Repository) ListMenu(ctx context.Context) ([]domain.Category, []domain.MenuItem, error) {
	return r.listMenu(ctx, true)
}

func (r *Repository) ListMenuAdmin(ctx context.Context) ([]domain.Category, []domain.MenuItem, error) {
	return r.listMenu(ctx, false)
}

func (r *Repository) listMenu(ctx context.Context, activeOnly bool) ([]domain.Category, []domain.MenuItem, error) {
	categoryQuery := `SELECT id, name, sort_order, active FROM menu_categories`
	itemQuery := `SELECT id, category_id, name, description, price_cents, image_url, active, created_at FROM menu_items`
	if activeOnly {
		categoryQuery += ` WHERE active = 1`
		itemQuery += ` WHERE active = 1`
	}
	categoryQuery += ` ORDER BY sort_order, name`
	itemQuery += ` ORDER BY category_id, name`

	catRows, err := r.db.QueryContext(ctx, categoryQuery)
	if err != nil {
		return nil, nil, err
	}
	categories := []domain.Category{}
	for catRows.Next() {
		var category domain.Category
		var active int
		if err := catRows.Scan(&category.ID, &category.Name, &category.SortOrder, &active); err != nil {
			_ = catRows.Close()
			return nil, nil, err
		}
		category.Active = active == 1
		categories = append(categories, category)
	}
	if err := catRows.Err(); err != nil {
		_ = catRows.Close()
		return nil, nil, err
	}
	if err := catRows.Close(); err != nil {
		return nil, nil, err
	}

	itemRows, err := r.db.QueryContext(ctx, itemQuery)
	if err != nil {
		return nil, nil, err
	}
	defer itemRows.Close()
	items := []domain.MenuItem{}
	for itemRows.Next() {
		var item domain.MenuItem
		var active int
		if err := itemRows.Scan(&item.ID, &item.CategoryID, &item.Name, &item.Description, &item.PriceCents, &item.ImageURL, &active, &item.CreatedAt); err != nil {
			return nil, nil, err
		}
		item.Active = active == 1
		items = append(items, item)
	}
	return categories, items, itemRows.Err()
}

func (r *Repository) CreateCategory(ctx context.Context, name string, sortOrder int) (domain.Category, error) {
	row := r.db.QueryRowContext(ctx, `INSERT INTO menu_categories (name, sort_order) VALUES (?, ?) RETURNING id, name, sort_order, active`, strings.TrimSpace(name), sortOrder)
	var category domain.Category
	var active int
	err := row.Scan(&category.ID, &category.Name, &category.SortOrder, &active)
	category.Active = active == 1
	return category, err
}

func (r *Repository) GetMenuItem(ctx context.Context, id int64) (domain.MenuItem, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, category_id, name, description, price_cents, image_url, active, created_at FROM menu_items WHERE id = ?`, id)
	var item domain.MenuItem
	var active int
	err := row.Scan(&item.ID, &item.CategoryID, &item.Name, &item.Description, &item.PriceCents, &item.ImageURL, &active, &item.CreatedAt)
	item.Active = active == 1
	return item, err
}

type SaveMenuItemInput struct {
	CategoryID  int64
	Name        string
	Description string
	PriceCents  int64
	ImageURL    string
	Active      bool
}

func (r *Repository) CreateMenuItem(ctx context.Context, input SaveMenuItemInput) (domain.MenuItem, error) {
	row := r.db.QueryRowContext(ctx, `INSERT INTO menu_items (category_id, name, description, price_cents, image_url, active) VALUES (?, ?, ?, ?, ?, ?) RETURNING id, category_id, name, description, price_cents, image_url, active, created_at`,
		input.CategoryID,
		strings.TrimSpace(input.Name),
		strings.TrimSpace(input.Description),
		input.PriceCents,
		strings.TrimSpace(input.ImageURL),
		boolToInt(input.Active),
	)
	return scanMenuItem(row)
}

func (r *Repository) UpdateMenuItem(ctx context.Context, id int64, input SaveMenuItemInput) (domain.MenuItem, error) {
	row := r.db.QueryRowContext(ctx, `UPDATE menu_items SET category_id = ?, name = ?, description = ?, price_cents = ?, image_url = ?, active = ? WHERE id = ? RETURNING id, category_id, name, description, price_cents, image_url, active, created_at`,
		input.CategoryID,
		strings.TrimSpace(input.Name),
		strings.TrimSpace(input.Description),
		input.PriceCents,
		strings.TrimSpace(input.ImageURL),
		boolToInt(input.Active),
		id,
	)
	return scanMenuItem(row)
}

type CreateOrderInput struct {
	TableID      *int64                 `json:"table_id"`
	CustomerName string                 `json:"customer_name"`
	Source       string                 `json:"source"`
	Items        []CreateOrderItemInput `json:"items"`
}

type CreateOrderItemInput struct {
	MenuItemID int64  `json:"menu_item_id"`
	Quantity   int    `json:"quantity"`
	Notes      string `json:"notes"`
}

func (r *Repository) CreateOrder(ctx context.Context, input CreateOrderInput) (domain.Order, error) {
	if len(input.Items) == 0 {
		return domain.Order{}, errors.New("order must include at least one item")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Order{}, err
	}
	defer tx.Rollback()

	var subtotal int64
	type pricedItem struct {
		input CreateOrderItemInput
		menu  domain.MenuItem
	}
	priced := make([]pricedItem, 0, len(input.Items))
	for _, item := range input.Items {
		if item.Quantity <= 0 {
			return domain.Order{}, errors.New("item quantity must be greater than zero")
		}
		menu, err := r.getMenuItemTx(ctx, tx, item.MenuItemID)
		if err != nil {
			return domain.Order{}, fmt.Errorf("menu item %d: %w", item.MenuItemID, err)
		}
		subtotal += menu.PriceCents * int64(item.Quantity)
		priced = append(priced, pricedItem{input: item, menu: menu})
	}
	vat := subtotal * 16 / 116
	total := subtotal
	source := input.Source
	if source == "" {
		source = "qr"
	}
	row := tx.QueryRowContext(ctx, `INSERT INTO orders (table_id, customer_name, source, subtotal_cents, vat_cents, total_cents) VALUES (?, ?, ?, ?, ?, ?) RETURNING id, table_id, customer_name, status, subtotal_cents, vat_cents, total_cents, source, created_at, updated_at`, input.TableID, input.CustomerName, source, subtotal, vat, total)
	order, err := scanOrder(row)
	if err != nil {
		return domain.Order{}, err
	}
	for _, item := range priced {
		row := tx.QueryRowContext(ctx, `INSERT INTO order_items (order_id, menu_item_id, quantity, unit_price_cents, notes) VALUES (?, ?, ?, ?, ?) RETURNING id, order_id, menu_item_id, quantity, unit_price_cents, notes, status`, order.ID, item.menu.ID, item.input.Quantity, item.menu.PriceCents, item.input.Notes)
		orderItem, err := scanOrderItem(row)
		if err != nil {
			return domain.Order{}, err
		}
		orderItem.MenuItemName = item.menu.Name
		order.Items = append(order.Items, orderItem)
		if _, err := tx.ExecContext(ctx, `UPDATE ingredients SET stock_qty = stock_qty - (SELECT qty * ? FROM menu_item_ingredients WHERE menu_item_id = ? AND ingredient_id = ingredients.id) WHERE id IN (SELECT ingredient_id FROM menu_item_ingredients WHERE menu_item_id = ?)`, item.input.Quantity, item.menu.ID, item.menu.ID); err != nil {
			return domain.Order{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.Order{}, err
	}
	return order, nil
}

func (r *Repository) ListOrders(ctx context.Context, limit int) ([]domain.Order, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT o.id, o.table_id, COALESCE(t.number, ''), o.customer_name, o.status, o.subtotal_cents, o.vat_cents, o.total_cents, o.source, o.created_at, o.updated_at FROM orders o LEFT JOIN restaurant_tables t ON t.id = o.table_id ORDER BY o.created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	orders := []domain.Order{}
	for rows.Next() {
		order, err := scanOrderWithTable(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range orders {
		items, err := r.ListOrderItems(ctx, orders[i].ID)
		if err != nil {
			return nil, err
		}
		orders[i].Items = items
	}
	return orders, nil
}

func (r *Repository) ListOrderItems(ctx context.Context, orderID int64) ([]domain.OrderItem, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT oi.id, oi.order_id, oi.menu_item_id, mi.name, oi.quantity, oi.unit_price_cents, oi.notes, oi.status FROM order_items oi JOIN menu_items mi ON mi.id = oi.menu_item_id WHERE oi.order_id = ?`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.OrderItem{}
	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.MenuItemID, &item.MenuItemName, &item.Quantity, &item.UnitPriceCents, &item.Notes, &item.Status); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) UpdateOrderStatus(ctx context.Context, id int64, status string) (domain.Order, error) {
	row := r.db.QueryRowContext(ctx, `UPDATE orders SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? RETURNING id, table_id, customer_name, status, subtotal_cents, vat_cents, total_cents, source, created_at, updated_at`, status, id)
	order, err := scanOrder(row)
	if err != nil {
		return order, err
	}
	order.Items, err = r.ListOrderItems(ctx, order.ID)
	return order, err
}

func (r *Repository) CreatePayment(ctx context.Context, orderID int64, method string, amount int64, status string, reference string) (domain.Payment, error) {
	row := r.db.QueryRowContext(ctx, `INSERT INTO payments (order_id, method, amount_cents, status, reference) VALUES (?, ?, ?, ?, ?) RETURNING id, order_id, method, amount_cents, status, reference, created_at`, orderID, method, amount, status, reference)
	var payment domain.Payment
	err := row.Scan(&payment.ID, &payment.OrderID, &payment.Method, &payment.AmountCents, &payment.Status, &payment.Reference, &payment.CreatedAt)
	return payment, err
}

func (r *Repository) Analytics(ctx context.Context) ([]domain.SalesPoint, []domain.BestSeller, []domain.Ingredient, error) {
	salesRows, err := r.db.QueryContext(ctx, `SELECT date(created_at), COUNT(*), COALESCE(SUM(total_cents), 0) FROM orders WHERE status IN ('paid','served','ready') GROUP BY date(created_at) ORDER BY date(created_at) DESC LIMIT 30`)
	if err != nil {
		return nil, nil, nil, err
	}
	sales := []domain.SalesPoint{}
	for salesRows.Next() {
		var point domain.SalesPoint
		if err := salesRows.Scan(&point.Day, &point.OrderCount, &point.RevenueCents); err != nil {
			_ = salesRows.Close()
			return nil, nil, nil, err
		}
		sales = append(sales, point)
	}
	if err := salesRows.Err(); err != nil {
		_ = salesRows.Close()
		return nil, nil, nil, err
	}
	if err := salesRows.Close(); err != nil {
		return nil, nil, nil, err
	}
	bestRows, err := r.db.QueryContext(ctx, `SELECT mi.name, COALESCE(SUM(oi.quantity), 0), COALESCE(SUM(oi.quantity * oi.unit_price_cents), 0) FROM order_items oi JOIN menu_items mi ON mi.id = oi.menu_item_id GROUP BY mi.id, mi.name ORDER BY SUM(oi.quantity) DESC LIMIT 10`)
	if err != nil {
		return nil, nil, nil, err
	}
	best := []domain.BestSeller{}
	for bestRows.Next() {
		var item domain.BestSeller
		if err := bestRows.Scan(&item.Name, &item.Quantity, &item.RevenueCents); err != nil {
			_ = bestRows.Close()
			return nil, nil, nil, err
		}
		best = append(best, item)
	}
	if err := bestRows.Err(); err != nil {
		_ = bestRows.Close()
		return nil, nil, nil, err
	}
	if err := bestRows.Close(); err != nil {
		return nil, nil, nil, err
	}
	stockRows, err := r.db.QueryContext(ctx, `SELECT id, name, unit, stock_qty, low_stock_qty FROM ingredients WHERE stock_qty <= low_stock_qty ORDER BY stock_qty ASC`)
	if err != nil {
		return nil, nil, nil, err
	}
	defer stockRows.Close()
	lowStock := []domain.Ingredient{}
	for stockRows.Next() {
		var ingredient domain.Ingredient
		if err := stockRows.Scan(&ingredient.ID, &ingredient.Name, &ingredient.Unit, &ingredient.StockQty, &ingredient.LowStockQty); err != nil {
			return nil, nil, nil, err
		}
		lowStock = append(lowStock, ingredient)
	}
	return sales, best, lowStock, stockRows.Err()
}

func scanMenuItem(row interface{ Scan(dest ...any) error }) (domain.MenuItem, error) {
	var item domain.MenuItem
	var active int
	err := row.Scan(&item.ID, &item.CategoryID, &item.Name, &item.Description, &item.PriceCents, &item.ImageURL, &active, &item.CreatedAt)
	item.Active = active == 1
	return item, err
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (r *Repository) getMenuItemTx(ctx context.Context, tx *sql.Tx, id int64) (domain.MenuItem, error) {
	row := tx.QueryRowContext(ctx, `SELECT id, category_id, name, description, price_cents, image_url, active, created_at FROM menu_items WHERE id = ? AND active = 1`, id)
	var item domain.MenuItem
	var active int
	err := row.Scan(&item.ID, &item.CategoryID, &item.Name, &item.Description, &item.PriceCents, &item.ImageURL, &active, &item.CreatedAt)
	item.Active = active == 1
	return item, err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanOrder(row scanner) (domain.Order, error) {
	var order domain.Order
	var tableID sql.NullInt64
	err := row.Scan(&order.ID, &tableID, &order.CustomerName, &order.Status, &order.SubtotalCents, &order.VATCents, &order.TotalCents, &order.Source, &order.CreatedAt, &order.UpdatedAt)
	if tableID.Valid {
		order.TableID = &tableID.Int64
	}
	return order, err
}

func scanOrderWithTable(row scanner) (domain.Order, error) {
	var order domain.Order
	var tableID sql.NullInt64
	err := row.Scan(&order.ID, &tableID, &order.TableNumber, &order.CustomerName, &order.Status, &order.SubtotalCents, &order.VATCents, &order.TotalCents, &order.Source, &order.CreatedAt, &order.UpdatedAt)
	if tableID.Valid {
		order.TableID = &tableID.Int64
	}
	return order, err
}

func scanOrderItem(row scanner) (domain.OrderItem, error) {
	var item domain.OrderItem
	err := row.Scan(&item.ID, &item.OrderID, &item.MenuItemID, &item.Quantity, &item.UnitPriceCents, &item.Notes, &item.Status)
	return item, err
}

var _ = time.Time{}
