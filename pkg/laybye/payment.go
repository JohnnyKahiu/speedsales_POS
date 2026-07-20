package laybye

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/broker"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/sales"
	"github.com/jackc/pgx/v5"
)

// Payment is a single installment payment against a laybye.
type Payment struct {
	LaybyeID    int64   `json:"laybye_id"`
	TillNum     int64   `json:"till_num"`
	Cash        float64 `json:"cash"`
	Mpesa       float64 `json:"mpesa"`
	Ecard       float64 `json:"ecard"`
	Cheque      float64 `json:"cheque"`
	Branch      string  `json:"-"`
	StkLocation string  `json:"-"`
}

func (p *Payment) tendered() float64 {
	return p.Cash + p.Mpesa + p.Ecard + p.Cheque
}

// outstanding returns the total owed minus total already paid for a laybye.
func outstanding(ctx context.Context, laybyeID int64) (float64, error) {
	sql := `
		SELECT
			COALESCE((SELECT SUM(total) FROM laybye_items WHERE laybye_id = $1 AND state != 'DELETED'), 0)
			- COALESCE((SELECT SUM(cash + mpesa + ecard + cheque) FROM till_payments WHERE laybye_id = $1 AND payment_for = 'laybye_payment'), 0)
	`

	var bal float64
	if err := database.PgPool.QueryRow(ctx, sql, laybyeID).Scan(&bal); err != nil {
		return 0, err
	}
	return bal, nil
}

// Pay records an installment payment against a laybye. If the payment clears the
// outstanding balance, the laybye is marked completed and a sales_orders event is
// published so Inventory releases stock for its items — individual installments
// never move stock on their own.
func (p *Payment) Pay(ctxt context.Context) (*sales.TillPayment, error) {
	if p.LaybyeID == 0 {
		return nil, fmt.Errorf("laybye_id is required")
	}
	if p.tendered() <= 0 {
		return nil, fmt.Errorf("payment amount must be greater than zero")
	}

	ctx, cancel := context.WithTimeout(ctxt, 20*time.Second)
	defer cancel()

	var state string
	if err := database.PgPool.QueryRow(ctx, `SELECT state FROM laybyes WHERE laybye_id = $1`, p.LaybyeID).Scan(&state); err != nil {
		return nil, fmt.Errorf("laybye not found: %w", err)
	}
	if state == "completed" || state == "cancelled" {
		return nil, fmt.Errorf("laybye is %s, no further payments accepted", state)
	}

	balance, err := outstanding(ctx, p.LaybyeID)
	if err != nil {
		return nil, fmt.Errorf("failed to compute outstanding balance: %w", err)
	}
	if balance <= 0 {
		return nil, fmt.Errorf("laybye has no outstanding balance")
	}
	if p.tendered() > balance+0.01 {
		return nil, fmt.Errorf("payment of %.2f exceeds outstanding balance of %.2f", p.tendered(), balance)
	}

	tp := sales.TillPayment{
		PaymentFor: "laybye_payment",
		TillNum:    p.TillNum,
		LaybyeID:   p.LaybyeID,
		Cash:       p.Cash,
		Mpesa:      p.Mpesa,
		Ecard:      p.Ecard,
		Cheque:     p.Cheque,
	}
	if err := tp.RecordAndPublish(ctx); err != nil {
		return nil, fmt.Errorf("failed to record payment: %w", err)
	}

	if remaining := balance - p.tendered(); remaining <= 0.01 {
		if err := p.complete(ctx); err != nil {
			log.Println("error. failed to complete laybye     err =", err)
		}
	}

	return &tp, nil
}

// complete marks the laybye as completed and publishes its items as a sales_orders
// event so Inventory can release stock now that the full balance has been paid.
func (p *Payment) complete(ctx context.Context) error {
	tx, err := database.PgPool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE laybyes SET state = 'completed' WHERE laybye_id = $1`, p.LaybyeID); err != nil {
		return err
	}

	items, err := fetchItems(ctx, tx, p.LaybyeID)
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	p.publishSalesOrder(items)
	return nil
}

func fetchItems(ctx context.Context, tx pgx.Tx, laybyeID int64) ([]sales.Sales, error) {
	sql := `
		SELECT item_code, item_name, quantity, cost, price, discount, total, vat, vat_alpha, on_offer
		FROM laybye_items
		WHERE laybye_id = $1 AND state != 'DELETED'
	`

	rows, err := tx.Query(ctx, sql, laybyeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []sales.Sales
	for rows.Next() {
		var s sales.Sales
		if err := rows.Scan(&s.ItemCode, &s.ItemName, &s.Quantity, &s.Cost, &s.Price, &s.Discount, &s.Total, &s.Vat, &s.VatAlpha, &s.OnOffer); err != nil {
			return nil, err
		}
		s.State = "COMPLETED"
		s.ReceiptItem = fmt.Sprintf("laybye-%d-%s", laybyeID, s.ItemCode)
		items = append(items, s)
	}

	return items, nil
}

func (p *Payment) publishSalesOrder(items []sales.Sales) {
	ord := sales.Order{
		TransDate:    time.Now(),
		CompleteTime: time.Now(),
		OrderNum:     p.LaybyeID,
		ReceiptNum:   p.LaybyeID,
		OrderItems:   items,
		Branch:       p.Branch,
		StkLocation:  p.StkLocation,
		TillNum:      p.TillNum,
		State:        "laybye_completed",
	}

	payload, err := json.Marshal(ord)
	if err != nil {
		log.Println("error. failed to marshal laybye completion order    err =", err)
		return
	}

	kf := broker.Kafka{
		Broker:  os.Getenv("KAFKA_BROKER"),
		Topic:   "sales_orders",
		Key:     fmt.Sprintf("%v", ord.OrderNum),
		Payload: payload,
	}
	if err := kf.Produce(context.Background()); err != nil {
		log.Println("kafka error    failed to publish sales_orders for laybye completion    err =", err)
	}
}
