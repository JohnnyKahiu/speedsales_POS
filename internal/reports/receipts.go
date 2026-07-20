package reports

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
)

// ReceiptSummary holds the data for one row in the receipts report.
type ReceiptSummary struct {
	TransDate  time.Time       `json:"trans_date"`
	ReceiptNum int64           `json:"receipt_num"`
	Poster     string          `json:"poster"`
	Total      float64         `json:"total"`
	CustName   string          `json:"cust_name"`
	Paymode    string          `json:"paymode"`
	Change     float64         `json:"change"`
	State      string          `json:"state"`
	Cart       json.RawMessage `json:"cart"`
	PayDetails json.RawMessage `json:"pay_details"`
}

// FetchReceiptList returns committed receipts within [start, end].
// Pass cashier="" to include all tellers (managers only).
func FetchReceiptList(ctxt context.Context, start, end, cashier string) ([]ReceiptSummary, error) {
	sql := `
		SELECT
			trans_date AT TIME ZONE 'utc' AT TIME ZONE 'eat'  AS trans_date
			, receipt_num
			, COALESCE(poster, '')
			, COALESCE(total, 0)
			, COALESCE(cust_name, 'walk in')
			, COALESCE(paymode, '')
			, COALESCE("change", 0)
			, state
			, COALESCE(cart::varchar, '[]')
			, COALESCE(pay_details::varchar, '{}')
		FROM salestrace
		WHERE trans_date::date >= $1
			AND trans_date::date <= $2
			AND state NOT IN ('pending', 'suspend')
			AND ($3 = '' OR poster = $3)
		ORDER BY trans_date DESC`

	ctx, cancel := context.WithTimeout(ctxt, 30*time.Second)
	defer cancel()

	rows, err := database.PgPool.Query(ctx, sql, start, end, cashier)
	if err != nil {
		log.Println("FetchReceiptList query error:", err)
		return nil, err
	}
	defer rows.Close()

	results := []ReceiptSummary{}
	for rows.Next() {
		var r ReceiptSummary
		cartStr := "[]"
		payStr := "{}"
		if err := rows.Scan(
			&r.TransDate, &r.ReceiptNum, &r.Poster, &r.Total,
			&r.CustName, &r.Paymode, &r.Change, &r.State,
			&cartStr, &payStr,
		); err != nil {
			log.Println("FetchReceiptList scan error:", err)
			return nil, err
		}
		if cartStr == "" {
			cartStr = "[]"
		}
		if payStr == "" {
			payStr = "{}"
		}
		r.Cart = json.RawMessage(cartStr)
		r.PayDetails = json.RawMessage(payStr)
		results = append(results, r)
	}

	return results, nil
}
