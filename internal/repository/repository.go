package repository

import (
	"context"
	"database/sql"
	"encoding/json"
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
	var active int
	var storedRole string
	if err := row.Scan(&user.ID, &user.Name, &user.Email, &storedRole, &active, &user.CreatedAt); err != nil {
		return user, err
	}
	user.Role = domain.Role(storedRole)
	user.Active = active == 1
	return user, nil
}

func (r *Repository) ListTables(ctx context.Context) ([]domain.Table, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			t.id,
			t.number,
			t.seats,
			t.status,
			COALESCE(t.slug, ''),
			t.qr_token,
			COALESCE(q.public_url, ''),
			COALESCE(q.image_data, ''),
			COALESCE(t.active, 1),
			t.created_at
		FROM restaurant_tables t
		LEFT JOIN qr_codes q ON q.table_id = t.id
		WHERE COALESCE(t.active, 1) = 1
		ORDER BY CAST(t.number AS INTEGER), t.number
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := []domain.Table{}
	for rows.Next() {
		table, err := scanTable(rows)
		if err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, rows.Err()
}

func (r *Repository) GetTableByID(ctx context.Context, id int64) (domain.Table, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			t.id,
			t.number,
			t.seats,
			t.status,
			COALESCE(t.slug, ''),
			t.qr_token,
			COALESCE(q.public_url, ''),
			COALESCE(q.image_data, ''),
			COALESCE(t.active, 1),
			t.created_at
		FROM restaurant_tables t
		LEFT JOIN qr_codes q ON q.table_id = t.id
		WHERE t.id = ?
		LIMIT 1
	`, id)
	return scanTable(row)
}

func (r *Repository) GetTableByIdentifier(ctx context.Context, identifier string) (domain.Table, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			t.id,
			t.number,
			t.seats,
			t.status,
			COALESCE(t.slug, ''),
			t.qr_token,
			COALESCE(q.public_url, ''),
			COALESCE(q.image_data, ''),
			COALESCE(t.active, 1),
			t.created_at
		FROM restaurant_tables t
		LEFT JOIN qr_codes q ON q.table_id = t.id
		WHERE t.slug = ? OR t.qr_token = ?
		LIMIT 1
	`, identifier, identifier)
	return scanTable(row)
}

func (r *Repository) CreateTable(ctx context.Context, number string, seats int, slug string, token string) (domain.Table, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO restaurant_tables (number, seats, qr_token, slug)
		VALUES (?, ?, ?, ?)
		RETURNING id, number, seats, status, COALESCE(slug, ''), qr_token, '', '', COALESCE(active, 1), created_at
	`, number, seats, token, slug)
	return scanTable(row)
}

func (r *Repository) UpdateTableStatus(ctx context.Context, id int64, status string) (domain.Table, error) {
	if _, err := r.db.ExecContext(ctx, `UPDATE restaurant_tables SET status = ? WHERE id = ?`, status, id); err != nil {
		return domain.Table{}, err
	}
	return r.GetTableByID(ctx, id)
}

func (r *Repository) SaveQRCode(ctx context.Context, tableID int64, publicURL string, imageData string) (domain.Table, error) {
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO qr_codes (table_id, public_url, image_data)
		VALUES (?, ?, ?)
		ON CONFLICT(table_id) DO UPDATE SET
			public_url = excluded.public_url,
			image_data = CASE
				WHEN excluded.image_data = '' THEN qr_codes.image_data
				ELSE excluded.image_data
			END,
			updated_at = CURRENT_TIMESTAMP
	`, tableID, publicURL, imageData); err != nil {
		return domain.Table{}, err
	}
	return r.GetTableByID(ctx, tableID)
}

type CreateSessionInput struct {
	TableID      int64
	Token        string
	CustomerName string
}

func (r *Repository) CreateSession(ctx context.Context, input CreateSessionInput) (domain.TableSession, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO sessions (table_id, token, customer_name)
		VALUES (?, ?, ?)
		RETURNING id, table_id, '', token, customer_name, status, started_at, last_seen_at, ended_at
	`, input.TableID, input.Token, strings.TrimSpace(input.CustomerName))
	return scanSession(row)
}

func (r *Repository) TouchSession(ctx context.Context, token string, customerName string) (domain.TableSession, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE sessions
		SET
			customer_name = CASE WHEN ? = '' THEN customer_name ELSE ? END,
			last_seen_at = CURRENT_TIMESTAMP
		WHERE token = ?
		RETURNING id, table_id, '', token, customer_name, status, started_at, last_seen_at, ended_at
	`, strings.TrimSpace(customerName), strings.TrimSpace(customerName), token)
	return scanSession(row)
}

func (r *Repository) GetSessionByToken(ctx context.Context, token string) (domain.TableSession, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			s.id,
			s.table_id,
			COALESCE(t.number, ''),
			s.token,
			s.customer_name,
			s.status,
			s.started_at,
			s.last_seen_at,
			s.ended_at
		FROM sessions s
		JOIN restaurant_tables t ON t.id = s.table_id
		WHERE s.token = ?
		LIMIT 1
	`, token)
	return scanSession(row)
}

func (r *Repository) CloseSession(ctx context.Context, sessionID int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sessions SET status = 'closed', ended_at = CURRENT_TIMESTAMP, last_seen_at = CURRENT_TIMESTAMP WHERE id = ?`, sessionID)
	return err
}

