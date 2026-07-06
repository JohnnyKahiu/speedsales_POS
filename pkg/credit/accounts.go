package credit

import (
	"context"
	"log"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
)

type AcTrans struct {
	table             string  `name:"accounts_txn" type:"table"`
	AutoID            string  `json:"auto_id" type:"field" sql:"BIGSERIAL NOT NULL"`
	TransDate         string  `json:"trans_date" type:"field" sql:"TIMESTAMP NOT NULL DEFAULT now()"`
	AcNum             string  `json:"ac_num" type:"field" sql:"VARCHAR NOT NULL"`
	TillNum           int64   `json:"till_num" type:"field" sql:"BIGINT NOT NULL DEFAULT '0'"`
	Name              string  `json:"name" type:"field" sql:"VARCHAR"`
	TransType         string  `json:"trans_type" type:"field" sql:"VARCHAR NOT NULL"`
	TransReff         int64   `json:"trans_reff" type:"field" sql:"BIGINT"`
	Amount            float64 `json:"amount" type:"field" sql:"FLOAT NOT NULL DEFAULT '0.0'"`
	CashPaid          float64 `json:"cash_paid" type:"field" sql:"FLOAT NOT NULL DEFAULT '0.0'"`
	Paid              float64 `json:"paid" type:"field" sql:"FLOAT NOT NULL DEFAULT '0.0'"`
	PayDetails        Payment `json:"pay_details" type:"field" sql:"JSONB NOT NULL DEFAULT '{\"cash\":0,\"mpesa\":0,\"ecard\":0,\"check\":0}' "`
	ServedBy          string  `json:"served_by" type:"field" sql:"VARCHAR NOT NULL"`
	ApprovedBy        string  `json:"approved_by" type:"field" sql:"VARCHAR"`
	ConfirmedBy       string  `json:"confirmed_by" type:"field" sql:"VARCHAR"`
	ReceivedBy        string  `json:"received_by" type:"field" sql:"VARCHAR"`
	TransCompleteTime string  `json:"trans_complete_time" type:"field" sql:"TIMESTAMP NOT NULL DEFAULT now()"`
	ScCustomer        string  `json:"sc_customer"`
	Balance           float64 `json:"balance"`
	Branch            string  `json:"branch"`
	Approver          string  `json:"approver"`
	Password          string  `json:"password"`
	CustomerPin       string  `json:"customer_pin"`
}

func GenAccountsTxnTable() error {
	return database.CreateFromStruct(AcTrans{})
}

// AddAccountTxn adds a new transaction to the accounts table
func (a *AcTrans) AddAccountTxn(ctxt context.Context) error {
	ctx, cancel := context.WithTimeout(ctxt, 15*time.Second)
	defer cancel()

	sql := `INSERT INTO accounts_txn(trans_date, ac_num, till_num, name, trans_type, trans_reff, amount, paid, pay_details, served_by, approved_by, confirmed_by, received_by, trans_complete_time)
			VALUES(now(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) `

	_, err := database.PgPool.Exec(ctx, sql, a.AcNum, a.TillNum, a.Name, a.TransType, a.TransReff, a.Amount, a.Paid, a.PayDetails, a.ServedBy, a.ApprovedBy, a.ConfirmedBy, a.ReceivedBy, a.TransCompleteTime)
	if err != nil {
		log.Println("postgresql error. failed to add new accounts_txn       err =", err)
		return err
	}

	return nil
}

// GetBalance returns the balance of an account
func (a *AcTrans) GetBalance(ctxt context.Context) error {
	ctx, cancel := context.WithTimeout(ctxt, 15*time.Second)
	defer cancel()

	sql := `SELECT 
				SUM(amount) - SUM(paid) 
			FROM accounts_txn WHERE ac_num = $1`

	if err := database.PgPool.QueryRow(ctx, sql, a.AcNum).Scan(&a.Balance); err != nil {
		log.Println("postgresql error. failed to get balance       err =", err)
		return err
	}

	return nil
}
