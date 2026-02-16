package sales

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/logins"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/products"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/variables"
)

type Sales struct {
	table       string    `name:"sales" type:"table"`
	TransDate   time.Time `json:"trans_date" type:"field" sql:"TIMESTAMPTZ NOT NULL DEFAULT now()"`
	ReceiptNum  int64     `json:"receipt_num" type:"field" sql:"BIGINT NOT NULL"`
	OrderNum    int64     `json:"order_num" type:"field" sql:"BIGINT NOT NULL"`
	TxnID       int64     `json:"txn_id" type:"field" sql:"BIGSERIAL NOT NULL UNIQUE"`
	HsCode      string    `json:"hs_code" type:"field" sql:"VARCHAR NOT NULL"`
	ItemCode    string    `json:"item_code" type:"field" sql:"VARCHAR NOT NULL"`
	ItemName    string    `json:"item_name" type:"field" sql:"VARCHAR NOT NULL"`
	Quantity    float64   `json:"quantity" type:"field" sql:"FLOAT NOT NULL DEFAULT '0' "`
	Cost        float64   `json:"cost" type:"field" sql:"FLOAT NOT NULL DEFAULT '0' "`
	Price       float64   `json:"price" type:"field" sql:"FLOAT NOT NULL DEFAULT '0' "`
	Discount    float64   `json:"discount" type:"field" sql:"FLOAT NOT NULL DEFAULT '0' "`
	Total       float64   `json:"total" type:"field" sql:"FLOAT NOT NULL DEFAULT '0' "`
	OnOffer     bool      `json:"on_offer" type:"field" sql:"BOOL NOT NULL DEFAULT 'false'"`
	Vat         float64   `json:"vat" type:"field" sql:"FLOAT NOT NULL DEFAULT '0'"`
	VatAlpha    string    `json:"vat_alpha" type:"field" sql:"VARCHAR(1) NOT NULL"`
	State       string    `json:"state" type:"field" sql:"VARCHAR NOT NULL DEFAULT 'active' "`
	ReceiptItem string    `json:"receipt_item" type:"field" sql:"VARCHAR NOT NULL"`
}

// genSalesTbl
func genSalesTbl() error {
	var tblStruct Sales
	return database.CreateFromStruct(tblStruct)
}

var fetchUser = func(u *logins.Users, ctx context.Context) error {
	return u.FetchUser(ctx)
}

// AddCart adds an item to the cart
// writes through to cache
// returns an error if fails
func (arg *Sales) AddCart() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// fetch from details from inventory microservice
	p := products.StockMaster{ItemCode: arg.ItemCode}
	err := p.Fetch(ctx)
	if err != nil {
		return err
	}

	// validate p
	if p.ItemCode == "" {
		return errors.New("item code is required")
	}
	if p.TillPrice == 0 {
		return errors.New("item price is required")
	}

	// create a unique ReceiptItem for entry

	log.Println("product details = ", p)

	return nil
}

// CreateReceipt creates a new receipt number
func (arg *ReceiptLog) CreateReceipt() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	userDetails := logins.Users{Username: arg.Poster}
	err := fetchUser(&userDetails, ctx)
	if err != nil {
		return 0, err
	}
	fmt.Println("user_details =", userDetails)

	if userDetails.AcceptPayment {
		arg.PayTill = userDetails.TillNum
	}

	// prepare sql statement to get the next receipt number
	sql := `SELECT CAST(CONCAT(
						cast(1 as varchar)
						, extract(YEAR FROM now())
						, LPAD(EXTRACT(MONTH FROM now())::text, 2, '0')
						, LPAD(EXTRACT(DAY FROM now())::text, 2, '0')
						, '0'
						, cast(coalesce(max(daily_count), 0) + 1 as varchar) 
    				) AS BIGINT)
     			, coalesce(max(daily_count), 0) + 1
			FROM salestrace WHERE trans_date::date = (SELECT now()::date)
	`

	rows, err := database.PgPool.Query(ctx, sql)
	if err != nil {
		log.Println("error. failed to get receipt     err =", err)
		return 0, err
	}
	defer rows.Close()

	// scan items
	for rows.Next() {
		rows.Scan(&arg.ReceiptNum, &arg.DailyCount)
	}

	fmt.Printf("created receipt = %v", arg.ReceiptNum)

	err = arg.LogReceipt(ctx)
	if err != nil {
		return -1, err
	}
	return arg.ReceiptNum, nil
}

// LogReceipt logs the created receipt number to database
func (a *ReceiptLog) LogReceipt(ctx context.Context) error {
	fmt.Printf("\n\treceipt logged = %v, %v, %v, %v, %v, %v, %v, %v ", a.TillNum, a.ReceiptNum, a.Poster, a.DailyCount, a.Branch, a.CompanyID, a.SaleType, a.LaybyeID)

	// prepare sql to insert new receipt number
	sql := `INSERT INTO salestrace(trans_date, till_num, receipt_num, poster, daily_count, branch, company_id, sale_type, laybye_id, pay_till) 
			VALUES(now(), $1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING trans_date`
	// execute statement
	rows, err := database.PgPool.Query(ctx, sql, a.TillNum, a.ReceiptNum, a.Poster, a.DailyCount, a.Branch, a.CompanyID, a.SaleType, a.LaybyeID, a.PayTill)
	if err != nil {
		log.Println("error. failed to save receipt to log     err =", err)
		return err
	}

	var transDate time.Time
	for rows.Next() {
		err := rows.Scan(&transDate)
		if err != nil {
			log.Println("failed to scan salestrace time    err =", err)
		}
	}
	return nil
}

