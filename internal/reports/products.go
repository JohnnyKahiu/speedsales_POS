package reports

import (
	"context"
	"log"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
)

// Product holds aggregated sales-by-product totals for a date range.
type Product struct {
	ItemCode string  `json:"item_code"`
	ItemName string  `json:"item_name"`
	Quantity float64 `json:"quantity"`
	Sales    float64 `json:"sales"`
	Cost     float64 `json:"cost"`
	Margin   float64 `json:"margin"`
}

// FetchProductSales returns quantity/value/cost/margin totals per item sold
// within [start, end], optionally filtered by cashier (teller) and by an
// item_name search term (pass empty strings to skip a filter).
func FetchProductSales(ctxt context.Context, start, end, teller, search string) ([]Product, error) {
	sql := `
		SELECT
			a.item_code
			, a.item_name
			, SUM(a.quantity)                          AS qty
			, SUM(a.quantity * a.price)                AS sales_value
			, SUM(a.quantity * a.cost)                  AS cost_value
			, SUM(a.quantity * (a.price - a.cost))      AS margin
		FROM (
			SELECT sales.*
			FROM salestrace s, jsonb_to_recordset(s.cart) AS sales(
				trans_date timestamptz,
				item_code text,
				item_name text,
				quantity float,
				price float,
				cost float,
				state text
			)
			WHERE s.state IN ('PAID', 'POSTED', 'CREDITED')
				AND sales.state = 'pending'
				AND sales.trans_date::date >= $1 AND sales.trans_date::date <= $2
				--AND ($3 = '' OR s.poster = $3)
				--AND ($4 = '' OR sales.item_name ILIKE '%' || $4 || '%')
		) a
		GROUP BY a.item_code, a.item_name
		ORDER BY a.item_name`

	ctx, cancel := context.WithTimeout(ctxt, 20*time.Second)
	defer cancel()

	rows, err := database.PgPool.Query(ctx, sql, start, end)
	if err != nil {
		log.Println("postgresql error. failed to query products sales    err =", err)
		return nil, err
	}
	defer rows.Close()

	vals := []Product{}
	for rows.Next() {
		r := Product{}
		if err := rows.Scan(&r.ItemCode, &r.ItemName, &r.Quantity, &r.Sales, &r.Cost, &r.Margin); err != nil {
			log.Println("postgresql error. failed to scan product sales row    err =", err)
			return nil, err
		}
		vals = append(vals, r)
	}
	return vals, nil
}
