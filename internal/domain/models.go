package domain

import "time"

type Role string

const (
	RoleAdmin   Role = "admin"
	RoleManager Role = "manager"
	RoleChef    Role = "chef"
	RoleCashier Role = "cashier"
	RoleWaiter  Role = "waiter"
)

type User struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      Role      `json:"role"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

type Table struct {
	ID        int64     `json:"id"`
	Number    string    `json:"number"`
	Seats     int       `json:"seats"`
	Status    string    `json:"status"`
	QRToken   string    `json:"qr_token"`
	QRURL     string    `json:"qr_url"`
	CreatedAt time.Time `json:"created_at"`
}

type Category struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	Active    bool   `json:"active"`
}

type MenuItem struct {
	ID          int64     `json:"id"`
	CategoryID  int64     `json:"category_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	PriceCents  int64     `json:"price_cents"`
	ImageURL    string    `json:"image_url"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
}

type Order struct {
	ID            int64       `json:"id"`
	TableID       *int64      `json:"table_id,omitempty"`
	TableNumber   string      `json:"table_number,omitempty"`
	CustomerName  string      `json:"customer_name"`
	Status        string      `json:"status"`
	SubtotalCents int64       `json:"subtotal_cents"`
	VATCents      int64       `json:"vat_cents"`
	TotalCents    int64       `json:"total_cents"`
	Source        string      `json:"source"`
	Items         []OrderItem `json:"items,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

type OrderItem struct {
	ID             int64  `json:"id"`
	OrderID        int64  `json:"order_id"`
	MenuItemID     int64  `json:"menu_item_id"`
	MenuItemName   string `json:"menu_item_name,omitempty"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	Notes          string `json:"notes"`
	Status         string `json:"status"`
}

type Payment struct {
	ID          int64     `json:"id"`
	OrderID     int64     `json:"order_id"`
	Method      string    `json:"method"`
	AmountCents int64     `json:"amount_cents"`
	Status      string    `json:"status"`
	Reference   string    `json:"reference"`
	CreatedAt   time.Time `json:"created_at"`
}

type Ingredient struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Unit        string  `json:"unit"`
	StockQty    float64 `json:"stock_qty"`
	LowStockQty float64 `json:"low_stock_qty"`
}

type SalesPoint struct {
	Day          string `json:"day"`
	OrderCount   int64  `json:"order_count"`
	RevenueCents int64  `json:"revenue_cents"`
}

type BestSeller struct {
	Name         string `json:"name"`
	Quantity     int64  `json:"quantity"`
	RevenueCents int64  `json:"revenue_cents"`
}
