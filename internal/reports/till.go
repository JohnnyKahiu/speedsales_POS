package reports

import (
	"context"
	"log"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
)

// TillReport holds the aggregated EOD summary for one till / cashier.
type TillReport struct {
	TillNo          int64   `json:"till_no"`
	Teller          string  `json:"teller"`
	OpenFloat       float64 `json:"open_float"`
	CloseCash       float64 `json:"close_cash"`
	// raw payment method totals from cash sales
	CashSale        float64 `json:"cash_sale"`
	MpesaSale       float64 `json:"mpesa_sale"`
	CardSale        float64 `json:"card_sale"`
	ChequeSale      float64 `json:"cheque_sale"`
	VoucherSale     float64 `json:"voucher_sale"`
	TotalCashSale   float64 `json:"total_cash_sale"`
	TotalCreditSale float64 `json:"total_credit_sale"`
	SalesReturn     float64 `json:"sales_return"`
	// derived totals (cash sale + credit_pay + laybye_pay per method)
	CashIn      float64 `json:"cash_in"`
	MpesaIn     float64 `json:"mpesa_in"`
	CardIn      float64 `json:"card_in"`
	ChequeIn    float64 `json:"cheque_in"`
	CashSummary float64 `json:"cash_summary"`
	Balance     float64 `json:"balance"`
}