func (r *Repository) SessionHasOpenOrders(ctx context.Context, sessionID int64) (bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM orders
		WHERE session_id = ?
			AND status NOT IN ('paid', 'cancelled')
	`, sessionID)
	var count int64
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) GetLatestOrderForSession(ctx context.Context, sessionID int64) (*domain.Order, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			o.id,
			o.table_id,
			COALESCE(t.number, ''),
			o.session_id,
			o.customer_name,
			o.status,
			COALESCE(o.payment_status, 'unpaid'),
			o.subtotal_cents,
			o.vat_cents,
			o.total_cents,
			o.source,
			o.created_at,
			o.updated_at
		FROM orders o
		LEFT JOIN restaurant_tables t ON t.id = o.table_id
		WHERE o.session_id = ?
		ORDER BY o.created_at DESC
		LIMIT 1
	`, sessionID)
	order, err := scanOrder(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if err := r.hydrateOrder(ctx, &order); err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *Repository) ListMenu(ctx context.Context) ([]domain.Category, []domain.MenuItem, error) {
	return r.listMenu(ctx, true)
}

func (r *Repository) ListMenuAdmin(ctx context.Context) ([]domain.Category, []domain.MenuItem, error) {
	return r.listMenu(ctx, false)
}

func (r *Repository) listMenu(ctx context.Context, activeOnly bool) ([]domain.Category, []domain.MenuItem, error) {
	categoryQuery := `SELECT id, name, sort_order, active FROM menu_categories`
	itemQuery := `SELECT id, category_id, name, description, price_cents, image_url, COALESCE(sku, ''), COALESCE(item_type, 'food'), COALESCE(cost_cents, 0), COALESCE(sort_order, 0), active, created_at FROM menu_items`
	if activeOnly {
		categoryQuery += ` WHERE active = 1`
		itemQuery += ` WHERE active = 1`
	}
	categoryQuery += ` ORDER BY sort_order, name`
	itemQuery += ` ORDER BY category_id, sort_order, name`

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
		item, err := scanMenuItem(itemRows)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	return categories, items, itemRows.Err()
}

func (r *Repository) CreateCategory(ctx context.Context, name string, sortOrder int) (domain.Category, error) {
	row := r.db.QueryRowContext(ctx, `INSERT INTO menu_categories (name, sort_order) VALUES (?, ?) RETURNING id, name, sort_order, active`, strings.TrimSpace(name), sortOrder)
	var category domain.Category
	var active int
	if err := row.Scan(&category.ID, &category.Name, &category.SortOrder, &active); err != nil {
		return category, err
	}
	category.Active = active == 1
	return category, nil
}

