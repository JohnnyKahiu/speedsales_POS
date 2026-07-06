package laybye

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/pkg/products"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/variables"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

// insertArgs is the number of $N params in the laybye_items INSERT statement:
// laybye_id, item_code, item_name, quantity, cost, price, discount, total, vat, vat_alpha, on_offer
const insertArgs = 11

func anyArgs(n int) []interface{} {
	args := make([]interface{}, n)
	for i := range args {
		args[i] = pgxmock.AnyArg()
	}
	return args
}

// stubProduct returns a fetchProduct stub that populates p without a gRPC call.
func stubProduct(name string, cost, price, vatPct float64, vatAlpha string, onOffer bool) func(*products.StockMaster, context.Context) error {
	return func(p *products.StockMaster, _ context.Context) error {
		p.ItemName = name
		p.ItemCost = cost
		p.TillPrice = price
		p.VatPercent = vatPct
		p.VatAlpha = vatAlpha
		p.OnOffer = onOffer
		return nil
	}
}

// stubSettings returns a fetchSettings stub with the given PriceTag value.
func stubSettings(priceTag bool) func() (variables.PosSettings, error) {
	return func() (variables.PosSettings, error) {
		return variables.PosSettings{PriceTag: priceTag}, nil
	}
}

// newMockAndTx creates a pgxmock pool, consumes ExpectBegin, and returns both
// the mock (for further expectation setup and ExpectationsWereMet) and the tx.
func newMockAndTx(t *testing.T) (pgxmock.PgxPoolIface, pgx.Tx) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool: %v", err)
	}
	mock.ExpectBegin()
	tx, err := mock.BeginTx(context.Background(), pgx.TxOptions{})
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	return mock, tx
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		item    LaybyeItem
		wantErr bool
	}{
		{
			name: "valid",
			item: LaybyeItem{LaybyeID: 1, ItemCode: "ITEM001", Quantity: 2},
		},
		{
			name:    "zero laybye_id",
			item:    LaybyeItem{ItemCode: "ITEM001", Quantity: 2},
			wantErr: true,
		},
		{
			name:    "empty item_code",
			item:    LaybyeItem{LaybyeID: 1, Quantity: 2},
			wantErr: true,
		},
		{
			name:    "zero quantity",
			item:    LaybyeItem{LaybyeID: 1, ItemCode: "ITEM001", Quantity: 0},
			wantErr: true,
		},
		{
			name:    "negative quantity",
			item:    LaybyeItem{LaybyeID: 1, ItemCode: "ITEM001", Quantity: -1},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.item.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestApplyProduct(t *testing.T) {
	tests := []struct {
		name         string
		item         LaybyeItem
		productStub  func(*products.StockMaster, context.Context) error
		settingsStub func() (variables.PosSettings, error)
		wantErr      bool
		wantName     string
		wantPrice    float64
		wantTotal    float64
		wantVat      float64
	}{
		{
			name:         "price tag on — uses product till price",
			item:         LaybyeItem{ItemCode: "ITEM001", Quantity: 2},
			productStub:  stubProduct("Widget", 50, 100, 16, "A", false),
			settingsStub: stubSettings(true),
			wantName:     "Widget",
			wantPrice:    100,
			wantTotal:    200,
			wantVat:      16.0 * 200.0 / 116.0,
		},
		{
			name:         "price tag off — uses caller price at till price",
			item:         LaybyeItem{ItemCode: "ITEM001", Quantity: 2, Price: 100},
			productStub:  stubProduct("Widget", 50, 100, 16, "A", false),
			settingsStub: stubSettings(false),
			wantName:     "Widget",
			wantPrice:    100,
			wantTotal:    200,
			wantVat:      16.0 * 200.0 / 116.0,
		},
		{
			name:         "price tag off — uses caller price above till price",
			item:         LaybyeItem{ItemCode: "ITEM001", Quantity: 2, Price: 120},
			productStub:  stubProduct("Widget", 50, 100, 16, "A", false),
			settingsStub: stubSettings(false),
			wantName:     "Widget",
			wantPrice:    120,
			wantTotal:    240,
			wantVat:      16.0 * 240.0 / 116.0,
		},
		{
			name:         "settings fetch error",
			item:         LaybyeItem{ItemCode: "ITEM001", Quantity: 1},
			productStub:  stubProduct("Widget", 50, 100, 16, "A", false),
			settingsStub: func() (variables.PosSettings, error) { return variables.PosSettings{}, fmt.Errorf("db error") },
			wantErr:      true,
		},
		{
			name:         "product fetch error",
			item:         LaybyeItem{ItemCode: "ITEM001", Quantity: 1},
			productStub:  func(p *products.StockMaster, _ context.Context) error { return fmt.Errorf("rpc error") },
			settingsStub: stubSettings(true),
			wantErr:      true,
		},
		{
			name:         "price tag off, no price supplied",
			item:         LaybyeItem{ItemCode: "ITEM001", Quantity: 1, Price: 0},
			productStub:  stubProduct("Widget", 50, 100, 16, "A", false),
			settingsStub: stubSettings(false),
			wantErr:      true,
		},
		{
			name:         "price tag off, price below till price",
			item:         LaybyeItem{ItemCode: "ITEM001", Quantity: 1, Price: 90},
			productStub:  stubProduct("Widget", 50, 100, 16, "A", false),
			settingsStub: stubSettings(false),
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origProduct := fetchProduct
			fetchProduct = tt.productStub
			defer func() { fetchProduct = origProduct }()

			origSettings := fetchSettings
			fetchSettings = tt.settingsStub
			defer func() { fetchSettings = origSettings }()

			err := tt.item.applyProduct(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("applyProduct() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if tt.item.ItemName != tt.wantName {
				t.Errorf("ItemName = %q, want %q", tt.item.ItemName, tt.wantName)
			}
			if tt.item.Price != tt.wantPrice {
				t.Errorf("Price = %v, want %v", tt.item.Price, tt.wantPrice)
			}
			if tt.item.Total != tt.wantTotal {
				t.Errorf("Total = %v, want %v", tt.item.Total, tt.wantTotal)
			}
			if math.Abs(tt.item.Vat-tt.wantVat) > 0.001 {
				t.Errorf("Vat = %v, want ≈ %v", tt.item.Vat, tt.wantVat)
			}
		})
	}
}

func TestInsert(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		item    LaybyeItem
		setup   func(m pgxmock.PgxPoolIface)
		wantErr bool
		wantID  int64
	}{
		{
			name: "success",
			item: LaybyeItem{
				LaybyeID: 1, ItemCode: "ITEM001", ItemName: "Widget",
				Quantity: 2, Cost: 50, Price: 100,
				Total: 200, Vat: 27.59, VatAlpha: "A",
			},
			setup: func(m pgxmock.PgxPoolIface) {
				m.ExpectQuery(`INSERT INTO laybye_items`).
					WithArgs(anyArgs(insertArgs)...).
					WillReturnRows(m.NewRows([]string{"item_id", "trans_date"}).AddRow(int64(42), now))
			},
			wantID: 42,
		},
		{
			name: "db error",
			item: LaybyeItem{LaybyeID: 1, ItemCode: "ITEM001"},
			setup: func(m pgxmock.PgxPoolIface) {
				m.ExpectQuery(`INSERT INTO laybye_items`).
					WithArgs(anyArgs(insertArgs)...).
					WillReturnError(fmt.Errorf("connection reset"))
			},
			wantErr: true,
		},
		{
			name: "no rows returned",
			item: LaybyeItem{LaybyeID: 1, ItemCode: "ITEM001"},
			setup: func(m pgxmock.PgxPoolIface) {
				m.ExpectQuery(`INSERT INTO laybye_items`).
					WithArgs(anyArgs(insertArgs)...).
					WillReturnRows(m.NewRows([]string{"item_id", "trans_date"}))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, tx := newMockAndTx(t)
			defer mock.Close()
			tt.setup(mock)

			err := tt.item.insert(context.Background(), tx)
			if (err != nil) != tt.wantErr {
				t.Errorf("insert() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.item.ItemID != tt.wantID {
				t.Errorf("ItemID = %d, want %d", tt.item.ItemID, tt.wantID)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled mock expectations: %s", err)
			}
		})
	}
}

func TestActivateLaybye(t *testing.T) {
	tests := []struct {
		name     string
		laybyeID int64
		setup    func(m pgxmock.PgxPoolIface)
		wantErr  bool
	}{
		{
			name:     "success",
			laybyeID: 1,
			setup: func(m pgxmock.PgxPoolIface) {
				m.ExpectExec(`UPDATE laybyes`).
					WithArgs(int64(1)).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			},
		},
		{
			name:     "db error",
			laybyeID: 1,
			setup: func(m pgxmock.PgxPoolIface) {
				m.ExpectExec(`UPDATE laybyes`).
					WithArgs(int64(1)).
					WillReturnError(fmt.Errorf("connection reset"))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, tx := newMockAndTx(t)
			defer mock.Close()
			tt.setup(mock)

			err := activateLaybye(context.Background(), tx, tt.laybyeID)
			if (err != nil) != tt.wantErr {
				t.Errorf("activateLaybye() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled mock expectations: %s", err)
			}
		})
	}
}

func TestAddItem(t *testing.T) {
	now := time.Now()

	validItem := func() LaybyeItem {
		return LaybyeItem{LaybyeID: 1, ItemCode: "ITEM001", Quantity: 2}
	}

	tests := []struct {
		name         string
		item         LaybyeItem
		productStub  func(*products.StockMaster, context.Context) error
		settingsStub func() (variables.PosSettings, error)
		setup        func(m pgxmock.PgxPoolIface) pgx.Tx
		wantErr      bool
		wantItemID   int64
	}{
		{
			name:         "success with price tag on",
			item:         validItem(),
			productStub:  stubProduct("Widget", 50, 100, 16, "A", false),
			settingsStub: stubSettings(true),
			setup: func(m pgxmock.PgxPoolIface) pgx.Tx {
				m.ExpectBegin()
				m.ExpectQuery(`INSERT INTO laybye_items`).
					WithArgs(anyArgs(insertArgs)...).
					WillReturnRows(m.NewRows([]string{"item_id", "trans_date"}).AddRow(int64(42), now))
				m.ExpectExec(`UPDATE laybyes`).
					WithArgs(int64(1)).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
				m.ExpectCommit()
				tx, _ := m.BeginTx(context.Background(), pgx.TxOptions{})
				return tx
			},
			wantItemID: 42,
		}, {
			name:         "success with price tag off, price at till price",
			item:         LaybyeItem{LaybyeID: 1, ItemCode: "ITEM001", Quantity: 2, Price: 100},
			productStub:  stubProduct("Widget", 50, 100, 16, "A", false),
			settingsStub: stubSettings(false),
			setup: func(m pgxmock.PgxPoolIface) pgx.Tx {
				m.ExpectBegin()
				m.ExpectQuery(`INSERT INTO laybye_items`).
					WithArgs(anyArgs(insertArgs)...).
					WillReturnRows(m.NewRows([]string{"item_id", "trans_date"}).AddRow(int64(55), now))
				m.ExpectExec(`UPDATE laybyes`).
					WithArgs(int64(1)).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
				m.ExpectCommit()
				tx, _ := m.BeginTx(context.Background(), pgx.TxOptions{})
				return tx
			},
			wantItemID: 55,
		}, {
			name:         "success with price tag off, price above till price",
			item:         LaybyeItem{LaybyeID: 1, ItemCode: "ITEM001", Quantity: 2, Price: 120},
			productStub:  stubProduct("Widget", 50, 100, 16, "A", false),
			settingsStub: stubSettings(false),
			setup: func(m pgxmock.PgxPoolIface) pgx.Tx {
				m.ExpectBegin()
				m.ExpectQuery(`INSERT INTO laybye_items`).
					WithArgs(anyArgs(insertArgs)...).
					WillReturnRows(m.NewRows([]string{"item_id", "trans_date"}).AddRow(int64(66), now))
				m.ExpectExec(`UPDATE laybyes`).
					WithArgs(int64(1)).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
				m.ExpectCommit()
				tx, _ := m.BeginTx(context.Background(), pgx.TxOptions{})
				return tx
			},
			wantItemID: 66,
		}, {
			name:    "zero laybye_id",
			item:    LaybyeItem{ItemCode: "ITEM001", Quantity: 2},
			setup:   func(m pgxmock.PgxPoolIface) pgx.Tx { return nil },
			wantErr: true,
		}, {
			name:    "empty item_code",
			item:    LaybyeItem{LaybyeID: 1, Quantity: 2},
			setup:   func(m pgxmock.PgxPoolIface) pgx.Tx { return nil },
			wantErr: true,
		}, {
			name:    "zero quantity",
			item:    LaybyeItem{LaybyeID: 1, ItemCode: "ITEM001"},
			setup:   func(m pgxmock.PgxPoolIface) pgx.Tx { return nil },
			wantErr: true,
		}, {
			name:         "settings fetch error",
			item:         validItem(),
			productStub:  stubProduct("Widget", 50, 100, 16, "A", false),
			settingsStub: func() (variables.PosSettings, error) { return variables.PosSettings{}, fmt.Errorf("db error") },
			setup: func(m pgxmock.PgxPoolIface) pgx.Tx {
				m.ExpectBegin()
				tx, _ := m.BeginTx(context.Background(), pgx.TxOptions{})
				return tx
			},
			wantErr: true,
		}, {
			name:         "product fetch error",
			item:         validItem(),
			productStub:  func(p *products.StockMaster, _ context.Context) error { return fmt.Errorf("rpc error") },
			settingsStub: stubSettings(true),
			setup: func(m pgxmock.PgxPoolIface) pgx.Tx {
				m.ExpectBegin()
				tx, _ := m.BeginTx(context.Background(), pgx.TxOptions{})
				return tx
			},
			wantErr: true,
		}, {
			name:         "price tag off, no price supplied",
			item:         LaybyeItem{LaybyeID: 1, ItemCode: "ITEM001", Quantity: 2, Price: 0},
			productStub:  stubProduct("Widget", 50, 100, 16, "A", false),
			settingsStub: stubSettings(false),
			setup: func(m pgxmock.PgxPoolIface) pgx.Tx {
				m.ExpectBegin()
				tx, _ := m.BeginTx(context.Background(), pgx.TxOptions{})
				return tx
			},
			wantErr: true,
		}, {
			name:         "price tag off, price below till price",
			item:         LaybyeItem{LaybyeID: 1, ItemCode: "ITEM001", Quantity: 2, Price: 90},
			productStub:  stubProduct("Widget", 50, 100, 16, "A", false),
			settingsStub: stubSettings(false),
			setup: func(m pgxmock.PgxPoolIface) pgx.Tx {
				m.ExpectBegin()
				tx, _ := m.BeginTx(context.Background(), pgx.TxOptions{})
				return tx
			},
			wantErr: true,
		}, {
			name:         "insert error",
			item:         validItem(),
			productStub:  stubProduct("Widget", 50, 100, 16, "A", false),
			settingsStub: stubSettings(true),
			setup: func(m pgxmock.PgxPoolIface) pgx.Tx {
				m.ExpectBegin()
				m.ExpectQuery(`INSERT INTO laybye_items`).
					WithArgs(anyArgs(insertArgs)...).
					WillReturnError(fmt.Errorf("unique violation"))
				tx, _ := m.BeginTx(context.Background(), pgx.TxOptions{})
				return tx
			},
			wantErr: true,
		}, {
			name:         "activate error",
			item:         validItem(),
			productStub:  stubProduct("Widget", 50, 100, 16, "A", false),
			settingsStub: stubSettings(true),
			setup: func(m pgxmock.PgxPoolIface) pgx.Tx {
				m.ExpectBegin()
				m.ExpectQuery(`INSERT INTO laybye_items`).
					WithArgs(anyArgs(insertArgs)...).
					WillReturnRows(m.NewRows([]string{"item_id", "trans_date"}).AddRow(int64(42), now))
				m.ExpectExec(`UPDATE laybyes`).
					WithArgs(int64(1)).
					WillReturnError(fmt.Errorf("connection reset"))
				tx, _ := m.BeginTx(context.Background(), pgx.TxOptions{})
				return tx
			},
			wantErr: true,
		}, {
			name:         "commit error",
			item:         validItem(),
			productStub:  stubProduct("Widget", 50, 100, 16, "A", false),
			settingsStub: stubSettings(true),
			setup: func(m pgxmock.PgxPoolIface) pgx.Tx {
				m.ExpectBegin()
				m.ExpectQuery(`INSERT INTO laybye_items`).
					WithArgs(anyArgs(insertArgs)...).
					WillReturnRows(m.NewRows([]string{"item_id", "trans_date"}).AddRow(int64(42), now))
				m.ExpectExec(`UPDATE laybyes`).
					WithArgs(int64(1)).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
				m.ExpectCommit().WillReturnError(fmt.Errorf("commit failed"))
				tx, _ := m.BeginTx(context.Background(), pgx.TxOptions{})
				return tx
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.productStub != nil {
				orig := fetchProduct
				fetchProduct = tt.productStub
				defer func() { fetchProduct = orig }()
			}
			if tt.settingsStub != nil {
				orig := fetchSettings
				fetchSettings = tt.settingsStub
				defer func() { fetchSettings = orig }()
			}

			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatalf("pgxmock.NewPool: %v", err)
			}
			defer mock.Close()

			tx := tt.setup(mock)

			err = tt.item.AddItem(context.Background(), tx)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddItem() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.item.ItemID != tt.wantItemID {
				t.Errorf("ItemID = %d, want %d", tt.item.ItemID, tt.wantItemID)
			}
			if merr := mock.ExpectationsWereMet(); merr != nil {
				t.Errorf("unfulfilled mock expectations: %s", merr)
			}
		})
	}
}
