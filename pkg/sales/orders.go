package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
	db "github.com/JohnnyKahiu/speedsales/poserver/database"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/variables"
	"github.com/jackc/pgx/v5"
)

// Order holds a new sales order variable
type Order struct {
	table        string    `name:"salesorders" type:"table"`
	TransDate    time.Time `json:"trans_date" name:"trans_date" type:"field" sql:"TIMESTAMPTZ NOT NULL DEFAULT now()"`
	CompleteTime time.Time `json:"complete_time" name:"complete_time" type:"field" sql:"TIMESTAMPTZ NOT NULL DEFAULT now()"`
	OrderNum     int64     `json:"order_num" name:"order_num" type:"field" sql:"BIGINT NOT NULL PRIMARY KEY"`
	DailyCount   int64     `json:"daily_count" name:"daily_count" type:"field" sql:"BIGINT NOT NULL"`
	OrderItems   []Sales   `json:"order_items" name:"order_items" type:"field" sql:"JSONB"`
	Poster       string    `json:"poster" name:"poster" type:"field" sql:"VARCHAR NOT NULL"`
	Branch       string    `json:"branch" name:"branch" type:"field" sql:"VARCHAR NOT NULL"`
	StkLocation  string    `json:"stk_Location"`
	DispBy       string    `json:"disp_by" name:"disp_by" type:"field" sql:"VARCHAR NOT NULL DEFAULT 'nan'"`
	DispTime     time.Time `json:"disp_time" name:"disp_time" type:"field" sql:"TIMESTAMPTZ"`
	CompanyID    int64     `json:"company_id" name:"company_id" type:"field" sql:"BIGINT NOT NULL"`
	TillNum      int64     `json:"till_num" name:"till_num" type:"field" sql:"BIGINT NOT NULL"`
	PayTill      int64     `json:"pay_till" name:"pay_till" type:"field" sql:"BIGINT NOT NULL DEFAULT '0'"`
	Receipt      int64     `json:"receipt" name:"receipt" type:"field" sql:"BIGINT NOT NULL DEFAULT '0'"`
	ReceiptNum   int64     `json:"receipt_num" name:"receipt_num" type:"field" sql:"BIGINT NOT NULL DEFAULT '0'"`
	AcNum        string    `json:"ac_num" name:"ac_num" type:"field" sql:"VARCHAR NOT NULL DEFAULT 'pending'"`
	State        string    `json:"state" name:"state" type:"field" sql:"VARCHAR NOT NULL DEFAULT 'pending'"`
	Elapsed      float64   `json:"elapsed"`
	Total        float64   `json:"total"`
}

// OrderItem is a variable for current order
type OrderItem struct {
	ItemCode string  `json:"item_code"`
	ItemName string  `json:"item_name"`
	Quantity float64 `json:"quantity"`
	Price    float64 `json:"price"`
	Cost     float64 `json:"cost"`
	Total    float64 `json:"total"`
	VatAlpha string  `json:"vat_alpha"`
	VatPerc  float64 `json:"vat_perc"`
	Vat      float64 `json:"vat"`
	State    string  `json:"state"`
	Poster   string  `json:"poster"`
	OrderNum string  `json:"order_num"`
	TxnTime  string  `json:"txn_time"`
}

// OrderCategories

// genOrderTable
func genOrderTable() error {
	var tblStruct Order
	return db.CreateFromStruct(tblStruct)
}

// nextOrder fetches the next available order number
func (ord *Order) NextOrder() error {
	fmt.Println("\n\t\t ac_num =", ord.AcNum)
	fmt.Println("\t\t poster =", ord.Poster)
	fmt.Println("\t\t receipt =", ord.ReceiptNum)

	// return nil
	sql := `SELECT 
				coalesce(max(order_num), 0) 
			FROM salesorders 
			WHERE state = 'pending' 
				AND till_num::varchar = (
					SELECT 
						till_num::varchar 
					FROM users 
					WHERE username = $1 LIMIT 1)
				AND receipt_num = $2`

	rows, err := db.PgPool.Query(context.Background(), sql, ord.Poster, ord.ReceiptNum)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&ord.OrderNum)
		if err != nil {
			return err
		}
	}

	return nil
}

