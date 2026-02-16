package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
	"github.com/jackc/pgx/v5"
)

// ReceiptLog holds receipt traces
type ReceiptLog struct {
	table           string                 `name:"salestrace" type:"table"`
	TransDate       time.Time              `json:"trans_date" name:"trans_date" type:"field" sql:"TIMESTAMPTZ NOT NULL DEFAULT now()"`
	ReceiptNum      int64                  `json:"receipt_num" name:"receipt_num" type:"field" sql:"BIGINT PRIMARY KEY"`
	TillNum         int64                  `json:"till_num" name:"till_num" type:"field" sql:"BIGINT NOT NULL"`
	PayTill         int64                  `json:"pay_till" name:"pay_till" type:"field" sql:"BIGINT NOT NULL"`
	CompanyID       int64                  `json:"company_id" name:"company_id" type:"field" sql:"BIGINT"`
	DailyCount      int32                  `json:"daily_count" name:"daily_count" type:"field" sql:"INT"`
	Branch          string                 `json:"branch" name:"branch" type:"field" sql:"VARCHAR"`
	Poster          string                 `json:"poster" name:"poster" type:"field" sql:"VARCHAR"`
	Total           float32                `json:"total" name:"total" type:"field" sql:"FLOAT NOT NULL DEFAULT '0'"`
	Cash            float32                `json:"cash" name:"cash" type:"field" sql:"FLOAT NOT NULL DEFAULT '0'"`
	Change          float32                `json:"change" name:"change" type:"field" sql:"FLOAT"`
	MpesaDetails    []MpesaDetails         `json:"mpesa_details" name:"mpesa_details" type:"field" sql:"JSONB"`
	Loyalty         map[string]string      `json:"loyalty" name:"loyalty" type:"field" sql:"JSONB"`
	Paymode         string                 `json:"paymode" name:"paymode" type:"field" sql:"VARCHAR"`
	SaleType        string                 `json:"sale_type" name:"sale_type" type:"field" sql:"VARCHAR NOT NULL DEFAULT 'Cash Sale'"`
	Cart            []Sales                `json:"cart" name:"cart" type:"field" sql:"JSONB"`
	CashBal         float64                `json:"cash_bal" name:"cash_bal" type:"field" sql:"FLOAT NOT NULL DEFAULT '0'"`
	State           string                 `json:"state" name:"state" type:"field" sql:"VARCHAR NOT NULL DEFAULT 'pending'"`
	Approver        string                 `json:"approver" name:"approver" type:"field" sql:"VARCHAR(100) NOT NULL DEFAULT 'nan'"`
	MirroredBy      []string               `json:"mirrored_by" name:"mirrored_by" type:"field"  sql:"VARCHAR[]"`
	LaybyeID        int64                  `json:"laybye_id" name:"laybye_id" type:"field" sql:"BIGINT NOT NULL DEFAULT '0'"`
	PayDetails      string                 `json:"pay_details" name:"pay_details" type:"field" sql:"JSONB NOT NULL DEFAULT '{}' "`
	MpesaTxn        string                 `json:"mpesa_txn" name:"mpesa_txn" type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	EtrSeal         string                 `json:"etr_seal" name:"etr_seal" type:"field" sql:"VARCHAR" `
	SyncServers     string                 `json:"sync_servers" name:"sync_servers" type:"field" sql:"VARCHAR[] NOT NULL DEFAULT '{}'"`
	LastUpdated     time.Time              `json:"last_updated" name:"last_updated" type:"field" sql:"TIMESTAMPTZ NOT NULL DEFAULT now()"`
	OrdersInBill    int                    `json:"orders_in_bill" name:"orders_in_bill" type:"field" sql:"INT NOT NULL DEFAULT '0'"`
	Etr             ETR                    `json:"etr" name:"etr" type:"field" sql:"JSONB"`
	ReturnTrace     int64                  `json:"return_trace" name:"return_trace" type:"field" sql:"BIGINT NOT NULL DEFAULT '0'"`
	Analysis        map[string]interface{} `json:"analysis" name:"analysis" type:"field" sql:"JSONB"`
	AcNum           string                 `json:"ac_num" name:"ac_num" type:"field" sql:"VARCHAR"`
	Token           string                 `json:"token"`
	constraint      string                 `name:"" type:"field" sql:"CONSTRAINT fk_salestrace_till_num FOREIGN KEY (till_num) REFERENCES sales_till(till_no)"`
	debtoronstraint string                 `name:"" type:"field" sql:"CONSTRAINT fk_debtors_acnum FOREIGN KEY (ac_num) REFERENCES debtors(ac_num)"`
}

func genReceiptTbl() error {
	var tblStruct ReceiptLog
	return database.CreateFromStruct(tblStruct)
}

// GenReceipt creates or returns next available sales receipt
func (arg *ReceiptLog) GenReceipt() (int64, error) {
	start := time.Now()
	fmt.Println("sale type =", arg.SaleType)
	fmt.Println("laybye id =", arg.LaybyeID)
	defer fmt.Printf("GenReceipt took %v", time.Since(start))

	if arg.TillNum == 0 {
		return 0, fmt.Errorf("op error, till num is null")
	}

	fmt.Println("Gen Receipt for till num =", arg.TillNum)
	// get all active receipts for current user
	sql := `
			SELECT 
				coalesce(max(receipt_num), 0) 
			FROM salestrace 
			WHERE state in ('pending', 'paying')
				AND till_num = $1
				AND sale_type = $2`

	// Query database rows
	rows, err := database.PgPool.Query(context.Background(), sql, arg.TillNum, arg.SaleType)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	// scan rows
	for rows.Next() {
		rows.Scan(&arg.ReceiptNum)
	}

	fmt.Println("Gen Receipt num =", arg.ReceiptNum)
	// return receipt number when there exists a pending receipt number
	if arg.ReceiptNum > 0 {
		elapsed := time.Since(start)
		fmt.Printf("\n\t\t function GenReceipt() took:   %v \n", elapsed)
		return arg.ReceiptNum, nil
	}

	// create a new receipt number
	arg.ReceiptNum, err = arg.CreateReceipt()
	fmt.Printf("\nreceipt = %v\n", arg.ReceiptNum)
	if err != nil {
		fmt.Printf("Error creating receipt %v\n", err.Error())
		return 0, err
	}

	return arg.ReceiptNum, nil
}

// GenReceipt creates or returns next available sales receipt
func (arg *ReceiptLog) Fetch() error {
	start := time.Now()
	defer fmt.Printf("GenReceipt took %v", time.Since(start))

	if arg.ReceiptNum == 0 {
		return fmt.Errorf("op error, receipt num is null")
	}

	fmt.Println("Gen Receipt for till num =", arg.TillNum)
	// get all pending receipts for current user
	sql := `SELECT
				trans_date
				, receipt_num
				, till_num
				, pay_till
				, branch
				, poster
				, coalesce(total, 0)
				, coalesce(change, 0)
				, state
				, approver
				, coalesce(cart::varchar, '')
				, pay_details
				, total
			FROM salestrace
			WHERE receipt_num = $1`

	// Query database rows
	rows, err := database.PgPool.Query(context.Background(), sql, arg.ReceiptNum)
	if err != nil {
		fmt.Printf("operation error \n%v", err.Error())
		return err
	}
	defer rows.Close()

	cart := ""
	payDets := ""
	// scan rows
	arg.ReceiptNum = 0
	for rows.Next() {
		err := rows.Scan(&arg.TransDate, &arg.ReceiptNum, &arg.TillNum, &arg.PayTill, &arg.Branch, &arg.Poster,
			&arg.Total, &arg.Change, &arg.State, &arg.Approver, &cart, &payDets, &arg.Total)
		if err != nil {
			fmt.Printf("error. failed to scan receipt_log items \n\t %v\n\n", err.Error())
			return fmt.Errorf("error. failed to scan receipt log items    err = %v", err)
		}
	}

	json.Unmarshal([]byte(cart), &arg.Cart)
	if cart == "" {
		arg.Cart = nil
	}

	json.Unmarshal([]byte(payDets), &arg.PayDetails)

	arg.Total = 0
	Cart := []Sales{}
	for _, item := range arg.Cart {
		if item.State == "pending" {
			arg.Total += (float32(item.Price) * float32(item.Quantity))

			Cart = append(Cart, item)
		}

	}

	arg.Cart = Cart

	return nil
}

// FetchAll creates or returns next available sales receipt
func (arg *ReceiptLog) FetchAll() error {
	start := time.Now()
	defer fmt.Printf("GenReceipt took %v", time.Since(start))

	if arg.ReceiptNum == 0 {
		return fmt.Errorf("op error, receipt num is null")
	}

	fmt.Println("Gen Receipt for till num =", arg.TillNum)
	// get all pending receipts for current user
	sql := `SELECT
				trans_date, receipt_num, till_num, pay_till, branch, poster
				, coalesce(total, 0), coalesce(cash, 0), coalesce(change, 0), loyalty, state, approver
				, coalesce(cart::varchar, ''), pay_details, etr_seal, coalesce(orders_in_bill, 0)
				, analysis::varchar, last_updated
			FROM salestrace
			WHERE receipt_num = $1`

	// Query database rows
	rows, err := database.PgPool.Query(context.Background(), sql, arg.ReceiptNum)
	if err != nil {
		fmt.Printf("operation error \n%v", err.Error())
		return err
	}
	defer rows.Close()

	cart := ""
	// payDets := ""
	// scan rows
	arg.ReceiptNum = 0
	for rows.Next() {
		loyalty := ""
		analysis := ""

		err := rows.Scan(&arg.TransDate, &arg.ReceiptNum, &arg.TillNum, &arg.PayTill, &arg.Branch, &arg.Poster,
			&arg.Total, &arg.Cash, &arg.Change, &loyalty, &arg.State, &arg.Approver,
			&cart, &arg.PayDetails, &arg.EtrSeal, &arg.OrdersInBill,
			&analysis, &arg.LastUpdated)
		if err != nil {
			fmt.Printf("error. failed to scan receipt_log items \n\t %v\n\n", err.Error())
			return fmt.Errorf("error. failed to scan receipt log items    err = %v", err)
		}

		err = json.Unmarshal([]byte(loyalty), &arg.Loyalty)
		if err != nil {
			log.Println("error unmarshalling loyalty data: ", err)
			return fmt.Errorf("error unmarshalling loyalty data: %v", err)
		}

		err = json.Unmarshal([]byte(analysis), &arg.Analysis)
		if err != nil {
			log.Println("error. failed to unmarshall analysis data: ", err)
			return err
		}
	}

	json.Unmarshal([]byte(cart), &arg.Cart)
	if cart == "" {
		arg.Cart = nil
	}

	arg.Total = 0
	for _, item := range arg.Cart {
		if item.State == "pending" {
			arg.Total += (float32(item.Price) * float32(item.Quantity))
		}
	}

	return nil
}

func (arg *ReceiptLog) Archive() error {
	err := arg.FetchAll()
	if err != nil {
		return err
	}

	return nil
}

func (arg *ReceiptLog) FetchRange() error {
	return nil
}

func (arg *ReceiptLog) GetEmpty() (int, error) {
	sql := `SELECT count(*) FROM salestrace WHERE receipt_num = $1 AND cart IS NULL`

	rows, err := database.PgPool.Query(context.Background(), sql, arg.ReceiptNum)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	counted := 0
	for rows.Next() {
		err := rows.Scan(&counted)
		if err != nil {
			return 0, err
		}
	}

	return counted, nil
}

func (arg *ReceiptLog) Delete() error {
	sql := `UPDATE salestrace SET state = 'VOIDED' WHERE receipt_num = $1 AND state not in ('POSTED', 'DEBITED', 'CREDITED', 'PAID', 'AWAITING RECEIPT')`

	_, err := database.PgPool.Exec(context.Background(), sql, arg.ReceiptNum)
	if err != nil {
		return fmt.Errorf("failed to void receipt")
	}
	return nil
}

func (arg *ReceiptLog) DeleteCtx(ctx context.Context, tx pgx.Tx) error {
	sql := `UPDATE salestrace SET state = 'VOIDED' WHERE receipt_num = $1 AND state not in ('POSTED', 'DEBITED', 'CREDITED', 'PAID', 'AWAITING RECEIPT')`

	_, err := tx.Exec(ctx, sql, arg.ReceiptNum)
	if err != nil {
		return fmt.Errorf("failed to void receipt")
	}
	return nil
}

func (arg *ReceiptLog) DelOrderCtx(ctx context.Context, tx pgx.Tx) error {
	sql := `UPDATE salesorders SET state = 'DELETED' WHERE receipt_num = $1`

	_, err := tx.Exec(ctx, sql, arg.ReceiptNum)
	if err != nil {
		return err
	}
	return nil
}

func (arg *ReceiptLog) DelCascade() error {
	ctx := context.Background()

	tx, err := database.PgPool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	err = arg.DeleteCtx(ctx, tx)
	if err != nil {
		return err
	}

	err = arg.DelOrderCtx(ctx, tx)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (arg *ReceiptLog) Suspend() error {
	sql := `UPDATE salestrace SET state = 'suspend' 
			WHERE till_num = $1 AND state = 'pending' AND cart IS NOT NULL `

	_, err := database.PgPool.Exec(context.Background(), sql, arg.TillNum)
	if err != nil {
		log.Println("suspend error ", err)
		return err
	}
	return nil
}

func (arg *ReceiptLog) NewBill() error {
	sql := `UPDATE salestrace st
			SET
				state = 'suspend'
			FROM(
				SELECT 
					s.receipt_num
					, count(so.order_num) orders_in_bill
				FROM salestrace s LEFT JOIN salesorders so ON so.receipt_num = s.receipt_num
				WHERE s.state = 'pending' 
					AND (so.order_items IS NOT NULL OR s.cart IS NOT NULL)
					AND s.till_num = $1
				GROUP BY s.receipt_num) as a
			WHERE st.receipt_num = a.receipt_num
		`

	_, err := database.PgPool.Exec(context.Background(), sql, arg.TillNum)
	if err != nil {
		log.Println("suspend error ", err)
		return err
	}
	return nil
}

func (arg *ReceiptLog) ResumeOrderContext(ctx context.Context, tx pgx.Tx) error {
	sql := `UPDATE salesorders 
			SET 
				state = 'dispatched' 
			WHERE state = 'paying' AND receipt_num = $1`

	_, err := tx.Exec(ctx, sql, arg.ReceiptNum)
	if err != nil {
		return err
	}
	return nil
}

func (arg *ReceiptLog) ResumeBillContext(ctx context.Context, tx pgx.Tx) error {
	sql := `UPDATE salestrace 
			SET 
				state = 'pending' 
			WHERE 
				state IN ('pending payment', 'paying') 
				AND receipt_num = $1
			`

	_, err := tx.Exec(ctx, sql, arg.ReceiptNum)
	if err != nil {
		return err
	}
	return nil
}

func (arg *ReceiptLog) Resume() error {
	ctx := context.Background()
	tx, err := database.PgPool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	err = arg.ResumeOrderContext(ctx, tx)
	if err != nil {
		return err
	}

	err = arg.ResumeBillContext(ctx, tx)
	if err != nil {
		return err
	}

	tx.Commit(ctx)
	return nil
}

func (arg *ReceiptLog) Merge(receipts []int64) error {
	ctx := context.Background()
	tx, err := database.PgPool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	receiptsStmt := "("
	for i, rcpt := range receipts {
		if i > 0 { // (len(receipts) - 1) {
			receiptsStmt += ", "
		}
		receiptsStmt += fmt.Sprintf(`%v`, rcpt)
	}
	receiptsStmt += ")"
	fmt.Println(receiptsStmt)

	err = arg.CombineOrdersContext(receiptsStmt, ctx, tx)
	if err != nil {
		return err
	}

	err = arg.VoidReceiptsContext(receiptsStmt, ctx, tx)
	if err != nil {
		return err
	}

	err = arg.GetNewOrderItems(ctx, tx)
	if err != nil {
		return err
	}

	err = arg.CombineReceiptsContext(ctx, tx)
	if err != nil {
		return err
	}

	tx.Commit(ctx)
	return nil
}

func (arg *ReceiptLog) CombineOrdersContext(receiptCombo string, ctx context.Context, tx pgx.Tx) error {
	sql := fmt.Sprintf(`UPDATE salesorders SET receipt_num = $1, state = 'paying' WHERE state not IN ('pending', 'voided', 'VOIDED', 'DELETED') AND receipt_num IN %v`, receiptCombo)

	_, err := tx.Exec(ctx, sql, arg.ReceiptNum)
	if err != nil {
		fmt.Println("sql error CombineOrdersContext()    err =", err)
		return err
	}

	return nil
}

func (arg *ReceiptLog) VoidReceiptsContext(receiptCombo string, ctx context.Context, tx pgx.Tx) error {
	sql := fmt.Sprintf(`UPDATE salestrace SET state = 'VOIDED' WHERE receipt_num IN %v`, receiptCombo)

	_, err := tx.Exec(ctx, sql)
	if err != nil {
		fmt.Println("sql error VoidReceiptsContext()    err =", err)
		return err
	}

	return nil
}

func (arg *ReceiptLog) GetNewOrderItems(ctx context.Context, tx pgx.Tx) error {
	if arg.ReceiptNum == 0 {
		return fmt.Errorf("error ReceiptLog->CombineOrdersInBill()    receipt_num is null")
	}

	sql := `SELECT 
				cast(coalesce(order_items, '[]') as varchar) 
			FROM salesorders 
			WHERE state in ('paying', 'pending payment') AND receipt_num = $1`

	// var values []Sales
	rows, err := tx.Query(ctx, sql, arg.ReceiptNum)
	if err != nil {
		fmt.Println("sql error GetNewOrderItems()    err =", err)
		return err
	}
	defer rows.Close()

	var orderItems string

	arg.Cart = nil
	for rows.Next() {
		err := rows.Scan(&orderItems)
		if err != nil {
			return err
		}

		var val []Sales
		err = json.Unmarshal([]byte(orderItems), &val)
		if err != nil {
			return err
		}

		arg.Cart = append(arg.Cart, val...)
	}

	return nil
}

func (arg *ReceiptLog) CombineReceiptsContext(ctx context.Context, tx pgx.Tx) error {
	sql := `UPDATE salestrace 
			SET 
				state = 'pending payment', 
				cart = $1,
				total = $2
			WHERE receipt_num = $3`

	items, err := json.Marshal(arg.Cart)
	if err != nil {
		return err
	}

	arg.Total = 0
	for _, row := range arg.Cart {
		if row.State != "DELETED" {
			arg.Total += (float32(row.Quantity) * float32(row.Price))
		}
	}

	_, err = tx.Exec(ctx, sql, items, arg.Total, arg.ReceiptNum)
	if err != nil {
		fmt.Println("sql error CombineReceiptsContext()    err =", err)
		return err
	}

	return nil
}

func (arg *ReceiptLog) CommitSaleCtx(ctx context.Context, tx pgx.Tx) error {
	err := arg.Fetch()
	if err != nil {
		return err
	}

	sql := `
			UPDATE salestrace 
			SET 
				state = 'pending payment'
				, total = $1
				, analysis = $2
			WHERE receipt_num = $3`

	analysis, _ := json.Marshal(arg.Analysis)
	fmt.Printf("%s", analysis)

	_, err = tx.Exec(ctx, sql, arg.Total, analysis, arg.ReceiptNum)
	if err != nil {
		return err
	}

	return nil
}

// Summarize: gives an analysis of how much time was taken
func (arg *ReceiptLog) Analyze() error {
	err := arg.Fetch()
	if err != nil {
		return err
	}

	// Get when first item was scanned
	start := arg.Cart[0].TransDate
	last := arg.Cart[0].TransDate
	prodCount := float64(0)
	for _, itm := range arg.Cart {
		prodCount += itm.Quantity
		if itm.TransDate.Before(start) && itm.State != "DELETED" {
			start = itm.TransDate
		}

		if itm.TransDate.After(last) {
			last = itm.TransDate
		}
	}

	scanDiff, _ := strconv.ParseFloat(fmt.Sprintf("%v", last.Sub(start).Seconds()), 64)
	scanRate := float64(0)
	if scanDiff != 0 {
		scanRate = scanDiff / prodCount
	}

	timeOnSale := time.Since(start)

	analysis := make(map[string]interface{})
	analysis["scan_rate"] = scanRate
	analysis["pay_time"] = time.Since(last).Seconds()
	analysis["time_on_sale"] = timeOnSale.Seconds()
	analysis["prod_sold"] = prodCount

	arg.Analysis = analysis

	return nil
}
