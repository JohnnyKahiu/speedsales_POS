package sales

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
)

type GiftVoucher struct {
	table        string    `name:"gift_voucher" type:"table"`
	RegDate      time.Time `json:"reg_date" name:"reg_date" type:"field" sql:"TIMESTAMPTZ NOT NULL DEFAULT NOW()"`
	Serial       string    `json:"serial" name:"serial" type:"field" sql:"VARCHAR NOT NULL"`
	RegisteredBY string    `json:"registerd_by" name:"registerd_by" type:"field" sql:"VARCHAR NOT NULL"`
	Amount       float64   `json:"amount" name:"amount" type:"field" sql:"FLOAT NOT NULL"`
	TxnReceipt   int64     `json:"txn_receipt" name:"txn_receipt" type:"field" sql:"BIGINT NOT NULL DEFAULT '0'"`
	Teller       string    `json:"teller" name:"teller" type:"field" sql:"VARCHAR NOT NULL DEFAULT 'nil'"`
	ClaimerName  string    `json:"claimer_name" name:"claimer_name" type:"field" sql:"VARCHAR NOT NULL DEFAULT 'nil'"`
	ClaimerTel   string    `json:"claimer_tel" name:"claimer_tel" type:"field" sql:"VARCHAR NOT NULL DEFAULT 'nil'"`
	ClaimerID    string    `json:"claimer_id" name:"claimer_id" type:"field" sql:"VARCHAR NOT NULL DEFAULT 'nil'"`
	Approvers    []string  `json:"approvers" name:"approvers" type:"field" sql:"VARCHAR NOT NULL DEFAULT 'nil'"`
	constraint   string    `name:"gift_voucherPK" type:"constraint"  sql:"PRIMARY KEY(serial)`
	fkconstraint string    `name:"gift_voucherFk" type:"constraint"  sql:"FOREIGN KEY (registerd_by) REFERENCES users(username)`
}

func genGiftVoucherTbl() error {
	tbl := GiftVoucher{}
	return database.CreateFromStruct(tbl)
}

func (arg *GiftVoucher) Create() error {
	if arg.Amount <= 0 {
		return fmt.Errorf("no 0 amount gift voucher")
	}
	if arg.RegisteredBY == "" {
		return fmt.Errorf("provide the registerer's account")
	}

	sql := `INSERT INTO gift_voucher(serial, registerd_by, amount)
			VALUES($1, $2, $3)`

	_, err := database.PgPool.Exec(context.Background(), sql, arg.Serial, arg.RegisteredBY, arg.Amount)
	if err != nil {
		log.Println("error. failed to create a new gift voucher     err =", err)
		return err
	}
	return nil
}
