package reports

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
)

type Product struct {
	ItemCode string  `json:"item_code"`
	ItemName string  `json:"item_name"`
	Quantity float64 `json:"quantity"`
	Sales    float64 `json:"sales"`
	Cost     float64 `json:"cost"`
	Margin   float64 `json:"margin"`
	Start    string  `json:"start"`
	End      string  `json:"end"`
	Teller   string  `json:"teller"`
}

// FetchProductSales - fetches sales by products
// queries sales from salestrace
// returns a slice of products and an error if it fails
func (arg *Product) FetchProductSales(ctxt context.Context) ([]Product, error) {
	tellerCon := ""
	if arg.Teller == "" {
		tellerCon = "AND s.poster = '" + strings.Replace(arg.Teller, "'", "|| chr(39) ||", -1) + "'"
	}

	codeCon := ""
	if arg.ItemCode == "" {
		codeCon = "AND sales.item_code = '" + strings.Replace(arg.ItemCode, "'", "|| chr(39) ||", -1) + "'"
	}

	sql := fmt.Sprintf(`
		SELECT
			a.item_code
			, a.item_name
			, SUM(a.quantity) as qty
			, SUM(a.quantity * a.price) as total
			, SUM((a.price - a.cost)* a.quantity)
		FROM
			(
				SELECT 
					sales.*
				FROM salestrace s, jsonb_to_recordset(s.cart) as sales(
					trans_date timestamptz,
					item_code text,
					item_name text,
					quantity float,
					price float,
					cost float,
					state text
			)
			WHERE s.state IN ('PAID', 'POSTED', 'CREDITED') AND sales.state = 'pending'
				AND sales.trans_date::date >= $1 AND sales.trans_date::date <= $2  %v %v
			GROUP BY sales.item_code, sales.item_name) as a
		GROUP BY a.item_code, a.item_name`, tellerCon, codeCon)

	ctx, cancel := context.WithTimeout(ctxt, 20*time.Second)
	defer cancel()

	rows, err := database.PgPool.Query(ctx, sql, arg.Start, arg.End)
	if err != nil {
		log.Println("postgresql error.  failed to query products sales       err =", err)
		return []Product{}, err
	}
	defer rows.Close()

	vals := []Product{}
	for rows.Next() {
		r := Product{}
		err = rows.Scan(&r.ItemCode, &r.ItemName, &r.Quantity, &r.Sales, &r.Margin)
		if err != nil {
			return vals, err
		}
		vals = append(vals, r)
	}
	return []Product{}, nil
}
