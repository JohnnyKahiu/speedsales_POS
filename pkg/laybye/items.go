package laybye

import (
	"context"
	"fmt"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/products"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/variables"
	"github.com/jackc/pgx/v5"
)

// fetchProduct is overridable in tests to avoid a live inventory gRPC call.
var fetchProduct = func(p *products.StockMaster, ctx context.Context) error {
	return p.Fetch(ctx)
}

// fetchSettings is overridable in tests to avoid a live DB call.
var fetchSettings = func() (variables.PosSettings, error) {
	s, err := variables.SysDefaults()
	return s.PosDefaults, err
}

type LaybyeItem struct {
	table     string    `name:"laybye_items" type:"table"`
	ItemID    int64     `json:"item_id"    type:"field" sql:"BIGSERIAL PRIMARY KEY"`
	LaybyeID  int64     `json:"laybye_id"  type:"field" sql:"BIGINT NOT NULL"`
	TransDate time.Time `json:"trans_date" type:"field" sql:"TIMESTAMPTZ NOT NULL DEFAULT now()"`
	ItemCode  string    `json:"item_code"  type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	ItemName  string    `json:"item_name"  type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	Quantity  float64   `json:"quantity"   type:"field" sql:"FLOAT NOT NULL DEFAULT '0'"`
	Cost      float64   `json:"cost"       type:"field" sql:"FLOAT NOT NULL DEFAULT '0'"`
	Price     float64   `json:"price"      type:"field" sql:"FLOAT NOT NULL DEFAULT '0'"`
	Discount  float64   `json:"discount"   type:"field" sql:"FLOAT NOT NULL DEFAULT '0'"`
	Total     float64   `json:"total"      type:"field" sql:"FLOAT NOT NULL DEFAULT '0'"`
	Vat       float64   `json:"vat"        type:"field" sql:"FLOAT NOT NULL DEFAULT '0'"`
	VatAlpha  string    `json:"vat_alpha"  type:"field" sql:"VARCHAR(1) NOT NULL DEFAULT ''"`
	OnOffer   bool      `json:"on_offer"   type:"field" sql:"BOOL NOT NULL DEFAULT 'false'"`
	State     string    `json:"state"      type:"field" sql:"VARCHAR NOT NULL DEFAULT 'active'"`
}

func GenItemsTable() error {
	return database.CreateFromStruct(LaybyeItem{})
}

// validate checks for nulls and
// returns an error if it fails
func (arg *LaybyeItem) validate() error {
	if arg.LaybyeID == 0 {
		return fmt.Errorf("laybye_id is required")
	}
	if arg.ItemCode == "" {
		return fmt.Errorf("item code is required")
	}
	if arg.Quantity <= 0 {
		return fmt.Errorf("quantity must be greater than zero")
	}
	return nil
}

// applyProduct populates product details in LaybyeItem
// returns an error if it fails
func (arg *LaybyeItem) applyProduct(ctx context.Context) error {
	settings, err := fetchSettings()
	if err != nil {
		return fmt.Errorf("failed to fetch pos settings: %w", err)
	}

	p := products.StockMaster{ItemCode: arg.ItemCode}
	if err := fetchProduct(&p, ctx); err != nil {
		return fmt.Errorf("failed to fetch product: %w", err)
	}

	arg.ItemName = p.ItemName
	arg.Cost = p.ItemCost
	arg.OnOffer = p.OnOffer
	arg.VatAlpha = p.VatAlpha

	if settings.PriceTag {
		arg.Price = p.TillPrice
	} else {
		if arg.Price <= 0 {
			return fmt.Errorf("price is required when price tag is disabled")
		}
		if arg.Price < p.TillPrice {
			return fmt.Errorf("price is below minimum sale price")
		}
	}

	arg.Total = arg.Quantity * arg.Price
	arg.Vat = p.VatPercent * arg.Total / (100 + p.VatPercent)
	return nil
}

// insert adds laybyeItem data to database
// populates item_id and trans_date
// returns an error if it fails
func (arg *LaybyeItem) insert(ctx context.Context, tx pgx.Tx) error {
	sql := `INSERT INTO laybye_items(laybye_id, item_code, item_name, quantity, cost, price, discount, total, vat, vat_alpha, on_offer)
			VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING item_id, trans_date`

	return tx.QueryRow(ctx, sql,
		arg.LaybyeID, arg.ItemCode, arg.ItemName,
		arg.Quantity, arg.Cost, arg.Price, arg.Discount,
		arg.Total, arg.Vat, arg.VatAlpha, arg.OnOffer,
	).Scan(&arg.ItemID, &arg.TransDate)
}

// activateLaybye sets laybye detail as active
// updates laybyes state to active
// returns an error if it fails
func activateLaybye(ctx context.Context, tx pgx.Tx, laybyeID int64) error {
	_, err := tx.Exec(ctx,
		`UPDATE laybyes SET state = 'active' WHERE laybye_id = $1`,
		laybyeID,
	)
	return err
}

// AddItem validates, fetches product pricing, inserts the item, and activates
// the parent laybye. The caller owns BeginTx and defer Rollback; AddItem commits.
func (arg *LaybyeItem) AddItem(ctxt context.Context, tx pgx.Tx) error {
	if err := arg.validate(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctxt, 15*time.Second)
	defer cancel()

	if err := arg.applyProduct(ctx); err != nil {
		return err
	}

	if err := arg.insert(ctx, tx); err != nil {
		return fmt.Errorf("failed to add laybye item: %w", err)
	}

	if err := activateLaybye(ctx, tx, arg.LaybyeID); err != nil {
		return fmt.Errorf("failed to activate laybye: %w", err)
	}

	return tx.Commit(ctx)
}
