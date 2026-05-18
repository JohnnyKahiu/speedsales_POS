package sales

import (
	"context"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
)

// Cashmovement holds information on movement of cash
type Cashmovement struct {
	table        string    `name:"cash_movement" type:"table" `
	AutoID       int64     `json:"auto_id" name:"auto_id" type:"field" sql:"BIGSERIAL PRIMARY KEY "`
	TranTime     time.Time `json:"tran_time" name:"tran_time" type:"field" sql:"TIMESTAMPTZ NOT NULL DEFAULT now()"`
	Type         string    `json:"type" name:"type" type:"field" sql:"VARCHAR(50) NOT NULL"`
	CompanyID    int64     `json:"company_id" name:"company_id" type:"field" sql:"BIGINT NOT NULL"`
	Branch       string    `json:"branch" name:"branch" type:"field" sql:"VARCHAR(30) NOT NULL "`
	Teller       string    `json:"teller" name:"teller" type:"field" sql:"VARCHAR(50) NOT NULL "`
	Supervisor   string    `json:"supervisor" name:"supervisor" type:"field" sql:"VARCHAR(50)"`
	TillNum      int64     `json:"till_num" name:"till_num" type:"field" sql:"BIGINT NOT NULL"`
	Amount       float64   `json:"amount" name:"amount" type:"field" sql:"DECIMAL NOT NULL DEFAULT 0"`
	ConfirmState string    `json:"confirm_state" name:"confirm_state" type:"field" sql:"VARCHAR(30) NOT NULL DEFAULT 'pending'"`
	ConfirmedBy  string    `json:"confirmed_by" name:"confirmed_by" type:"field" sql:"VARCHAR(50)"`
	ConfirmTime  time.Time `json:"confirm_time" name:"confirm_time" type:"field" sql:"TIMESTAMPTZ"`
	TxnTime      string    `json:"txn_time"`
}

func genCashTable() error {
	return database.CreateFromStruct(Cashmovement{})
}

func (arg *Cashmovement) Rollup(ctxt context.Context) error {

	return nil
}