func (r *Repository) GetMenuItem(ctx context.Context, id int64) (domain.MenuItem, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, category_id, name, description, price_cents, image_url, COALESCE(sku, ''), COALESCE(item_type, 'food'), COALESCE(cost_cents, 0), COALESCE(sort_order, 0), active, created_at FROM menu_items WHERE id = ?`, id)
	return scanMenuItem(row)
}

type SaveMenuItemInput struct {
	CategoryID  int64
	Name        string
	Description string
	PriceCents  int64
	ImageURL    string
	SKU         string
	ItemType    string
	CostCents   int64
	SortOrder   int
	StockQty    float64
	ReorderLevel float64
	Unit        string
	TrackStock  bool
	Active      bool
}

func (r *Repository) CreateMenuItem(ctx context.Context, input SaveMenuItemInput) (domain.MenuItem, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.MenuItem{}, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		INSERT INTO menu_items (category_id, name, description, price_cents, image_url, sku, item_type, cost_cents, sort_order, active)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, category_id, name, description, price_cents, image_url, COALESCE(sku, ''), COALESCE(item_type, 'food'), COALESCE(cost_cents, 0), COALESCE(sort_order, 0), active, created_at
	`,
		input.CategoryID,
		strings.TrimSpace(input.Name),
		strings.TrimSpace(input.Description),
		input.PriceCents,
		strings.TrimSpace(input.ImageURL),
		strings.TrimSpace(input.SKU),
		defaultString(strings.TrimSpace(input.ItemType), "food"),
		input.CostCents,
		input.SortOrder,
		boolToInt(input.Active),
	)
	item, err := scanMenuItem(row)
	if err != nil {
		return domain.MenuItem{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO inventory (product_id, stock_qty, reorder_level, unit, track_stock, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(product_id) DO UPDATE SET
			stock_qty = excluded.stock_qty,
			reorder_level = excluded.reorder_level,
			unit = excluded.unit,
			track_stock = excluded.track_stock,
			updated_at = CURRENT_TIMESTAMP
	`, item.ID, input.StockQty, input.ReorderLevel, defaultString(strings.TrimSpace(input.Unit), "pcs"), boolToInt(input.TrackStock)); err != nil {
		return domain.MenuItem{}, err
	}

	if input.StockQty != 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO inventory_movements (product_id, change_qty, reason, reference)
			VALUES (?, ?, 'initial_stock', 'product_create')
		`, item.ID, input.StockQty); err != nil {
			return domain.MenuItem{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return domain.MenuItem{}, err
	}
	return item, nil
}

func (r *Repository) UpdateMenuItem(ctx context.Context, id int64, input SaveMenuItemInput) (domain.MenuItem, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.MenuItem{}, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		UPDATE menu_items
		SET category_id = ?, name = ?, description = ?, price_cents = ?, image_url = ?, sku = ?, item_type = ?, cost_cents = ?, sort_order = ?, active = ?
		WHERE id = ?
		RETURNING id, category_id, name, description, price_cents, image_url, COALESCE(sku, ''), COALESCE(item_type, 'food'), COALESCE(cost_cents, 0), COALESCE(sort_order, 0), active, created_at
	`,
		input.CategoryID,
		strings.TrimSpace(input.Name),
		strings.TrimSpace(input.Description),
		input.PriceCents,
		strings.TrimSpace(input.ImageURL),
		strings.TrimSpace(input.SKU),
		defaultString(strings.TrimSpace(input.ItemType), "food"),
		input.CostCents,
		input.SortOrder,
		boolToInt(input.Active),
		id,
	)
	item, err := scanMenuItem(row)
	if err != nil {
		return domain.MenuItem{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO inventory (product_id, stock_qty, reorder_level, unit, track_stock, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(product_id) DO UPDATE SET
			stock_qty = excluded.stock_qty,
			reorder_level = excluded.reorder_level,
			unit = excluded.unit,
			track_stock = excluded.track_stock,
			updated_at = CURRENT_TIMESTAMP
	`, id, input.StockQty, input.ReorderLevel, defaultString(strings.TrimSpace(input.Unit), "pcs"), boolToInt(input.TrackStock)); err != nil {
		return domain.MenuItem{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.MenuItem{}, err
	}
	return item, nil
}

type CreateOrderInput struct {
	TableID      *int64                 `json:"table_id"`
	SessionToken string                 `json:"session_token"`
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

	var sessionID sql.NullInt64
	tableID := input.TableID
	if strings.TrimSpace(input.SessionToken) != "" {
		var sessionTableID int64
		row := tx.QueryRowContext(ctx, `SELECT id, table_id FROM sessions WHERE token = ? AND status = 'active' LIMIT 1`, strings.TrimSpace(input.SessionToken))
		if err := row.Scan(&sessionID, &sessionTableID); err != nil {
			return domain.Order{}, errors.New("invalid table session")
		}
		if tableID == nil {
			tableID = &sessionTableID
		}
	}

	type pricedItem struct {
		input CreateOrderItemInput
		menu  domain.MenuItem
	}

	var subtotal int64
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
	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = "qr"
	}

	row := tx.QueryRowContext(ctx, `
		INSERT INTO orders (table_id, session_id, customer_name, status, payment_status, source, subtotal_cents, vat_cents, total_cents)
		VALUES (?, ?, ?, 'new', 'unpaid', ?, ?, ?, ?)
		RETURNING id, table_id, '', session_id, customer_name, status, payment_status, subtotal_cents, vat_cents, total_cents, source, created_at, updated_at
	`, nullablePtrInt64(tableID), nullableInt64(sessionID), strings.TrimSpace(input.CustomerName), source, subtotal, vat, total)
	order, err := scanOrder(row)
	if err != nil {
		return domain.Order{}, err
	}

	for _, item := range priced {
		row := tx.QueryRowContext(ctx, `
			INSERT INTO order_items (order_id, menu_item_id, quantity, unit_price_cents, notes)
			VALUES (?, ?, ?, ?, ?)
			RETURNING id, order_id, menu_item_id, quantity, unit_price_cents, notes, status
		`, order.ID, item.menu.ID, item.input.Quantity, item.menu.PriceCents, strings.TrimSpace(item.input.Notes))
		orderItem, err := scanOrderItem(row)
		if err != nil {
			return domain.Order{}, err
		}
		orderItem.MenuItemName = item.menu.Name
		order.Items = append(order.Items, orderItem)

		if _, err := tx.ExecContext(ctx, `
			UPDATE ingredients
			SET stock_qty = stock_qty - (
				SELECT mii.qty * ?
				FROM menu_item_ingredients mii
				WHERE mii.menu_item_id = ? AND mii.ingredient_id = ingredients.id
			)
			WHERE id IN (
				SELECT ingredient_id FROM menu_item_ingredients WHERE menu_item_id = ?
			)
		`, item.input.Quantity, item.menu.ID, item.menu.ID); err != nil {
			return domain.Order{}, err
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE inventory
			SET stock_qty = stock_qty - ?, updated_at = CURRENT_TIMESTAMP
			WHERE product_id = ? AND track_stock = 1
		`, item.input.Quantity, item.menu.ID); err != nil {
			return domain.Order{}, err
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO inventory_movements (product_id, change_qty, reason, reference)
			SELECT ?, ?, 'sale', 'order-' || ?
			WHERE EXISTS (SELECT 1 FROM inventory WHERE product_id = ? AND track_stock = 1)
		`, item.menu.ID, -float64(item.input.Quantity), order.ID, item.menu.ID); err != nil {
			return domain.Order{}, err
		}
	}

	if tableID != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE restaurant_tables SET status = 'occupied' WHERE id = ?`, *tableID); err != nil {
			return domain.Order{}, err
		}
	}

	if sessionID.Valid {
		if _, err := tx.ExecContext(ctx, `UPDATE sessions SET last_seen_at = CURRENT_TIMESTAMP WHERE id = ?`, sessionID.Int64); err != nil {
			return domain.Order{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return domain.Order{}, err
	}

	return r.GetOrder(ctx, order.ID)
}

func (r *Repository) GetOrder(ctx context.Context, id int64) (domain.Order, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			o.id,
			o.table_id,
			COALESCE(t.number, ''),
			o.session_id,
			o.customer_name,
			o.status,
			COALESCE(o.payment_status, 'unpaid'),
			o.subtotal_cents,
			o.vat_cents,
			o.total_cents,
			o.source,
			o.created_at,
			o.updated_at
		FROM orders o
		LEFT JOIN restaurant_tables t ON t.id = o.table_id
		WHERE o.id = ?
		LIMIT 1
	`, id)
	order, err := scanOrder(row)
	if err != nil {
		return order, err
	}
	return order, r.hydrateOrder(ctx, &order)
}

