package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/sales"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/variables"
	"github.com/jackc/pgx/v5"
)

type ETR struct {
	Seal string `json:"seal"`
	TSIN string `json:"tsin"`
	DATE string `json:"date"`
	CUSN string `json:"cusn"`
	CUIN string `json:"cuin"`
}

type Payment struct {
	Receipt         string                   `json:"receipt"`
	TransDate       time.Time                `json:"trans_date"`
	Approver        string                   `json:"approver"`
	ApproverToken   string                   `json:"approver_token"`
	ScCustomer      string                   `json:"sc_customer"`
	ScNumber        string                   `json:"sc_number"`
	ScPoints        float64                  `json:"sc_points"`
	ScOpen          float64                  `json:"sc_open"`
	ScEarned        float64                  `json:"sc_earned"`
	ScClosing       float64                  `json:"sc_closing"`
	ScBal           string                   `json:"sc_balance"`
	ScDescription   string                   `json:"sc_description"`
	CashTendered    float64                  `json:"cash_tendered"`
	MpesaTendered   float64                  `json:"mpesa_tendered"`
	EcardTendered   float64                  `json:"ecard_tendered"`
	CheckTendered   float64                  `json:"check_tendered"`
	VoucherTotal    float64                  `json:"voucher_total"`
	MpesaDetails    []MpesaDetails           `json:"mpesa_details"`
	EcardDetails    []EcardDetails           `json:"ecard_details"`
	CheckDetails    []CheckDetails           `json:"check_details"`
	GVoucherDetails []GVoucherDetails        `json:"gift_voucher_details"`
	PointsRedeemed  float64                  `json:"points_redeemed"`
	RedPerc         float64                  `json:"red_perc"`
	Tendered        float64                  `json:"tendered"`
	Change          float64                  `json:"change"`
	Items           []sales.Sales            `json:"items"`
	Total           float64                  `json:"total"`
	Branch          string                   `json:"branch"`
	Teller          string                   `json:"teller"`
	Etr             ETR                      `json:"etr"`
	Header          variables.DocHead        `json:"header"`
	Tax             []map[string]interface{} `json:"tax"`
	Loyalty         map[string]interface{}   `json:"loyalty"`
	AcNum           string                   `json:"ac_num"`
	AcName          string                   `json:"ac_name`
	AcTel           string                   `json:"ac_tel`
	CustomerPin     string                   `json:"customer_pin"`
	TillNum         string                   `json:"till"`
	Analysis        map[string]interface{}   `json:"analysis"`
}

type ScPayDetails struct {
	ScCustomer  string `json:"sc_customer"`
	TransType   string `json:"trans_type"`
	TransPoints string `json:"trans_points"`
	TransBal    string `json:"trans_bal"`
}

type MpesaDetails struct {
	MpesaCode string  `json:"mpesa_code"`
	Amount    float64 `json:"mpesa_tendered"`
}

type EcardDetails struct {
	TxnID  int64   `json:"txn_id"`
	Amount float64 `json:"amount"`
}

type CheckDetails struct {
	CheckNum string  `json:"check_num"`
	Bank     string  `json:"bank"`
	PayDate  string  `json:"ecard_tendered"`
	Amount   float64 `json:"amount"`
}

type GVoucherDetails struct {
	SerialNum   string  `json:"serial_num"`
	Amount      float64 `json:"amount"`
	ClaimerName string  `json:"claimer_name"`
	ClaimerTel  string  `json:"claimer_tel"`
	ClaimerID   string  `json:"claimer_id"`
}

// claim receipt paid
func (arg *Payment) ClaimReceipt(ctx context.Context, tx pgx.Tx) error {
	sql := `UPDATE salestrace
			SET
				total = $2
				, cash = $3
				, change = $4
				, state = $5
				, cart = $6
				, pay_details = $7
				, last_updated = now()
				, pay_till = $8
				, analysis = $9
				, etr = $10
				--, loyalty = $11
			WHERE receipt_num = $1`

	fmt.Println("=============== cash tendered ==============================")
	fmt.Println("cash tendered =", arg.CashTendered)
	fmt.Println("change =", arg.Change)
	fmt.Println("total =", arg.Total)
	// prepare payment details to be recorded
	payDets := make(map[string]interface{})
	payDets["cash"] = arg.CashTendered - arg.Change
	payDets["mpesa"] = arg.MpesaTendered
	payDets["ecard"] = arg.EcardTendered
	payDets["check"] = arg.CheckTendered
	payDets["voucher"] = arg.VoucherTotal
	payDets["redeem"] = arg.PointsRedeemed
	fmt.Printf("pays = %v\n\n", payDets)
	pays, _ := json.Marshal(payDets)

	// prepare smartcard details to be recorded
	loyalty := make(map[string]interface{})
	loyalty["name"] = arg.ScCustomer
	// loyals, _ := json.Marshal(loyalty)
	// if arg.ScCustomer != "" {
	// 	loyals = nil
	// }

	cartItems, _ := arg.SalesCart(ctx)
	cart, _ := json.Marshal(cartItems)
	analysis, _ := json.Marshal(arg.Analysis)
	etr, _ := json.Marshal(arg.Etr)

	fmt.Printf("till_num = %s \n\n", arg.TillNum)
	_, err := tx.Exec(ctx, sql, arg.Receipt, arg.Total, arg.CashTendered, arg.Change,
		"POSTED", string(cart), string(pays), arg.TillNum, string(analysis), string(etr) /*, loyals*/)
	if err != nil {
		log.Println("\n\t Error updating salestrace \t", err)
		return err
	}

	return nil
}

