package payment

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
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
	Items           []SalesItem              `json:"items"`
	Total           float64                  `json:"total"`
	Branch          string                   `json:"branch"`
	Teller          string                   `json:"teller"`
	Etr             ETR                      `json:"etr"`
	Header          base.DocHead             `json:"header"`
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

func (arg *Payment) CommitPay(ctxt context.Context) error {
	ctx, cancel := context.WithTimeout(ctxt, 30*time.Second)
	defer cancel()

	tx, err := database.PgPool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		log.Println("failed to begin transactions     err=", err)
		return err
	}
	defer tx.Rollback(ctx)

	return tx.Commit(ctx)
}

// claim payment on receipt
func (arg *Payment) PayCart(ctx context.Context, tx *sql.Tx) error {
	sql := `UPDATE sales_live
			SET trans_complete_time = $1
				, state = 'PAID'
				, sc_id = $2
			WHERE receipt_num = $3 AND state = 'pending' `

	t := time.Now()
	_, err := tx.ExecContext(ctx, sql, t, arg.ScCustomer, arg.Receipt)
	if err != nil {
		log.Println("error. failed to complete cart payment    err =", err)
		return err
	}
	return nil
}

// claim receipt paid
func (arg *Payment) ClaimReceipt(ctx context.Context, tx *sql.Tx) error {
	sql := `UPDATE salestrace 
			SET 
				total = $1
				, cash = $2 
				, change = $3
				, state = $4
				, cart = $5
				, pay_details = $6 
				, last_updated = now()
				, pay_till = $7
				, analysis = $8
				, loyalty = $9
			WHERE receipt_num = $10`

	// prepare payment details to be recorded
	payDets := make(map[string]interface{})
	payDets["cash"] = arg.CashTendered - arg.Change
	payDets["mpesa"] = arg.MpesaTendered
	payDets["ecard"] = arg.EcardTendered
	payDets["check"] = arg.CheckTendered
	payDets["voucher"] = arg.VoucherTotal
	payDets["redeem"] = arg.PointsRedeemed
	pays, _ := json.Marshal(payDets)

	// prepare smartcard details to be recorded
	loyalty := make(map[string]interface{})
	loyalty["name"] = arg.ScCustomer
	loyals, _ := json.Marshal(loyalty)

	if arg.ScCustomer != "" {
		loyals = nil
	}

	cartItems, _ := SalesCart(arg.Receipt)
	cart, _ := json.Marshal(cartItems)
	analysis, _ := json.Marshal(arg.Analysis)
	_, err := tx.ExecContext(ctx, sql, arg.Total, arg.CashTendered, arg.Change,
		"POSTED", cart, pays, arg.TillNum, analysis, loyals, arg.Receipt)
	if err != nil {
		fmt.Println("\t Error updating salestrace \t", err)
		return err
	}

	return nil
}

// claim all mpesa payments
func (arg *Payment) ClaimMpesa(ctx context.Context, tx pgx.Tx) error {
	var total float64
	sql := `UPDATE mobile_money SET trace_num = $1, claimed = True, notation = 'cash sale' WHERE code = $2 RETURNING amount`
	for _, detail := range arg.MpesaDetails {
		var amount float64
		if err := tx.QueryRow(ctx, sql, arg.Receipt, detail.MpesaCode).Scan(&amount); err != nil {
			log.Println("error. failed to claim mobile_money     err =", err)
			return err
		}

		total += amount
	}

	arg.MpesaTendered = total
	return nil
}

// claim all ecard payments
func (arg *Payment) ClaimEcards(ctx context.Context, tx pgx.Tx) error {
	fmt.Println("\t\t\tstarted claim ecards")
	start := time.Now()
	defer fmt.Printf("\t\t\tClaimEcards took %v\n", time.Since(start))

	var total float64
	sql := `UPDATE card_transaction SET sales_receipt = $1 WHERE auto_id = $2 RETURNING amount`
	for _, detail := range arg.EcardDetails {
		fmt.Println("\t\t\t\t ecards detail =", detail)
		var ecard float64
		err := tx.QueryRow(ctx, sql, arg.Receipt, detail.TxnID).Scan(&ecard)
		if err != nil {
			log.Println("error. failed to claim ecards     err =", err)
			return err
		}

		total += ecard
	}
	arg.EcardTendered = total

	return nil
}

// claim all gift vouchers
func (arg *Payment) ClaimGiftVoucher(ctx context.Context, tx pgx.Tx) (float64, error) {
	sql := `UPDATE gift_voucher 
			SET 
				txn_receipt = $1
				, teller = $2
				, claimer_name = $3
				, claimer_tel = $4
				, claimer_id = $5
				, approvers = $6
			WHERE serial = $7
			`

	total := float64(0)
	for _, detail := range arg.GVoucherDetails {
		vcher := float64(0)
		err := tx.QueryRow(ctx, sql, arg.Receipt, arg.Teller, detail.ClaimerName, detail.ClaimerTel, detail.ClaimerID, arg.Approver, detail.SerialNum).Scan(&vcher)
		if err != nil {
			log.Println("error. failed to claim gift_voucher     err =", err)
			return total, err
		}

		total += vcher
	}
	return total, nil
}

func (arg *Payment) AddCart(ctx context.Context, tx pgx.Tx) error {
	cart, err := SalesCart(arg.Receipt)
	if err != nil {
		log.Println("error. failed getting sales cart    err =", err)
		return fmt.Errorf("error. %v", err)
	}

	// prepare sql statement
	sql := `INSERT INTO sales_live(trans_date, item_code, return_code, item_name
				, receipt_num, quantity, cost, price
				, vat_alpha, vat_perc, vat, served_by, branch, till_num
				, sale_type, approved_by, trace, company_id, state, sc_id, on_offer, balance, receipt_item)
			VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23) 
			ON CONFLICT DO NOTHING`

	for _, r := range cart {
		_, err := database.PgPool.Exec(ctx, sql, r.TransDate, r.ItemCode, r.ReturnCode, r.ItemName,
			r.ReceiptNum, r.Quantity, r.Cost, r.Price,
			r.VatAlpha, r.VatPerc, r.Vat, r.ServedBy, r.Branch, r.TillNum,
			r.SaleType, r.ApprovedBy, r.Trace, r.CompanyID, r.State, r.ScID, r.OnOffer, r.Balance, r.ReceiptItem)

		if err != nil {
			log.Println("error. failed to add cart items    err =", err)
			return fmt.Errorf("error. %v", err)
		}
	}
	return nil
}

func (arg *Payment) EarnScPoint(ctx context.Context, tx *sql.Tx) error {
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

// FetchReceipt fetches receipt details
func GetReceipt(receipt string) (Payment, error) {
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
	rows, err := database.PgPool.Query(ctx, receipt)
	if err != nil {
		log.Println("error. GetReceipt() failed     err =", err)
		return pay, err
	}

	for rows.Next() {
		var cartStr, etrStr string
		err = rows.Scan(&pay.Receipt, &pay.TransDate, &pay.ScCustomer, &pay.ScPoints, &pay.ScBal,
			&pay.CashTendered, &pay.MpesaTendered, &pay.EcardTendered, &pay.CheckTendered,
			&pay.VoucherTotal, &pay.PointsRedeemed, &pay.Change, &pay.Total, &pay.Branch, &pay.Teller, &etrStr, &cartStr)

		var cart []SalesItem
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

	pay.Tax, err = GetTaxBreak(pay.Receipt)
	if err != nil {
		return pay, fmt.Errorf("failed while getting tax breakdown")
	}

	return pay, nil
}