func (r *Repository) ListOrders(ctx context.Context, limit int) ([]domain.Order, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			o.id,
			o.table_id,
			COALESCE(t.number, ''),
			o.session_id,
			o.customer_name,
			o.status,
			COALESCE(o.payment_status, 'unpaid'),
			o.subtotal_cents,
			o.vat_cents,
			o.total_cents,
			o.source,
			o.created_at,
			o.updated_at
		FROM orders o
		LEFT JOIN restaurant_tables t ON t.id = o.table_id
		ORDER BY o.created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := []domain.Order{}
	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range orders {
		if err := r.hydrateOrder(ctx, &orders[i]); err != nil {
			return nil, err
		}
	}
	return orders, nil
}

func (r *Repository) ListOrderItems(ctx context.Context, orderID int64) ([]domain.OrderItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			oi.id,
			oi.order_id,
			oi.menu_item_id,
			COALESCE(mi.name, ''),
			oi.quantity,
			oi.unit_price_cents,
			oi.notes,
			oi.status
		FROM order_items oi
		JOIN menu_items mi ON mi.id = oi.menu_item_id
		WHERE oi.order_id = ?
		ORDER BY oi.id
	`, orderID)
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

func (r *Repository) ListOrderPayments(ctx context.Context, orderID int64) ([]domain.Payment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, order_id, method, amount_cents, status, reference, phone_number, provider, metadata_json, confirmed_at, created_at
		FROM payments
		WHERE order_id = ?
		ORDER BY created_at DESC
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	payments := []domain.Payment{}
	for rows.Next() {
		payment, err := scanPayment(rows)
		if err != nil {
			return nil, err
		}
		payments = append(payments, payment)
	}
	return payments, rows.Err()
}

func (r *Repository) UpdateOrderStatus(ctx context.Context, id int64, status string) (domain.Order, error) {
	if status == "paid" {
		if _, err := r.db.ExecContext(ctx, `UPDATE orders SET status = 'paid', payment_status = 'paid', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id); err != nil {
			return domain.Order{}, err
		}
	} else {
		if _, err := r.db.ExecContext(ctx, `UPDATE orders SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, id); err != nil {
			return domain.Order{}, err
		}
	}
	return r.GetOrder(ctx, id)
}

func (r *Repository) UpdateOrderPaymentStatus(ctx context.Context, id int64, paymentStatus string, updateStatus bool) (domain.Order, error) {
	status := "status"
	value := "status"
	if updateStatus && paymentStatus == "paid" {
		status = "'paid'"
		value = status
	}
	query := fmt.Sprintf(`UPDATE orders SET payment_status = ?, status = %s, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, value)
	if _, err := r.db.ExecContext(ctx, query, paymentStatus, id); err != nil {
		return domain.Order{}, err
	}
	return r.GetOrder(ctx, id)
}

type CreatePaymentInput struct {
	OrderID      int64
	Method       string
	AmountCents  int64
	Status       string
	Reference    string
	PhoneNumber  string
	Provider     string
	MetadataJSON string
}

func (r *Repository) CreatePayment(ctx context.Context, input CreatePaymentInput) (domain.Payment, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO payments (order_id, method, amount_cents, status, reference, phone_number, provider, metadata_json, confirmed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CASE WHEN ? = 'paid' THEN CURRENT_TIMESTAMP ELSE NULL END)
		RETURNING id, order_id, method, amount_cents, status, reference, phone_number, provider, metadata_json, confirmed_at, created_at
	`, input.OrderID, input.Method, input.AmountCents, input.Status, input.Reference, input.PhoneNumber, input.Provider, input.MetadataJSON, input.Status)
	return scanPayment(row)
}

func (r *Repository) GetPayment(ctx context.Context, id int64) (domain.Payment, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, order_id, method, amount_cents, status, reference, phone_number, provider, metadata_json, confirmed_at, created_at
		FROM payments
		WHERE id = ?
		LIMIT 1
	`, id)
	return scanPayment(row)
}

func (r *Repository) ConfirmPayment(ctx context.Context, id int64, reference string) (domain.Payment, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE payments
		SET
			status = 'paid',
			reference = CASE WHEN ? = '' THEN reference ELSE ? END,
			confirmed_at = CURRENT_TIMESTAMP
		WHERE id = ?
		RETURNING id, order_id, method, amount_cents, status, reference, phone_number, provider, metadata_json, confirmed_at, created_at
	`, strings.TrimSpace(reference), strings.TrimSpace(reference), id)
	return scanPayment(row)
}

func (r *Repository) GetReceiptByOrderID(ctx context.Context, orderID int64) (*domain.Receipt, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, order_id, payment_id, receipt_number, payload_json, created_at
		FROM receipts
		WHERE order_id = ?
		LIMIT 1
	`, orderID)
	receipt, err := scanReceipt(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &receipt, nil
}

