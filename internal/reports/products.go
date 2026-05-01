package reports

import "context"

type Product struct {
	ItemCode string  `json:"item_code"`
	ItemName string  `json:"item_name"`
	Quantity float64 `json:"quantity"`
	Sales    float64 `json:"sales"`
	Cost     float64 `json:"cost"`
	Margin   float64 `json:"margin"`
}

func FetchProductSales(ctxt context.Context, start, end string) ([]Product, error) {
	sql := `SELECT FROM WHERE `
}