// FetchTillReport returns EOD summary rows filtered by date range and optionally
// by cashier username or till number (pass empty/0 to include all).
func FetchTillReport(ctxt context.Context, start, end, cashier string, tillNo int64) ([]TillReport, error) {
	sql := `
	WITH
	cash_sales AS (
		SELECT
			pay_till AS till_num
			, COALESCE(SUM(CAST(pay_details::json->>'cash'    AS float)), 0) AS cash
			, COALESCE(SUM(CAST(pay_details::json->>'mpesa'   AS float)), 0) AS mpesa
			, COALESCE(SUM(CAST(pay_details::json->>'ecard'   AS float)), 0) AS ecard
			, COALESCE(SUM(CAST(pay_details::json->>'check'   AS float)), 0) AS cheque
			, COALESCE(SUM(CAST(pay_details::json->>'voucher' AS float)), 0) AS voucher
			, COALESCE(SUM(total), 0) AS total_cash_sale
		FROM salestrace
		WHERE state IN ('POSTED', 'PAID')
			AND sale_type = 'Cash Sale'
			AND trans_date::date >= $1 AND trans_date::date <= $2
		GROUP BY pay_till
	),
	returns AS (
		SELECT
			pay_till AS till_num
			, COALESCE(SUM(total), 0) AS amount
		FROM salestrace
		WHERE state = 'RETURN'
			AND trans_date::date >= $1 AND trans_date::date <= $2
		GROUP BY pay_till
	),
	credit_sales AS (
		SELECT
			pay_till AS till_num
			, COALESCE(SUM(total), 0) AS total
		FROM salestrace
		WHERE state IN ('DEBITED', 'CREDITED')
			AND trans_date::date >= $1 AND trans_date::date <= $2
		GROUP BY pay_till
	),
	credit_pays AS (
		SELECT
			till_num
			, COALESCE(SUM(CAST(pay_details->'cash'  AS varchar)::float), 0) AS cash
			, COALESCE(SUM(CAST(pay_details->'mpesa' AS varchar)::float), 0) AS mpesa
			, COALESCE(SUM(CAST(pay_details->'ecard' AS varchar)::float), 0) AS ecard
			, COALESCE(SUM(CAST(pay_details->'check' AS varchar)::float), 0) AS cheque
		FROM accounts_txn
		WHERE trans_date::date >= $1 AND trans_date::date <= $2
		GROUP BY till_num
	),
	laybye_pays AS (
		SELECT
			till_num
			, COALESCE(SUM(CASE WHEN pay_type = 'cash'   THEN amount_paid END), 0) AS cash
			, COALESCE(SUM(CASE WHEN pay_type = 'mpesa'  THEN amount_paid END), 0) AS mpesa
			, COALESCE(SUM(CASE WHEN pay_type = 'ecard'  THEN amount_paid END), 0) AS ecard
			, COALESCE(SUM(CASE WHEN pay_type = 'cheque' THEN amount_paid END), 0) AS cheque
		FROM laybye_trans
		WHERE trans_type = 'payment'
			AND trans_date::date >= $1 AND trans_date::date <= $2
		GROUP BY till_num
	)
	SELECT
		st.till_no
		, st.teller
		, COALESCE(st.open_float, 0)            AS open_float
		, COALESCE(st.close_cash, 0)             AS close_cash
		, COALESCE(cs.cash, 0)                   AS cash_sale
		, COALESCE(cs.mpesa, 0)                  AS mpesa_sale
		, COALESCE(cs.ecard, 0)                  AS card_sale
		, COALESCE(cs.cheque, 0)                 AS cheque_sale
		, COALESCE(cs.voucher, 0)                AS voucher_sale
		, COALESCE(cs.total_cash_sale, 0)        AS total_cash_sale
		, COALESCE(ret.amount, 0)                AS sales_return
		, COALESCE(crs.total, 0)                 AS total_credit_sale
		, COALESCE(cp.cash, 0)                   AS credit_cash
		, COALESCE(cp.mpesa, 0)                  AS credit_mpesa
		, COALESCE(cp.ecard, 0)                  AS credit_ecard
		, COALESCE(cp.cheque, 0)                 AS credit_cheque
		, COALESCE(lp.cash, 0)                   AS laybye_cash
		, COALESCE(lp.mpesa, 0)                  AS laybye_mpesa
		, COALESCE(lp.ecard, 0)                  AS laybye_ecard
		, COALESCE(lp.cheque, 0)                 AS laybye_cheque
	FROM sales_till st
		LEFT JOIN cash_sales    cs  ON cs.till_num  = st.till_no
		LEFT JOIN returns       ret ON ret.till_num = st.till_no
		LEFT JOIN credit_sales  crs ON crs.till_num = st.till_no
		LEFT JOIN credit_pays   cp  ON cp.till_num  = st.till_no
		LEFT JOIN laybye_pays   lp  ON lp.till_num  = st.till_no
	WHERE
		($3 = '' OR st.teller   = $3)
		AND ($4 = 0 OR st.till_no = $4)
		AND st.open_time::date >= $1
	ORDER BY st.open_time ASC`

	ctx, cancel := context.WithTimeout(ctxt, 30*time.Second)
	defer cancel()

	rows, err := database.PgPool.Query(ctx, sql, start, end, cashier, tillNo)
	if err != nil {
		log.Println("FetchTillReport query error:", err)
		return nil, err
	}
	defer rows.Close()

	var results []TillReport
	for rows.Next() {
		var r TillReport
		var creditCash, creditMpesa, creditEcard, creditCheque float64
		var laybyCash, laybyMpesa, laybyEcard, laybyCheque float64

		if err := rows.Scan(
			&r.TillNo, &r.Teller, &r.OpenFloat, &r.CloseCash,
			&r.CashSale, &r.MpesaSale, &r.CardSale, &r.ChequeSale, &r.VoucherSale,
			&r.TotalCashSale, &r.SalesReturn, &r.TotalCreditSale,
			&creditCash, &creditMpesa, &creditEcard, &creditCheque,
			&laybyCash, &laybyMpesa, &laybyEcard, &laybyCheque,
		); err != nil {
			log.Println("FetchTillReport scan error:", err)
			return nil, err
		}

		// derived totals per payment method
		r.CashIn   = r.CashSale   + creditCash   + laybyCash
		r.MpesaIn  = r.MpesaSale  + creditMpesa  + laybyMpesa
		r.CardIn   = r.CardSale   + creditEcard  + laybyEcard
		r.ChequeIn = r.ChequeSale + creditCheque + laybyCheque

		r.CashSummary = r.CashIn + r.MpesaIn + r.CardIn + r.ChequeIn + r.VoucherSale - r.SalesReturn
		r.Balance = (r.OpenFloat - r.CloseCash) - r.CashIn

		results = append(results, r)
	}

	return results, nil
}