func (r *Repository) CreateReceipt(ctx context.Context, orderID int64, paymentID *int64, receiptNumber string, payloadJSON string) (domain.Receipt, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO receipts (order_id, payment_id, receipt_number, payload_json)
		VALUES (?, ?, ?, ?)
		RETURNING id, order_id, payment_id, receipt_number, payload_json, created_at
	`, orderID, paymentID, receiptNumber, payloadJSON)
	return scanReceipt(row)
}

func (r *Repository) Analytics(ctx context.Context) (domain.AnalyticsSnapshot, error) {
	snapshot := domain.AnalyticsSnapshot{
		DailySales:     []domain.SalesPoint{},
		BestSellers:    []domain.BestSeller{},
		LowStock:       []domain.Ingredient{},
		PaymentMethods: []domain.PaymentMethodStat{},
	}

	salesRows, err := r.db.QueryContext(ctx, `
		SELECT
			date(created_at) AS day,
			COUNT(*) AS order_count,
			COALESCE(SUM(total_cents), 0) AS revenue_cents
		FROM orders
		WHERE status IN ('paid', 'served', 'ready') OR payment_status = 'paid'
		GROUP BY date(created_at)
		ORDER BY date(created_at) DESC
		LIMIT 30
	`)
	if err != nil {
		return snapshot, err
	}
	for salesRows.Next() {
		var point domain.SalesPoint
		if err := salesRows.Scan(&point.Day, &point.OrderCount, &point.RevenueCents); err != nil {
			_ = salesRows.Close()
			return snapshot, err
		}
		snapshot.DailySales = append(snapshot.DailySales, point)
	}
	if err := salesRows.Err(); err != nil {
		_ = salesRows.Close()
		return snapshot, err
	}
	if err := salesRows.Close(); err != nil {
		return snapshot, err
	}

	bestRows, err := r.db.QueryContext(ctx, `
		SELECT mi.name, COALESCE(SUM(oi.quantity), 0), COALESCE(SUM(oi.quantity * oi.unit_price_cents), 0)
		FROM order_items oi
		JOIN menu_items mi ON mi.id = oi.menu_item_id
		GROUP BY mi.id, mi.name
		ORDER BY SUM(oi.quantity) DESC, mi.name ASC
		LIMIT 10
	`)
	if err != nil {
		return snapshot, err
	}
	for bestRows.Next() {
		var item domain.BestSeller
		if err := bestRows.Scan(&item.Name, &item.Quantity, &item.RevenueCents); err != nil {
			_ = bestRows.Close()
			return snapshot, err
		}
		snapshot.BestSellers = append(snapshot.BestSellers, item)
	}
	if err := bestRows.Err(); err != nil {
		_ = bestRows.Close()
		return snapshot, err
	}
	if err := bestRows.Close(); err != nil {
		return snapshot, err
	}

	stockRows, err := r.db.QueryContext(ctx, `SELECT id, name, unit, stock_qty, low_stock_qty FROM ingredients WHERE stock_qty <= low_stock_qty ORDER BY stock_qty ASC`)
	if err != nil {
		return snapshot, err
	}
	for stockRows.Next() {
		var ingredient domain.Ingredient
		if err := stockRows.Scan(&ingredient.ID, &ingredient.Name, &ingredient.Unit, &ingredient.StockQty, &ingredient.LowStockQty); err != nil {
			_ = stockRows.Close()
			return snapshot, err
		}
		snapshot.LowStock = append(snapshot.LowStock, ingredient)
	}
	if err := stockRows.Err(); err != nil {
		_ = stockRows.Close()
		return snapshot, err
	}
	if err := stockRows.Close(); err != nil {
		return snapshot, err
	}

	paymentRows, err := r.db.QueryContext(ctx, `
		SELECT method, COUNT(*), COALESCE(SUM(amount_cents), 0)
		FROM payments
		WHERE status = 'paid'
		GROUP BY method
		ORDER BY COUNT(*) DESC, method ASC
	`)
	if err != nil {
		return snapshot, err
	}
	for paymentRows.Next() {
		var stat domain.PaymentMethodStat
		if err := paymentRows.Scan(&stat.Method, &stat.Count, &stat.RevenueCents); err != nil {
			_ = paymentRows.Close()
			return snapshot, err
		}
		snapshot.PaymentMethods = append(snapshot.PaymentMethods, stat)
	}
	if err := paymentRows.Err(); err != nil {
		_ = paymentRows.Close()
		return snapshot, err
	}
	if err := paymentRows.Close(); err != nil {
		return snapshot, err
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN date(created_at) = date('now', 'localtime') AND status = 'paid' THEN amount_cents END), 0),
			COALESCE(SUM(CASE WHEN date(created_at) >= date('now', '-6 day', 'localtime') AND status = 'paid' THEN amount_cents END), 0),
			COALESCE(SUM(CASE WHEN strftime('%Y-%m', created_at) = strftime('%Y-%m', 'now', 'localtime') AND status = 'paid' THEN amount_cents END), 0)
		FROM payments
	`)
	if err := row.Scan(&snapshot.DailyTotalCents, &snapshot.WeeklyTotalCents, &snapshot.MonthlyTotalCents); err != nil {
		return snapshot, err
	}

	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders WHERE status NOT IN ('paid', 'cancelled')`).Scan(&snapshot.OpenOrders); err != nil {
		return snapshot, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE status = 'active'`).Scan(&snapshot.ActiveSessions); err != nil {
		return snapshot, err
	}

	return snapshot, nil
}