// NewOrder generates a new sales order
func (ord *Order) NewOrder() error {
	var err error
	err = ord.NextOrder()
	if err != nil {
		fmt.Println("error newOrder    err =", err)
		return err
	}

	if ord.OrderNum > 0 {
		return nil
	}

	if ord.Poster == "" {
		return fmt.Errorf(("order error poster is null"))
	}

	if ord.AcNum == "" {
		ord.AcNum = fmt.Sprintf("%v", ord.ReceiptNum)
	}

	// create a new order if there is no active order
	sql := `INSERT INTO salesorders(order_num, daily_count, till_num, poster, branch, company_id, ac_num, receipt_num)
			SELECT CAST(CONCAT(
							extract(YEAR FROM now()), 
							LPAD(EXTRACT(MONTH FROM now())::text, 2, '0'), 
							LPAD(EXTRACT(DAY FROM now())::text, 2, '0'), 
							(SELECT CONCAT(company_id, branch_id) FROM branches WHERE branch_name = (SELECT branch FROM users WHERE username = $1) LIMIT 1), 
							cast(coalesce(max(daily_count), 0) + 1 as varchar), cast(0 as varchar) 
						) AS BIGINT) as order_num
					, coalesce(max(daily_count), 0) + 1 as daily_count
					, (SELECT cast(till_num as bigint) FROM users WHERE username = $1) as till_num
					, $1 as poster
					, (SELECT branch::varchar FROM users WHERE username = $1) as branch
					, (SELECT company_id FROM users WHERE username = $1) as company_id
					, $2
					, $3
				FROM salesorders 
				WHERE trans_date::date = (SELECT now()::date) AND 
				company_id = (SELECT coalesce(company_id, 0) FROM users WHERE username = $1) AND 
				branch = (SELECT branch FROM users WHERE username = $1)
			RETURNING order_num`

	rows, err := db.PgPool.Query(context.Background(), sql, ord.Poster, ord.AcNum, ord.ReceiptNum)
	if err != nil {
		fmt.Println("sale.Orders->NewOrder() query error     err =", err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&ord.OrderNum)
		if err != nil {
			return err
		}
	}

	return nil
}

// FetchOrderItems gets all items in order
func (ord *Order) Fetchtems() error {
	sql := `SELECT 
				cast(coalesce(order_items, '[]') as varchar) 
			FROM salesorders 
			WHERE order_num = $1`

	rows, err := db.PgPool.Query(context.Background(), sql, ord.OrderNum)
	if err != nil {
		return err
	}
	defer rows.Close()

	var orderItems string
	for rows.Next() {
		err := rows.Scan(&orderItems)
		if err != nil {
			return err
		}
	}

	err = json.Unmarshal([]byte(orderItems), &ord.OrderItems)
	if err != nil {
		log.Println("failed to unmarshal orderItems to json    err =", err)
		return err
	}

	return nil
}

// FetchOrderItems gets all items in order
func (ord *Order) FetchtemsCtx(ctx context.Context, tx pgx.Tx) error {
	sql := `SELECT 
				cast(coalesce(order_items, '[]') as varchar) 
			FROM salesorders 
			WHERE order_num = $1`

	rows, err := db.PgPool.Query(ctx, sql, ord.OrderNum)
	if err != nil {
		return err
	}
	defer rows.Close()

	var orderItems string
	for rows.Next() {
		err := rows.Scan(&orderItems)
		if err != nil {
			return err
		}
	}

	err = json.Unmarshal([]byte(orderItems), &ord.OrderItems)
	if err != nil {
		log.Println("failed to unmarshal orderItems to json    err =", err)
		return err
	}

	return nil
}