func (arg *Payment) EarnScPoint(ctx context.Context, tx pgx.Tx) error {
	sql := `INSERT INTO sc_transactions (txn_type, customer_id, txn_receipt, total, point_opening, point_bal, branch, done_by, approved_by)
			VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	rows, err := database.PgPool.Query(ctx, sql, "Earned points", arg.ScCustomer, arg.Receipt, arg.ScEarned,
		arg.ScOpen, arg.ScClosing, arg.Branch, arg.Teller, arg.Approver)
	if err != nil {
		return err
	}
	defer rows.Close()

	return nil
}

func (arg *Payment) RecordScTxn(ctx context.Context, tx pgx.Tx) error {
	if arg.ScCustomer == "" {
		return nil
	}

	if arg.ScCustomer == "0" {
		return nil
	}

	sql := `INSERT INTO sc_transactions (txn_type, customer_id, txn_receipt, total, point_opening, point_bal, branch, done_by, approved_by)
			VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`
	_, err := tx.Exec(ctx, sql, arg.ScDescription, arg.ScCustomer, arg.Receipt, arg.ScPoints,
		arg.ScOpen, arg.ScClosing, arg.Branch, arg.Teller, arg.Approver)

	if err != nil {
		log.Println("error. failed to redeem sc points     err =", err)
		return err
	}
	return nil
}

func (arg *Payment) ClaimOrders(ctx context.Context, tx pgx.Tx) error {
	sql := `UPDATE salesorders 
			SET 
				state = 'PAID' 
			WHERE receipt_num = $1 AND state NOT IN ('VOIDED', 'DELETED', 'pending')`

	fmt.Println("\n\n\n\t setting salesorders to Paid    receipt =", arg.Receipt)

	// os.Exit(9)

	_, err := tx.Exec(ctx, sql, arg.Receipt)
	if err != nil {
		log.Println("error. failed to claim order as payment     err =", err)
		return err
	}
	return nil
}

func (arg *Payment) FetchReceipt(ctx context.Context, tx pgx.Tx) error {
	sql := `SELECT
				sales.item_code, sales.item_name
				, sales.vat_alpha, sales.vat_perc, sales.vat
				, sales.quantity, sales.price, sales.state
				, s.last_updated 
				, s.receipt_num
    			, coalesce(loyalty::varchar, '')
			FROM salestrace  s, jsonb_to_recordset(s.cart) as 
				sales (
					trans_date timestamptz
					, item_code text
					, item_name text
					, vat_alpha text
					, vat_perc float
					, vat float
					, quantity float
					, price float
					, cost float
					, receipt_num bigint
					, state text
				)
			WHERE s.receipt_num = $1 AND sales.state = 'pending'`

	rows, err := tx.Query(ctx, sql, arg.Receipt)
	if err != nil {
		log.Println("postgres error. failed to query receipt    err =", err)
		return fmt.Errorf("failed fetching receipt")
	}
	defer rows.Close()

	var vals []sales.Sales
	arg.Total = 0
	for rows.Next() {
		loyaltyDets := ""
		var r sales.Sales
		err = rows.Scan(&r.ItemCode, &r.ItemName, &r.VatAlpha, &r.VatPerc, &r.Vat,
			&r.Quantity, &r.Price, &r.State,
			&arg.TransDate, &arg.Receipt, &loyaltyDets)
		if err != nil {
			log.Println("sql scan error.     err =", err)
			return fmt.Errorf("failed fetching receipt")
		}

		if r.State == "pending" {
			arg.Total += (r.Price * r.Quantity)
		}

		if loyaltyDets != "" && loyaltyDets != "{}" {
			err = json.Unmarshal([]byte(loyaltyDets), &arg.Loyalty)
			if err != nil {
				log.Println("json unmarshalling error    err =", err)
				return fmt.Errorf("failed fetching receipt")
			}
		}

		vals = append(vals, r)
	}
	arg.Items = vals
	// vals = nil

	// set blank as default sc balance
	arg.ScBal = "_____"
	if arg.ScOpen != 0 {
		arg.ScBal = fmt.Sprintf("%v", arg.ScOpen+arg.ScEarned-arg.PointsRedeemed)
	}

	err = arg.GetTaxBreak(ctx)
	if err != nil {
		log.Println("error. getting tax breakdown.    err = %w", err)
		return fmt.Errorf("failed while getting tax breakdown")
	}

	return nil
}

// FetchReceipt fetches receipt details
func GetReceipt(ctxt context.Context, receipt string) (Payment, error) {
	var pay Payment

	sql := `
			SELECT
				s.receipt_num, s.trans_date, coalesce(c.full_names, '') sc_customer
				, coalesce(t.total, 0) sc_points
				, coalesce(t.point_bal, 0) sc_balance
				, s.pay_details::json->'cash' as cash_tendered
				, s.pay_details::json->'mpesa' as mpesa_tendered
				, s.pay_details::json->'ecard' as ecard_tendered
				, s.pay_details::json->'check' as check_tendered
				, s.pay_details::json->'voucher' as voucher_total
				, s.pay_details::json->'redeem' as points_redeemed
				, s.change
				, s.total
				, s.branch
				, s.poster as teller
				, s.etr::varchar
				, s.cart::varchar item
			FROM salestrace s 
				LEFT JOIN sc_transactions t ON s.receipt_num = t.txn_receipt
				LEFT JOIN sc_customer c ON c.customer_id = t.customer_id
			WHERE receipt_num = $1
	`

	ctx, cancel := context.WithTimeout(ctxt, 20*time.Second)
	defer cancel()

	rows, err := database.PgPool.Query(ctx, sql, receipt)
	if err != nil {
		log.Println("error. GetReceipt() failed     err =", err)
		return pay, err
	}

	for rows.Next() {
		var cartStr, etrStr string
		err = rows.Scan(&pay.Receipt, &pay.TransDate, &pay.ScCustomer, &pay.ScPoints, &pay.ScBal,
			&pay.CashTendered, &pay.MpesaTendered, &pay.EcardTendered, &pay.CheckTendered,
			&pay.VoucherTotal, &pay.PointsRedeemed, &pay.Change, &pay.Total, &pay.Branch, &pay.Teller, &etrStr, &cartStr)

		var cart []sales.Sales
		json.Unmarshal([]byte(cartStr), &cart)
		for _, item := range cart {
			if item.State != "DELETED" {
				pay.Items = append(pay.Items, item)
			}
		}

		json.Unmarshal([]byte(etrStr), &pay.Etr)
		if err != nil {
			fmt.Println("error =", err)
		}
	}

	err = pay.GetTaxBreak(ctxt)
	if err != nil {
		return pay, fmt.Errorf("failed while getting tax breakdown")
	}

	return pay, nil
}

// GetTaxBreak
func (pay *Payment) GetTaxBreak(ctxt context.Context) error {
	sql := `SELECT 
				s.vat_alpha code
				, min(s.vat_perc) rate
				, SUM(vat) vat
				, sum((quantity*price)-vat) as vatable 
			FROM (SELECT 
						sales.*, s.poster served_by, s.last_updated trans_complete_time
					FROM salestrace s, jsonb_to_recordset(s.cart) as 
						sales (
							trans_date timestamptz
							, item_code text
							, item_name text
							, vat_alpha text
							, vat_perc float
							, vat float
							, quantity float
							, price float
							, cost float
							, receipt_num bigint
							, state text
					)
					WHERE s.receipt_num = $1 AND s.state NOT IN ('DELETED', 'VOIDED')
				) s
			GROUP BY s.vat_alpha`

	ctx, cancel := context.WithTimeout(ctxt, 20*time.Second)
	defer cancel()

	rows, err := database.PgPool.Query(ctx, sql, pay.Receipt)
	if err != nil {
		return err
	}
	defer rows.Close()

	var vals []map[string]interface{}
	for rows.Next() {
		var alpha, perc string
		var vatable, vat float64

		err = rows.Scan(&alpha, &perc, &vatable, &vat)
		if err != nil {
			log.Println("error scanning tax sum err =", err)
		}

		r := make(map[string]interface{})

		r["vat_alpha"] = alpha
		r["perc"] = perc
		r["vatable"] = vatable
		r["vat"] = vat

		vals = append(vals, r)
	}
	pay.Tax = vals

	fmt.Println("tax breakdown = ", pay.Tax)

	return nil
}

func (pay *Payment) SalesCart(ctxt context.Context) ([]sales.Sales, error) {
	sql := `
		SELECT 
			cast(coalesce(cart, '[]') as varchar) 
		FROM salestrace 
		WHERE receipt_num = $1`

	ctx, cancel := context.WithTimeout(ctxt, 20*time.Second)
	defer cancel()

	vals := []sales.Sales{}
	cart := ""
	if err := database.PgPool.QueryRow(ctx, sql, pay.Receipt).Scan(&cart); err != nil {
		return vals, err
	}

	err := json.Unmarshal([]byte(cart), &vals)
	if err != nil {
		return vals, err
	}

	return vals, nil
}