func (r *Repository) ListProducts(ctx context.Context) ([]domain.Product, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			mi.id,
			mi.category_id,
			COALESCE(mc.name, ''),
			mi.name,
			mi.description,
			mi.price_cents,
			COALESCE(mi.cost_cents, 0),
			mi.image_url,
			COALESCE(mi.sku, ''),
			COALESCE(mi.item_type, 'food'),
			COALESCE(mi.sort_order, 0),
			mi.active,
			COALESCE(inv.stock_qty, 0),
			COALESCE(inv.reorder_level, 0),
			COALESCE(inv.unit, 'pcs'),
			COALESCE(inv.track_stock, 1),
			mi.created_at
		FROM menu_items mi
		LEFT JOIN menu_categories mc ON mc.id = mi.category_id
		LEFT JOIN inventory inv ON inv.product_id = mi.id
		ORDER BY mi.active DESC, mc.sort_order ASC, mi.sort_order ASC, mi.name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := []domain.Product{}
	for rows.Next() {
		product, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	return products, rows.Err()
}

func (r *Repository) GetProduct(ctx context.Context, id int64) (domain.Product, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			mi.id,
			mi.category_id,
			COALESCE(mc.name, ''),
			mi.name,
			mi.description,
			mi.price_cents,
			COALESCE(mi.cost_cents, 0),
			mi.image_url,
			COALESCE(mi.sku, ''),
			COALESCE(mi.item_type, 'food'),
			COALESCE(mi.sort_order, 0),
			mi.active,
			COALESCE(inv.stock_qty, 0),
			COALESCE(inv.reorder_level, 0),
			COALESCE(inv.unit, 'pcs'),
			COALESCE(inv.track_stock, 1),
			mi.created_at
		FROM menu_items mi
		LEFT JOIN menu_categories mc ON mc.id = mi.category_id
		LEFT JOIN inventory inv ON inv.product_id = mi.id
		WHERE mi.id = ?
		LIMIT 1
	`, id)
	return scanProduct(row)
}

func (r *Repository) ArchiveProduct(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE menu_items SET active = 0 WHERE id = ?`, id)
	return err
}

func (r *Repository) AdjustInventory(ctx context.Context, input domain.InventoryAdjustment) (domain.Product, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Product{}, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO inventory (product_id, stock_qty, reorder_level, unit, track_stock, updated_at)
		VALUES (?, ?, 0, 'pcs', 1, CURRENT_TIMESTAMP)
		ON CONFLICT(product_id) DO NOTHING
	`, input.ProductID, 0); err != nil {
		return domain.Product{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE inventory
		SET stock_qty = stock_qty + ?, updated_at = CURRENT_TIMESTAMP
		WHERE product_id = ?
	`, input.ChangeQty, input.ProductID); err != nil {
		return domain.Product{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO inventory_movements (product_id, change_qty, reason, reference)
		VALUES (?, ?, ?, ?)
	`, input.ProductID, input.ChangeQty, defaultString(strings.TrimSpace(input.Reason), "manual_adjustment"), strings.TrimSpace(input.Reference)); err != nil {
		return domain.Product{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.Product{}, err
	}

	return r.GetProduct(ctx, input.ProductID)
}

func (r *Repository) ListInventoryMovements(ctx context.Context, limit int) ([]domain.InventoryMovement, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			im.id,
			im.product_id,
			COALESCE(mi.name, ''),
			im.change_qty,
			im.reason,
			im.reference,
			im.created_at
		FROM inventory_movements im
		JOIN menu_items mi ON mi.id = im.product_id
		ORDER BY im.created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	movements := []domain.InventoryMovement{}
	for rows.Next() {
		var movement domain.InventoryMovement
		if err := rows.Scan(&movement.ID, &movement.ProductID, &movement.ProductName, &movement.ChangeQty, &movement.Reason, &movement.Reference, &movement.CreatedAt); err != nil {
			return nil, err
		}
		movements = append(movements, movement)
	}
	return movements, rows.Err()
}

func (r *Repository) GetSettings(ctx context.Context) (domain.Settings, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, business_name, business_type, phone, currency_code, mpesa_till, receipt_footer, created_at, updated_at
		FROM settings
		WHERE id = 1
		LIMIT 1
	`)
	var settings domain.Settings
	err := row.Scan(&settings.ID, &settings.BusinessName, &settings.BusinessType, &settings.Phone, &settings.CurrencyCode, &settings.MPesaTill, &settings.ReceiptFooter, &settings.CreatedAt, &settings.UpdatedAt)
	return settings, err
}

