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

func StaffRoles() []Role {
	return []Role{RoleAdmin, RoleChef, RoleCashier, RoleWaiter, RoleManager}
}

type User struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      Role      `json:"role"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

type Table struct {
	ID          int64     `json:"id"`
	Number      string    `json:"number"`
	Seats       int       `json:"seats"`
	Status      string    `json:"status"`
	Slug        string    `json:"slug"`
	QRToken     string    `json:"qr_token"`
	QRURL       string    `json:"qr_url"`
	QRImageData string    `json:"qr_image_data"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
}

type TableSession struct {
	ID           int64      `json:"id"`
	TableID      int64      `json:"table_id"`
	TableNumber  string     `json:"table_number,omitempty"`
	Token        string     `json:"token"`
	CustomerName string     `json:"customer_name"`
	Status       string     `json:"status"`
	StartedAt    time.Time  `json:"started_at"`
	LastSeenAt   time.Time  `json:"last_seen_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
}

type TableContext struct {
	Table       Table        `json:"table"`
	Session     TableSession `json:"session"`
	ActiveOrder *Order       `json:"active_order,omitempty"`
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
	SessionID     *int64      `json:"session_id,omitempty"`
	CustomerName  string      `json:"customer_name"`
	Status        string      `json:"status"`
	PaymentStatus string      `json:"payment_status"`
	SubtotalCents int64       `json:"subtotal_cents"`
	VATCents      int64       `json:"vat_cents"`
	TotalCents    int64       `json:"total_cents"`
	Source        string      `json:"source"`
	Items         []OrderItem `json:"items,omitempty"`
	Payments      []Payment   `json:"payments,omitempty"`
	Receipt       *Receipt    `json:"receipt,omitempty"`
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
	ID           int64      `json:"id"`
	OrderID      int64      `json:"order_id"`
	Method       string     `json:"method"`
	AmountCents  int64      `json:"amount_cents"`
	Status       string     `json:"status"`
	Reference    string     `json:"reference"`
	PhoneNumber  string     `json:"phone_number"`
	Provider     string     `json:"provider"`
	MetadataJSON string     `json:"metadata_json"`
	ConfirmedAt  *time.Time `json:"confirmed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type Receipt struct {
	ID            int64     `json:"id"`
	OrderID       int64     `json:"order_id"`
	PaymentID     *int64    `json:"payment_id,omitempty"`
	ReceiptNumber string    `json:"receipt_number"`
	PayloadJSON   string    `json:"payload_json"`
	CreatedAt     time.Time `json:"created_at"`
}

type PaymentMethodStat struct {
	Method       string `json:"method"`
	Count        int64  `json:"count"`
	RevenueCents int64  `json:"revenue_cents"`
}

type AnalyticsSnapshot struct {
	DailySales        []SalesPoint        `json:"daily_sales"`
	BestSellers       []BestSeller        `json:"best_sellers"`
	LowStock          []Ingredient        `json:"low_stock"`
	PaymentMethods    []PaymentMethodStat `json:"payment_methods"`
	DailyTotalCents   int64               `json:"daily_total_cents"`
	WeeklyTotalCents  int64               `json:"weekly_total_cents"`
	MonthlyTotalCents int64               `json:"monthly_total_cents"`
	OpenOrders        int64               `json:"open_orders"`
	ActiveSessions    int64               `json:"active_sessions"`
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
