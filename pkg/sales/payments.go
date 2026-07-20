package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/broker"
)

// TillPayment records every payment transaction processed at a till.
type TillPayment struct {
	table      string    `name:"till_payments" type:"table"`
	TxnID      int64     `json:"txn_id"       name:"txn_id"      type:"field" sql:"BIGSERIAL"`
	TransTime  time.Time `json:"trans_time"   name:"trans_time"  type:"field" sql:"TIMESTAMPTZ NOT NULL DEFAULT now()"`
	PaymentFor string    `json:"payment_for"  name:"payment_for" type:"field" sql:"VARCHAR(30) NOT NULL DEFAULT 'cash_sale'"` // cash_sale | laybye_payment | credit_pay
	TillNum    int64     `json:"till_num"     name:"till_num"    type:"field" sql:"BIGINT NOT NULL DEFAULT 0"`
	LaybyeID   int64     `json:"laybye_id"    name:"laybye_id"   type:"field" sql:"BIGINT NOT NULL DEFAULT 0"`
	CreditID   int64     `json:"credit_id"    name:"credit_id"   type:"field" sql:"BIGINT NOT NULL DEFAULT 0"`
	SaleID     int64     `json:"sale_id"      name:"sale_id"     type:"field" sql:"BIGINT NOT NULL DEFAULT 0"`
	Cash       float64   `json:"cash"         name:"cash"        type:"field" sql:"DECIMAL NOT NULL DEFAULT 0"`
	Mpesa      float64   `json:"mpesa"        name:"mpesa"       type:"field" sql:"DECIMAL NOT NULL DEFAULT 0"`
	Ecard      float64   `json:"ecard"        name:"ecard"       type:"field" sql:"DECIMAL NOT NULL DEFAULT 0"`
	Cheque     float64   `json:"cheque"       name:"cheque"      type:"field" sql:"DECIMAL NOT NULL DEFAULT 0"`
	CashInTill float64   `json:"cash_in_till" name:"cash_in_till" type:"field" sql:"DECIMAL NOT NULL DEFAULT 0"`
	CashOut    float64   `json:"cash_out"     name:"cash_out"    type:"field" sql:"DECIMAL NOT NULL DEFAULT 0"`
	pkey       string    `name:"pkey_till_pay" type:"constraint" sql:"PRIMARY KEY(txn_id)"`
}

func genPaymentsTable() error {
	return database.CreateFromStruct(TillPayment{})
}

// Record inserts a new payment transaction row.
func (p *TillPayment) Record(ctx context.Context) error {
	sql := `INSERT INTO till_payments
				(trans_time, payment_for, till_num, laybye_id, credit_id, sale_id,
				 cash, mpesa, ecard, cheque, cash_in_till, cash_out)
			VALUES
				(now(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING txn_id, trans_time`

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	row := database.PgPool.QueryRow(ctx, sql,
		p.PaymentFor,
		p.TillNum,
		p.LaybyeID,
		p.CreditID,
		p.SaleID,
		p.Cash,
		p.Mpesa,
		p.Ecard,
		p.Cheque,
		p.CashInTill,
		p.CashOut,
	)

	return row.Scan(&p.TxnID, &p.TransTime)
}

// RecordAndPublish persists the payment and publishes it to the "payments_in" topic
// (keyed by txn_id) for the Accounts service to consume, regardless of whether it's
// a cash_sale, credit_pay, or laybye_payment. Publish failures are logged, not
// returned: the payment itself is already durably recorded in till_payments.
func (p *TillPayment) RecordAndPublish(ctx context.Context) error {
	if err := p.Record(ctx); err != nil {
		return err
	}

	payload, err := json.Marshal(p)
	if err != nil {
		log.Println("error. failed to marshal till_payment for publish    err =", err)
		return nil
	}

	kf := broker.Kafka{
		Broker:  os.Getenv("KAFKA_BROKER"),
		Topic:   "payments_in",
		Key:     fmt.Sprintf("%v", p.TxnID),
		Payload: payload,
	}
	if err := kf.Produce(ctx); err != nil {
		log.Println("kafka error    failed to publish payments_in event    err =", err)
	}

	return nil
}

// FetchByTill returns all payment records for a given till, newest first.
func FetchPaymentsByTill(ctx context.Context, tillNum int64) ([]TillPayment, error) {
	sql := `SELECT
				txn_id, trans_time, payment_for, till_num,
				laybye_id, credit_id, sale_id,
				cash, mpesa, ecard, cheque, cash_in_till, cash_out
			FROM till_payments
			WHERE till_num = $1
			ORDER BY trans_time DESC`

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rows, err := database.PgPool.Query(ctx, sql, tillNum)
	if err != nil {
		log.Println("FetchPaymentsByTill query error:", err)
		return nil, err
	}
	defer rows.Close()

	var results []TillPayment
	for rows.Next() {
		var p TillPayment
		if err := rows.Scan(
			&p.TxnID, &p.TransTime, &p.PaymentFor, &p.TillNum,
			&p.LaybyeID, &p.CreditID, &p.SaleID,
			&p.Cash, &p.Mpesa, &p.Ecard, &p.Cheque, &p.CashInTill, &p.CashOut,
		); err != nil {
			return nil, err
		}
		results = append(results, p)
	}

	return results, nil
}

// FetchPaymentsBySale returns all payment records for a given sale_id (receipt_num).
func FetchPaymentsBySale(ctx context.Context, saleID int64) ([]TillPayment, error) {
	sql := `SELECT
				txn_id, trans_time, payment_for, till_num,
				laybye_id, credit_id, sale_id,
				cash, mpesa, ecard, cheque, cash_in_till, cash_out
			FROM till_payments
			WHERE sale_id = $1
			ORDER BY trans_time ASC`

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rows, err := database.PgPool.Query(ctx, sql, saleID)
	if err != nil {
		log.Println("FetchPaymentsBySale query error:", err)
		return nil, err
	}
	defer rows.Close()

	var results []TillPayment
	for rows.Next() {
		var p TillPayment
		if err := rows.Scan(
			&p.TxnID, &p.TransTime, &p.PaymentFor, &p.TillNum,
			&p.LaybyeID, &p.CreditID, &p.SaleID,
			&p.Cash, &p.Mpesa, &p.Ecard, &p.Cheque, &p.CashInTill, &p.CashOut,
		); err != nil {
			return nil, err
		}
		results = append(results, p)
	}

	return results, nil
}