func (r *Repository) SaveSettings(ctx context.Context, input domain.Settings) (domain.Settings, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO settings (id, business_name, business_type, phone, currency_code, mpesa_till, receipt_footer, created_at, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			business_name = excluded.business_name,
			business_type = excluded.business_type,
			phone = excluded.phone,
			currency_code = excluded.currency_code,
			mpesa_till = excluded.mpesa_till,
			receipt_footer = excluded.receipt_footer,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id, business_name, business_type, phone, currency_code, mpesa_till, receipt_footer, created_at, updated_at
	`, strings.TrimSpace(input.BusinessName), strings.TrimSpace(input.BusinessType), strings.TrimSpace(input.Phone), defaultString(strings.TrimSpace(input.CurrencyCode), "KES"), strings.TrimSpace(input.MPesaTill), strings.TrimSpace(input.ReceiptFooter))

	var settings domain.Settings
	err := row.Scan(&settings.ID, &settings.BusinessName, &settings.BusinessType, &settings.Phone, &settings.CurrencyCode, &settings.MPesaTill, &settings.ReceiptFooter, &settings.CreatedAt, &settings.UpdatedAt)
	return settings, err
}

func (r *Repository) ListRecentPayments(ctx context.Context, limit int) ([]domain.Payment, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, order_id, method, amount_cents, status, reference, phone_number, provider, metadata_json, confirmed_at, created_at
		FROM payments
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	payments := []domain.Payment{}
	for rows.Next() {
		payment, err := scanPayment(rows)
		if err != nil {
			return nil, err
		}
		payments = append(payments, payment)
	}
	return payments, rows.Err()
}

func (r *Repository) DashboardSnapshot(ctx context.Context) (domain.DashboardSnapshot, error) {
	var snapshot domain.DashboardSnapshot

	settings, err := r.GetSettings(ctx)
	if err != nil {
		return snapshot, err
	}
	snapshot.Business = settings

	row := r.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN date(p.created_at) = date('now', 'localtime') AND p.status = 'paid' THEN p.amount_cents END), 0),
			COALESCE(SUM(CASE WHEN date(p.created_at) >= date('now', '-6 day', 'localtime') AND p.status = 'paid' THEN p.amount_cents END), 0),
			COALESCE(SUM(CASE WHEN strftime('%Y-%m', p.created_at) = strftime('%Y-%m', 'now', 'localtime') AND p.status = 'paid' THEN p.amount_cents END), 0),
			COALESCE(SUM(CASE WHEN date(o.created_at) = date('now', 'localtime') AND p.status = 'paid' THEN (oi.unit_price_cents - COALESCE(mi.cost_cents, 0)) * oi.quantity END), 0),
			COALESCE(SUM(CASE WHEN date(o.created_at) >= date('now', '-6 day', 'localtime') AND p.status = 'paid' THEN (oi.unit_price_cents - COALESCE(mi.cost_cents, 0)) * oi.quantity END), 0),
			COALESCE(SUM(CASE WHEN strftime('%Y-%m', o.created_at) = strftime('%Y-%m', 'now', 'localtime') AND p.status = 'paid' THEN (oi.unit_price_cents - COALESCE(mi.cost_cents, 0)) * oi.quantity END), 0)
		FROM payments p
		LEFT JOIN orders o ON o.id = p.order_id
		LEFT JOIN order_items oi ON oi.order_id = o.id
		LEFT JOIN menu_items mi ON mi.id = oi.menu_item_id
	`)
	if err := row.Scan(
		&snapshot.DailyRevenueCents,
		&snapshot.WeeklyRevenueCents,
		&snapshot.MonthlyRevenueCents,
		&snapshot.DailyProfitCents,
		&snapshot.WeeklyProfitCents,
		&snapshot.MonthlyProfitCents,
	); err != nil {
		return snapshot, err
	}

	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders WHERE status NOT IN ('paid', 'cancelled')`).Scan(&snapshot.OpenOrders); err != nil {
		return snapshot, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders WHERE date(updated_at) = date('now', 'localtime') AND payment_status = 'paid'`).Scan(&snapshot.PaidOrdersToday); err != nil {
		return snapshot, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM menu_items WHERE active = 1`).Scan(&snapshot.ProductsCount); err != nil {
		return snapshot, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM restaurant_tables WHERE status = 'occupied'`).Scan(&snapshot.ActiveOrderPoints); err != nil {
		return snapshot, err
	}

	analytics, err := r.Analytics(ctx)
	if err != nil {
		return snapshot, err
	}
	snapshot.BestSellers = analytics.BestSellers
	snapshot.SalesTrend = analytics.DailySales
	snapshot.PaymentMethods = analytics.PaymentMethods

	recentOrders, err := r.ListOrders(ctx, 8)
	if err != nil {
		return snapshot, err
	}
	snapshot.RecentOrders = recentOrders

	recentPayments, err := r.ListRecentPayments(ctx, 8)
	if err != nil {
		return snapshot, err
	}
	snapshot.RecentPayments = recentPayments

	lowStockRows, err := r.db.QueryContext(ctx, `
		SELECT
			mi.id,
			mi.category_id,
			COALESCE(mc.name, ''),
			mi.name,
			mi.description,
			mi.price_cents,
			COALESCE(mi.cost_cents, 0),
			mi.image_url,
			COALESCE(mi.sku, ''),
			COALESCE(mi.item_type, 'food'),
			COALESCE(mi.sort_order, 0),
			mi.active,
			COALESCE(inv.stock_qty, 0),
			COALESCE(inv.reorder_level, 0),
			COALESCE(inv.unit, 'pcs'),
			COALESCE(inv.track_stock, 1),
			mi.created_at
		FROM menu_items mi
		JOIN inventory inv ON inv.product_id = mi.id
		LEFT JOIN menu_categories mc ON mc.id = mi.category_id
		WHERE mi.active = 1 AND inv.track_stock = 1 AND inv.stock_qty <= inv.reorder_level
		ORDER BY inv.stock_qty ASC, mi.name ASC
		LIMIT 8
	`)
	if err != nil {
		return snapshot, err
	}
	defer lowStockRows.Close()
	snapshot.LowStockProducts = []domain.Product{}
	for lowStockRows.Next() {
		product, err := scanProduct(lowStockRows)
		if err != nil {
			return snapshot, err
		}
		snapshot.LowStockProducts = append(snapshot.LowStockProducts, product)
	}
	return snapshot, lowStockRows.Err()
}

func (r *Repository) hydrateOrder(ctx context.Context, order *domain.Order) error {
	items, err := r.ListOrderItems(ctx, order.ID)
	if err != nil {
		return err
	}
	payments, err := r.ListOrderPayments(ctx, order.ID)
	if err != nil {
		return err
	}
	receipt, err := r.GetReceiptByOrderID(ctx, order.ID)
	if err != nil {
		return err
	}
	order.Items = items
	order.Payments = payments
	order.Receipt = receipt
	return nil
}

