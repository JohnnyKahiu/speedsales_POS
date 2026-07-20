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
	CashRollups     float64 `json:"cash_rollups"`
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
	-- salestrace: totals only (voucher and grand total for cash sales)
	cash_sale_totals AS (
		SELECT
			pay_till AS till_num
			, COALESCE(SUM(CAST(pay_details::json->>'voucher' AS float)), 0) AS voucher
			, COALESCE(SUM(total), 0)                                        AS total_cash_sale
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
	-- till_payments is the single source of truth for all payment method amounts
	payments AS (
		SELECT
			till_num
			, payment_for
			, COALESCE(SUM(cash),   0) AS cash
			, COALESCE(SUM(mpesa),  0) AS mpesa
			, COALESCE(SUM(ecard),  0) AS ecard
			, COALESCE(SUM(cheque), 0) AS cheque
		FROM till_payments
		WHERE trans_time::date >= $1 AND trans_time::date <= $2
		GROUP BY till_num, payment_for
	),
	rollups AS (
		SELECT
			till_num
			, COALESCE(SUM(amount), 0) AS amount
		FROM cash_movement
		WHERE type = 'cash rollup'
			AND confirm_state = 'CONFIRMED'
			AND trans_date::date >= $1 AND trans_date::date <= $2
		GROUP BY till_num
	)
	SELECT
		st.till_no
		, st.teller
		, COALESCE(st.open_float, 0)             AS open_float
		, COALESCE(st.close_cash, 0)             AS close_cash
		, COALESCE(rl.amount, 0)                 AS cash_rollups
		, COALESCE(cs_pay.cash,   0)             AS cash_sale
		, COALESCE(cs_pay.mpesa,  0)             AS mpesa_sale
		, COALESCE(cs_pay.ecard,  0)             AS card_sale
		, COALESCE(cs_pay.cheque, 0)             AS cheque_sale
		, COALESCE(cst.voucher, 0)               AS voucher_sale
		, COALESCE(cst.total_cash_sale, 0)       AS total_cash_sale
		, COALESCE(ret.amount, 0)                AS sales_return
		, COALESCE(crs.total, 0)                 AS total_credit_sale
		, COALESCE(cp_pay.cash,   0)             AS credit_cash
		, COALESCE(cp_pay.mpesa,  0)             AS credit_mpesa
		, COALESCE(cp_pay.ecard,  0)             AS credit_ecard
		, COALESCE(cp_pay.cheque, 0)             AS credit_cheque
		, COALESCE(lp_pay.cash,   0)             AS laybye_cash
		, COALESCE(lp_pay.mpesa,  0)             AS laybye_mpesa
		, COALESCE(lp_pay.ecard,  0)             AS laybye_ecard
		, COALESCE(lp_pay.cheque, 0)             AS laybye_cheque
	FROM sales_till st
		LEFT JOIN cash_sale_totals  cst    ON cst.till_num   = st.till_no
		LEFT JOIN returns           ret    ON ret.till_num   = st.till_no
		LEFT JOIN credit_sales      crs    ON crs.till_num   = st.till_no
		LEFT JOIN payments          cs_pay ON cs_pay.till_num = st.till_no AND cs_pay.payment_for = 'cash_sale'
		LEFT JOIN payments          cp_pay ON cp_pay.till_num = st.till_no AND cp_pay.payment_for = 'credit_pay'
		LEFT JOIN payments          lp_pay ON lp_pay.till_num = st.till_no AND lp_pay.payment_for = 'laybye_payment'
		LEFT JOIN rollups           rl     ON rl.till_num    = st.till_no
	WHERE
		($3 = '' OR st.teller   = $3)
		AND ($4 = 0 OR st.till_no = $4)
		AND st.open_time::date >= $1
		AND st.open_time::date <= $2
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
			&r.TillNo, &r.Teller, &r.OpenFloat, &r.CloseCash, &r.CashRollups,
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

		// cash_summary = cash_in + mpesa_in + ecard_in - sales_returns
		r.CashSummary = r.CashIn + r.MpesaIn + r.CardIn - r.SalesReturn

		// cash_bal = close_cash - (cash_in + open_float - rollups)
		r.Balance = r.CloseCash - (r.CashIn + r.OpenFloat - r.CashRollups)

		results = append(results, r)
	}

	return results, nil
}