// FetchPayingOrderItems gets all items in order
func FetchPayingOrderItems(tillNum string) ([]Sales, error) {
	if tillNum == "" || tillNum == "00" {
		return nil, fmt.Errorf("failed to fetch paying order till num")
	}
	sql := `SELECT 
				cast(coalesce(order_items, '[]') as varchar) 
			FROM salesorders 
			WHERE state = 'paying' AND till_num = $1`

	var values []Sales
	rows, err := db.PgPool.Query(context.Background(), sql, tillNum)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orderItems string
	for rows.Next() {
		err := rows.Scan(&orderItems)
		if err != nil {
			return values, err
		}

		var val []Sales
		err = json.Unmarshal([]byte(orderItems), &val)
		if err != nil {
			return nil, err
		}

		values = append(values, val...)
	}

	return values, nil
}

// FetchPayingOrderItems gets all items in order
func (arg *ReceiptLog) CombineOrdersInBill() error {
	if arg.ReceiptNum == 0 {
		return fmt.Errorf("error ReceiptLog->CombineOrdersInBill()    receipt_num is null")
	}

	sql := `SELECT 
				cast(coalesce(order_items, '[]') as varchar) 
			FROM salesorders 
			WHERE state in ('dispatched') AND receipt_num = $1`

	// var values []Sales
	rows, err := db.PgPool.Query(context.Background(), sql, arg.ReceiptNum)
	if err != nil {
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

// FetchActiveOrders gets all orders not paid yet
func FetchActiveOrders(poster string) ([]Order, error) {
	// userDetails, err := login.FetchUser(poster)
	// if err != nil {
	// 	return nil, err
	// }

	// state := `'pending', 'dispatched', 'paying'`
	// if userDetails.AcceptPayment {
	// 	state = `'pending', 'dispatched', 'paying'`
	// }

	sql := `SELECT 
				order_num
				, poster 
				, state
				, ac_num
			FROM salesorders 
			WHERE state IN ('pending', 'dispatched', 'paying') AND till_num = (SELECT cast(till_num as bigint) FROM users WHERE username = $1)
			ORDER BY trans_date ASC
			`

	rows, err := db.PgPool.Query(context.Background(), sql, poster)
	if err != nil {
		fmt.Println("failed to query orders.  error =", err)
		return nil, err
	}
	defer rows.Close()

	var values []Order
	for rows.Next() {
		var r Order
		err = rows.Scan(&r.OrderNum, &r.Poster, &r.State, &r.AcNum)
		if err != nil {
			fmt.Println("error scanning active orders err =", err)
		}

		values = append(values, r)
	}
	return values, nil
}

// FetchActiveOrders gets all orders not paid yet
func FetchActiveOrdersInBill(receipt string) ([]Order, error) {

	sql := `SELECT
                order_num
	            , daily_count
				, poster 
				, state
				, ac_num
				, receipt_num
			FROM salesorders 
			WHERE 
				state IN ('pending', 'dispatched', 'paying') AND 
				receipt_num = $1
			ORDER BY trans_date ASC
			`

	rows, err := db.PgPool.Query(context.Background(), sql, receipt)
	if err != nil {
		fmt.Println("failed to query orders.  error =", err)
		return nil, err
	}
	defer rows.Close()

	var values []Order
	for rows.Next() {
		var r Order
		err = rows.Scan(&r.OrderNum, &r.DailyCount, &r.Poster, &r.State, &r.AcNum, &r.ReceiptNum)
		if err != nil {
			fmt.Println("error scanning active orders err =", err)
		}

		values = append(values, r)
	}
	return values, nil
}

// AddToOrder adds a new item to orders
func (ord *Order) AddToOrder(args Sales) ([]Sales, float64, error) {
	if ord.ReceiptNum == 0 {
		return nil, 0, fmt.Errorf("error. Order->AddToOrder()    null order num")
	}
	if ord.ReceiptNum == 0 {
		return nil, 0, fmt.Errorf("error. Order->AddToOrder()    null receipt")
	}

	// get order item
	err := ord.Fetchtems()
	if err != nil {
		fmt.Println("error fetching order items error =", err)
		return nil, 0, err
	}

	orderItems := ord.OrderItems
	args.ReceiptItem = fmt.Sprintf("%v-%v", ord.OrderNum, (len(orderItems) + 1))

	if args.State == "" {
		args.State = "pending"
	}

	// append to order
	orderItems = append(orderItems, args)

	// marshal items to json string
	jStr, err := json.Marshal(orderItems)
	if err != nil {
		return nil, 0, err
	}

	fmt.Println("order items =", orderItems)

	// update order item
	sql := `UPDATE salesorders 
			SET 
				order_items = $1 
			WHERE order_num = $2 
			RETURNING cast(coalesce(order_items, '[]') as varchar)`
	rows, err := db.PgPool.Query(context.Background(), sql, jStr, ord.OrderNum)
	if err != nil {
		fmt.Println("error fetching order items err =", err)
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var order_items string
		err = rows.Scan(&order_items)
		if err != nil {
			fmt.Println("error scanning return columns   err =", err)
		}

		err = json.Unmarshal([]byte(order_items), &orderItems)
		if err != nil {
			fmt.Println("failed to get json   err =", err)
		}
	}

	total := OrderTotal(orderItems)

	return orderItems, total, nil
}

// OrderTotal
func OrderTotal(order []Sales) float64 {
	var total float64

	if order != nil {
		for _, itm := range order {
			if itm.State != "DELETED" && itm.State != "VOIDED" {
				total += (itm.Quantity * itm.Price)
			}
		}
	} else {
		total = 0
	}

	return total
}

// CompleteOrder completes an order
func (ord *Order) CompleteOrder() ([]OrderItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	tx, err := db.PgPool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	fmt.Println("\t order_num =", ord.OrderNum)

	// if kitchen is enabled complete order state should be    'ordered'
	// else  state = 'dispatched'
	state := "ordered"
	if !variables.ProductionDisp {
		state = "dispatched"
	}

	err = ord.FetchtemsCtx(ctx, tx)
	if err != nil {
		fmt.Printf("\n\tfailed to complete order for order_num = %v fetch error = %v \n", ord.OrderNum, err)
		return nil, err
	}

	if ord.OrderItems == nil || len(ord.OrderItems) == 0 {
		return nil, fmt.Errorf("error order is empty")
	}

	sql := `UPDATE salesorders 
			SET 
				state = $2
				, complete_time = now() 
			WHERE order_num = $1 `

	_, err = database.PgPool.Exec(ctx, sql, ord.OrderNum, state)
	if err != nil {
		fmt.Printf("\n\tfailed to complete order for order_num = %v error = %v \n", ord.OrderNum, err)
		return nil, err
	}

	voucher, err := ord.VoucherCtx(ctx, tx)
	if err != nil {
		return nil, err
	}

	tx.Commit(ctx)

	return voucher, nil
}

// OrderVoucher returns order details
func (ord *Order) VoucherCtx(ctx context.Context, tx pgx.Tx) ([]OrderItem, error) {
	sql := `SELECT items.item_name, SUM(items.quantity) as qty, items.price, SUM(items.quantity * items.price) as total
				, items.order_num
				, (SELECT poster FROM salesorders WHERE order_num = $1) 
				, (SELECT trans_date FROM salesorders WHERE order_num = $1)
			FROM salesorders ord, jsonb_to_recordset(ord.order_items) as  
				items(
					item_code varchar
					, item_name varchar
					, quantity float
					, price float
					, order_num bigint
					, state varchar 
				)
			WHERE ord.order_num = $1 AND items.state = 'pending'
			GROUP BY items.item_name, items.price, items.order_num `

	rows, err := db.PgPool.Query(ctx, sql, ord.OrderNum)
	if err != nil {
		log.Println("error fetching order voucher err =", err)
		return nil, err
	}
	defer rows.Close()

	var values []OrderItem
	for rows.Next() {
		var r OrderItem
		var t time.Time
		rows.Scan(&r.ItemName, &r.Quantity, &r.Price, &r.Total, &r.OrderNum, &r.Poster, &t)

		r.TxnTime = fmt.Sprintf("%d-%02d-%02d %02d:%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute())

		values = append(values, r)
	}

	return values, nil
}

// OrderVoucher returns order details
func OrderVoucher(orderNum string) ([]OrderItem, error) {
	sql := `SELECT items.item_name, SUM(items.quantity) as qty, items.price, SUM(items.quantity * items.price) as total
				, items.order_num
				, (SELECT poster FROM salesorders WHERE order_num = $1) 
				, (SELECT trans_date FROM salesorders WHERE order_num = $1)
			FROM salesorders ord, jsonb_to_recordset(ord.order_items) as  
				items(
					item_code varchar
					, item_name varchar
					, quantity float
					, price float
					, order_num bigint
					, state varchar 
				)
			WHERE ord.order_num = $1 AND items.state = 'pending'
			GROUP BY items.item_name, items.price, items.order_num `

	rows, err := db.PgPool.Query(context.Background(), sql, orderNum)
	if err != nil {
		log.Println("error fetching order voucher err =", err)
		return nil, err
	}
	defer rows.Close()

	var values []OrderItem
	for rows.Next() {
		var r OrderItem
		var t time.Time
		rows.Scan(&r.ItemName, &r.Quantity, &r.Price, &r.Total, &r.OrderNum, &r.Poster, &t)

		r.TxnTime = fmt.Sprintf("%d-%02d-%02d %02d:%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute())

		values = append(values, r)
	}

	return values, nil
}

// OrdIsDeletable checks if an order can be deleted
func OrdIsDeletable(ordNum string) bool {
	sql := `SELECT 
				CASE 
					WHEN state = 'pending' THEN true
					ELSE false
				END as is_deletable
			FROM salesorders WHERE order_num = $1`

	var isDelete bool
	rows, err := db.PgPool.Query(context.Background(), sql, ordNum)
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		rows.Scan(&isDelete)
	}

	return isDelete
}

// DelOrderItem deletes an order item
func DelOrderItem(orderItem, orderNum string) ([]Sales, float64, error) {
	ordNum, _ := strconv.ParseInt(orderNum, 10, 64)
	ord := Order{OrderNum: ordNum}

	// fetch order items
	err := ord.Fetchtems() //  FetchOrderItems(orderNum)
	if err != nil {
		return nil, 0, err
	}

	for i, itm := range ord.OrderItems {
		if itm.ReceiptItem == orderItem {
			if itm.State == "pending" {
				fmt.Println("\tdelete order_item =", itm.ReceiptItem)
				ord.OrderItems[i].State = "DELETED"
			}
		}
	}

	// marshal items to json string
	jStr, err := json.Marshal(ord.OrderItems)
	if err != nil {
		return nil, 0, err
	}

	// update order item
	sql := `UPDATE salesorders SET order_items = $1 WHERE order_num = $2 AND state = 'pending'
			RETURNING cast(coalesce(order_items, '[]') as varchar)`
	rows, err := db.PgPool.Query(context.Background(), sql, jStr, orderNum)
	if err != nil {
		fmt.Println("error fetching order items error =", err)
		return nil, 0, err
	}
	defer rows.Close()

	var cart []Sales
	for rows.Next() {
		var order_items string
		err = rows.Scan(&order_items)
		if err != nil {
			fmt.Println("error scanning return columns   err =", err)
		}

		err = json.Unmarshal([]byte(order_items), &cart)
		if err != nil {
			fmt.Println("failed to get json   err =", err)
		}
	}

	total := OrderTotal(cart)

	return cart, total, nil
}

// SetOrderPay sets an order to paying
func SetOrderPay(ordNum string, receipt int64) error {
	ctx := context.Background()
	tx, err := db.PgPool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	fmt.Println("receipt num =", receipt)

	sql := `INSERT INTO sales_live(trans_date, item_code, item_name
				, receipt_num, quantity, cost, price
				, vat_alpha, vat_perc, vat, served_by, branch, till_num
				, sale_type, approved_by, trace, company_id, state, sc_id, order_num, receipt_item)
			SELECT items.trans_date, items.item_code, items.item_name 
				, cast($1 as bigint) as receipt_num, items.quantity, items.cost, items.price 
				, items.vat_alpha, items.vat_perc, items.vat, items.served_by, items.branch, items.till_num 
				, items.sale_type, items.approved_by
				, coalesce(items.trace, 0) trace, items.company_id, items.state, items.sc_id, items.order_num, items.receipt_item
			FROM salesorders ord, jsonb_to_recordset(ord.order_items) as 
				items(trans_date timestamp
					, till_num bigint
					, served_by varchar
					, order_num bigint
					, company_id bigint
					, branch varchar
					, item_code varchar
					, item_name varchar
					, quantity float
					, price float
					, cost float
					, vat_alpha varchar
					, vat_perc float
					, vat float
					, sale_type varchar
					, sc_id varchar
					, receipt_item varchar
					, trace BIGINT
					, approved_by varchar
					, state varchar)

			WHERE ord.order_num = $2 
			ON CONFLICT (receipt_item) 
			DO NOTHING
				/*UPDATE SET quantity = excluded.quantity
						, cost = excluded.cost
						, price = excluded.price
						, vat_alpha = excluded.vat_alpha
						, vat_perc = excluded.vat_perc
						, vat = excluded.vat 
						, receipt_num = $1
					WHERE 
					(select state FROM salestrace WHERE 
						receipt_num = 
							(select max(receipt_num) FROM sales_live WHERE order_num = $2  )
					) = 'pending' */`
	fmt.Println(sql)

	_, err = tx.Exec(ctx, sql, receipt, ordNum)
	if err != nil {
		fmt.Println("error inserting into live_sales  err =", err)
		tx.Rollback(ctx)
		return err
	}

	sql = `UPDATE salesorders SET state = 'paying', receipt = $1 WHERE order_num = $2`
	_, err = tx.Exec(ctx, sql, receipt, ordNum)
	if err != nil {
		log.Println("failed setting order to paying err =", err)
		tx.Rollback(ctx)
		return err
	}

	tx.Commit(ctx)
	return nil
}

// combines existing orders into a single bill
func (arg *Order) CombineBill() error {
	var err error
	var rcpt ReceiptLog

	if arg.Receipt == 0 {
		rcpt.Poster = arg.Poster
		rcpt.TillNum = arg.TillNum

		arg.OrderNum, err = rcpt.GenReceipt()
		if err != nil {
			return fmt.Errorf("sales.Order->CombineBill(). failed to get next open order")
		}
	}

	if arg.OrderNum == 0 {
		return fmt.Errorf("sales.Order->CombineBill(). order num is null")
	}

	err = arg.addOrderToReceipt()
	if err != nil {
		return fmt.Errorf("sales.Order->CombineBill(). error combining orders")
	}

	return nil
}

// addOrderToReceipt adds current order items into receipt
// Updates receipt number to salesorders
// returns an error if it fails
func (arg *Order) addOrderToReceipt() error {
	sql := `UPDATE salesorders 
			SET 
				receipt = $1 
			WHERE order_num = $2`

	_, err := db.PgPool.Query(context.Background(), sql, arg.Receipt, arg.OrderNum)
	if err != nil {
		log.Println("sales.Order->addOrderToReceipt()    error =", err)
		return err
	}
	return nil
}

// OrderToSales adds current order_items into sales_live
func OrderToSales(orders []Sales, ords []string, receipt int64, username string) (float64, error) {
	ctx := context.Background()
	tx, err := db.PgPool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}

	// update receipt number for current sales records
	sql := `UPDATE sales_live SET receipt_num = $1, sale_type = 'cash sale' WHERE 
			order_num IN (SELECT order_num FROM salesorders WHERE state = 'paying' AND till_num = (SELECT cast(till_num as bigint) FROM users WHERE username = $2 ) )`
	_, err = tx.Exec(ctx, sql, receipt, username)
	if err != nil {
		fmt.Println("error updating receipt_num to sales_records  err =", err)
		tx.Rollback(ctx)
		return 0, err
	}

	total := OrderTotal(orders)
	fmt.Println("total =", total)

	for i, order := range orders {
		if order.State != "deleted" && order.State != "DELETED" && order.State != "VOIDED" && order.State != "voided" {
			order.State = "pending"
			orders[i] = order
		}
	}

	// update cart in salestrace
	jStr, _ := json.Marshal(orders)
	sql = `UPDATE salestrace SET cart = $2, state = 'closed_bill', total = $3 WHERE receipt_num = $1`
	_, err = tx.Exec(ctx, sql, receipt, jStr, total)
	if err != nil {
		fmt.Println("error updating salestrace  err =", err)
		tx.Rollback(ctx)
		return 0, err
	}

	tx.Commit(ctx)
	return total, nil
}

// OrdersToPay joins orders into sales
func (arg *ReceiptLog) OrdersToPay(orders []string, receipt int64) error {

	salesCarts, err := FetchPayingOrderItems(fmt.Sprintf("%v", arg.TillNum))
	if err != nil {
		log.Println("error. failed to fetch paying order items    err =", err)
		return nil
	}

	_, err = OrderToSales(salesCarts, orders, receipt, arg.Poster)
	if err != nil {
		fmt.Println("failed order to sales err =", err)
		return err
	}

	return nil
}

// CloseBill joins orders in bill to sale
func (arg *ReceiptLog) CloseBill() error {
	if arg.ReceiptNum == 0 {
		return fmt.Errorf("error. ReceiptLog->CloseBill()    receipt is null")
	}

	err := arg.CombineOrdersInBill()
	if err != nil {
		return fmt.Errorf("error. ReceiptLog->CloseBill()    failed to combine orders in bill")
	}

	err = arg.UpdateCart("")
	if err != nil {
		return fmt.Errorf("error. ReceiptLog->CloseBill()    %v", err)
	}

	err = arg.CloseOrders()
	if err != nil {
		return fmt.Errorf("error, ReceiptLog->CloseBill()    %v", err)
	}

	return nil
}

func (arg *ReceiptLog) UpdateCart(state string) error {
	if arg.Cart == nil {
		return fmt.Errorf("error. ReceiptLog->UpdateCart()    null cart")
	}

	if arg.ReceiptNum == 0 {
		return fmt.Errorf("error. ReceiptLog->UpdateCart()    null receipt")
	}

	if state == "" {
		state = "pending payment"
	}

	arg.Total = 0
	for _, row := range arg.Cart {
		if row.State == "pending" {
			arg.Total += float32(row.Quantity * row.Price)
		}
	}

	cart, _ := json.Marshal(arg.Cart)

	sql := `UPDATE salestrace 
			SET 
				cart = $1
				, total = $2
				, state = $3
				, last_updated = now() 
			WHERE receipt_num = $4 
			RETURNING poster`
	rows, err := db.PgPool.Query(context.Background(), sql, string(cart), arg.Total, state, arg.ReceiptNum)
	if err != nil {
		log.Println("sql error. ReceiptLog->UpdateCart()    err =", err)
		return fmt.Errorf("error. ReceiptLog->UpdateCart()    sql error")
	}
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&arg.Poster)
		if err != nil {
			return err
		}
	}
	return nil
}

func (arg *ReceiptLog) CloseOrders() error {
	sql := "UPDATE salesorders SET state = 'paying' WHERE receipt_num = $1 AND state = 'dispatched'"

	_, err := db.PgPool.Exec(context.Background(), sql, arg.ReceiptNum)
	if err != nil {
		log.Println("sql error. ReceiptLog->CloseOrders()    err =", err)
		return fmt.Errorf("error. ReceiptLog->CloseOrders()    sql error")
	}
	return nil
}

// ExcFromRcpt removes current order from receipt
func ExcFromRcpt(orderNum string) error {
	return nil
}