func scanMenuItem(row interface{ Scan(dest ...any) error }) (domain.MenuItem, error) {
	var item domain.MenuItem
	var active int
	err := row.Scan(
		&item.ID,
		&item.CategoryID,
		&item.Name,
		&item.Description,
		&item.PriceCents,
		&item.ImageURL,
		&item.SKU,
		&item.ItemType,
		&item.CostCents,
		&item.SortOrder,
		&active,
		&item.CreatedAt,
	)
	item.Active = active == 1
	return item, err
}

func scanProduct(row interface{ Scan(dest ...any) error }) (domain.Product, error) {
	var product domain.Product
	var active int
	var trackStock int
	err := row.Scan(
		&product.ID,
		&product.CategoryID,
		&product.CategoryName,
		&product.Name,
		&product.Description,
		&product.PriceCents,
		&product.CostCents,
		&product.ImageURL,
		&product.SKU,
		&product.ItemType,
		&product.SortOrder,
		&active,
		&product.StockQty,
		&product.ReorderLevel,
		&product.Unit,
		&trackStock,
		&product.CreatedAt,
	)
	product.Active = active == 1
	product.TrackStock = trackStock == 1
	product.OutOfStock = product.TrackStock && product.StockQty <= 0
	return product, err
}

func scanTable(row interface{ Scan(dest ...any) error }) (domain.Table, error) {
	var table domain.Table
	var active int
	err := row.Scan(&table.ID, &table.Number, &table.Seats, &table.Status, &table.Slug, &table.QRToken, &table.QRURL, &table.QRImageData, &active, &table.CreatedAt)
	table.Active = active == 1
	return table, err
}

func scanSession(row interface{ Scan(dest ...any) error }) (domain.TableSession, error) {
	var session domain.TableSession
	var endedAt sql.NullTime
	err := row.Scan(&session.ID, &session.TableID, &session.TableNumber, &session.Token, &session.CustomerName, &session.Status, &session.StartedAt, &session.LastSeenAt, &endedAt)
	if endedAt.Valid {
		session.EndedAt = &endedAt.Time
	}
	return session, err
}

func scanOrder(row interface{ Scan(dest ...any) error }) (domain.Order, error) {
	var order domain.Order
	var tableID sql.NullInt64
	var sessionID sql.NullInt64
	err := row.Scan(
		&order.ID,
		&tableID,
		&order.TableNumber,
		&sessionID,
		&order.CustomerName,
		&order.Status,
		&order.PaymentStatus,
		&order.SubtotalCents,
		&order.VATCents,
		&order.TotalCents,
		&order.Source,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if tableID.Valid {
		order.TableID = &tableID.Int64
	}
	if sessionID.Valid {
		order.SessionID = &sessionID.Int64
	}
	return order, err
}

func scanOrderItem(row interface{ Scan(dest ...any) error }) (domain.OrderItem, error) {
	var item domain.OrderItem
	err := row.Scan(&item.ID, &item.OrderID, &item.MenuItemID, &item.Quantity, &item.UnitPriceCents, &item.Notes, &item.Status)
	return item, err
}

func scanPayment(row interface{ Scan(dest ...any) error }) (domain.Payment, error) {
	var payment domain.Payment
	var confirmedAt sql.NullTime
	err := row.Scan(&payment.ID, &payment.OrderID, &payment.Method, &payment.AmountCents, &payment.Status, &payment.Reference, &payment.PhoneNumber, &payment.Provider, &payment.MetadataJSON, &confirmedAt, &payment.CreatedAt)
	if confirmedAt.Valid {
		payment.ConfirmedAt = &confirmedAt.Time
	}
	return payment, err
}

func scanReceipt(row interface{ Scan(dest ...any) error }) (domain.Receipt, error) {
	var receipt domain.Receipt
	var paymentID sql.NullInt64
	err := row.Scan(&receipt.ID, &receipt.OrderID, &paymentID, &receipt.ReceiptNumber, &receipt.PayloadJSON, &receipt.CreatedAt)
	if paymentID.Valid {
		receipt.PaymentID = &paymentID.Int64
	}
	return receipt, err
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func nullableInt64(value sql.NullInt64) any {
	if value.Valid {
		return value.Int64
	}
	return nil
}

func nullablePtrInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func (r *Repository) getMenuItemTx(ctx context.Context, tx *sql.Tx, id int64) (domain.MenuItem, error) {
	row := tx.QueryRowContext(ctx, `SELECT id, category_id, name, description, price_cents, image_url, COALESCE(sku, ''), COALESCE(item_type, 'food'), COALESCE(cost_cents, 0), COALESCE(sort_order, 0), active, created_at FROM menu_items WHERE id = ? AND active = 1`, id)
	return scanMenuItem(row)
}

func receiptPayload(order domain.Order, payment domain.Payment) string {
	body, err := json.Marshal(map[string]any{
		"order_id":       order.ID,
		"table_number":   order.TableNumber,
		"customer_name":  order.CustomerName,
		"status":         order.Status,
		"payment_status": order.PaymentStatus,
		"total_cents":    order.TotalCents,
		"vat_cents":      order.VATCents,
		"payment_method": payment.Method,
		"payment_ref":    payment.Reference,
		"paid_at":        payment.ConfirmedAt,
		"generated_at":   time.Now().UTC(),
		"items":          order.Items,
	})
	if err != nil {
		return "{}"
	}
	return string(body)
}