// CashInTill fetches and returns total cash in current till
func CashInTill(till int64) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	start := time.Now()
	fmt.Println("Fetching cash in till for till num ", till)
	defer fmt.Printf("\n\t\t function CashInTill() took:  %v \n", time.Since(start))

	// cash in register = amount_paid_in_cash - rollups
	// fetch cash amount in till from database
	sql := `
		SELECT 
			coalesce(coalesce(c.cash, 0) + coalesce(lay_payments.cash, 0) + coalesce(credit.cash , 0), 0) - coalesce(rolls.amount, 0) cash_in_till
		FROM
		(SELECT 
			pay_till 
			, SUM(cast(pay_details::json->'cash' as varchar)::float) as cash 
			, SUM(cast(pay_details::json->'mpesa' as varchar)::float) as mpesa 
			, SUM(cast(pay_details::json->'ecard' as varchar)::float) as ecard 
			, SUM(cast(pay_details::json->'check' as varchar)::float) as cheque
			, SUM(cast(pay_details::json->'redeem' as varchar)::float) as redeemed 
			, SUM(cast(pay_details::json->'voucher' as varchar)::float) as voucher 
		FROM salestrace  
		WHERE pay_till = $1 AND state = 'POSTED' GROUP BY pay_till) as c
				LEFT JOIN
		(SELECT coalesce(sum(amount), 0) as amount, till_num 
			FROM cash_movement WHERE type = 'cash rollup' GROUP BY till_num) as rolls
						ON rolls.till_num = c.pay_till
				LEFT JOIN
		(SELECT till_num, coalesce(SUM(cash_paid), 0) as cash, coalesce(SUM(mpesa_paid), 0) as mpesa
			, coalesce(SUM(ecard_paid), 0) as ecard, coalesce(SUM(cheque_paid), 0) as cheque
			, coalesce(SUM(amount), 0) total_sales  
		FROM accounts_txn GROUP BY till_num) as credit
			ON c.pay_till = credit.till_num
				LEFT JOIN
		(SELECT 
				t.till_no as till_num
				, cash.amount as cash
				, mpesa.amount as mpesa
				, ecards.amount as ecard
				, cheque.amount as cheque
			FROM
			(SELECT till_no FROM sales_till) as t
				LEFT JOIN
			(SELECT till_num, coalesce(sum(amount_paid), 0) as amount 
				FROM laybye_trans 
			WHERE trans_type = 'payment' AND pay_type = 'cash' GROUP BY till_num) as cash
				ON t.till_no = cash.till_num
				LEFT JOIN
			(SELECT till_num, coalesce(sum(amount_paid), 0) as amount 
				FROM laybye_trans 
			WHERE trans_type = 'payment' AND pay_type = 'mpesa' GROUP BY till_num) as mpesa
				ON t.till_no = mpesa.till_num
				LEFT JOIN
			(SELECT till_num, coalesce(sum(amount_paid), 0) as amount 
				FROM laybye_trans 
			WHERE trans_type = 'payment' AND pay_type = 'ecard' GROUP BY till_num) as ecards
				ON t.till_no = mpesa.till_num
				LEFT JOIN
			(SELECT till_num, coalesce(sum(amount_paid), 0) as amount 
				FROM laybye_trans 
			WHERE trans_type = 'payment' AND pay_type = 'cheque' GROUP BY till_num) as cheque
				ON t.till_no = mpesa.till_num) as lay_payments
			ON lay_payments.till_num = c.pay_till;
		`

	// fmt.Printf("\n\t SQL \n %v", sql)
	rows, err := database.PgPool.Query(ctx, sql, till)
	if err != nil {
		log.Println("\n\t\t error cash in till, ", err)
		return 0, err
	}
	defer rows.Close()

	var balance float64
	for rows.Next() {
		err := rows.Scan(&balance)
		if err != nil {
			log.Println("\t\t\t failed to scan CashInTill error:", err)
			return 0, err
		}
	}

	fmt.Printf("\n\t\t till_no = %v \t Amount in till = %v\n", till, balance)

	return balance, nil
}

// FetchSettings gets pos settings from database
func FetchSettings() (variables.PosSettings, error) {
	start := time.Now()
	// sql := "SELECT params FROM sys_conf WHERE label = 'pos_defaults'"

	settings, _ := variables.SysDefaults()

	elapsed := time.Since(start)
	fmt.Printf("\n\t\t function FetchSettings() took: %v \n", elapsed)
	return settings.PosDefaults, nil
}
