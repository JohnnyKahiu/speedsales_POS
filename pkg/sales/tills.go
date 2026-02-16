package sales

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	pb "github.com/JohnnyKahiu/speed_sales_proto/user"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/grpc"
	"github.com/jackc/pgx/v5"
)

// CashSumm holds data about cash summary
type CashSumm struct {
	Cash     float64 `json:"cash"`
	Mobile   float64 `json:"mobile"`
	Ecard    float64 `json:"ecard"`
	Cheque   float64 `json:"cheque"`
	Returns  float64 `json:"returns"`
	Discount float64 `json:"discount"`
}

type Till struct {
	table           string    `name:"sales_till" type:"table"`
	AutoID          int32     `json:"auto_id" name:"auto_id" type:"field" sql:"BIGSERIAL PRIMARY KEY" `
	TillID          int32     `json:"till_id" name:"till_id" type:"field" sql:"INT" `
	CompanyID       int64     `json:"company_id" name:"company_id" type:"field" sql:"BIGINT NOT NULL DEFAULT '0'"`
	DailyID         int64     `json:"daily_id" name:"daily_id" type:"field" sql:"BIGINT NOT NULL DEFAULT '1'"`
	TillNO          int64     `json:"till_no" name:"till_no" type:"field" sql:"BIGINT NOT NULL UNIQUE"`
	OpenTime        time.Time `json:"open_time" name:"open_time" type:"field" sql:"TIMESTAMPTZ NOT NULL DEFAULT now()" `
	OpenFloat       float64   `json:"open_float" name:"open_float" type:"field" sql:"FLOAT NOT NULL DEFAULT '5000'" `
	Teller          string    `json:"teller" name:"teller" type:"field" sql:"VARCHAR NOT NULL" `
	Supervisor      string    `json:"supervisor" name:"supervisor" type:"field" sql:"VARCHAR NOT NULL" `
	Branch          string    `json:"branch" name:"branch" type:"field" sql:"VARCHAR" `
	CashOuts        float32   `json:"cash_outs" name:"cash_outs" type:"field" sql:"FLOAT NOT NULL DEFAULT '0'" `
	CashSummary     CashSumm  `json:"cash_summary" name:"cash_summary" type:"field" sql:"JSONB NOT NULL DEFAULT '{\"cash\":0, \"mpesa\":0, \"ecard\":0, \"cheque\":0, \"returns\":0, \"discount\":0}'" `
	ConfirmSummary  CashSumm  `json:"confirm_summary" name:"confirm_summary" type:"field" sql:"JSONB NOT NULL  DEFAULT '{\"cash\":0, \"mpesa\":0, \"ecard\":0, \"cheque\":0, \"returns\":0, \"discount\":0}'" `
	CloseTime       time.Time `json:"close_time" name:"close_time" type:"field" sql:"TIMESTAMPTZ" `
	CloseCash       float32   `json:"close_cash" name:"close_cash" type:"field" sql:"FLOAT " `
	CloseSupervisor string    `json:"close_supervisor" name:"close_supervisor" type:"field" sql:"VARCHAR"`
	AmendTime       time.Time `json:"amend_time" name:"amend_time" type:"field" sql:"TIMESTAMPTZ"`
	AmendAmount     float32   `json:"amend_amount" name:"amend_amount" type:"field" sql:"FLOAT NOT NULL DEFAULT '0'"`
	AmendReason     string    `json:"amend_reason" name:"amend_reason" type:"field" sql:"VARCHAR DEFAULT 'nan'"`
	AmendSupervisor string    `json:"amend_supervisor" name:"amend_supervisor" type:"field" sql:"VARCHAR DEFAULT 'nan'"`
	ConfirmedBy     string    `json:"confirmed_by" name:"confirmed_by" type:"name" sql:"VARCHAR NOT NULL DEFAULT 'nan'"`
	Confirmed       bool      `json:"confirmed" name:"confirmed" type:"field" sql:"BOOL NOT NULL DEFAULT 'false'"`
}

// genTillTbl generates a new till number
func genTillTbl() error {
	var tblStruct Till
	return database.CreateFromStruct(tblStruct)
}

// DBPool defines the interface for database operations needed by Till
type DBPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// New creates a new till
// inserts into sales_till table and returns the till number
// returns an error if it fails
func (arg *Till) New(ctx context.Context, db DBPool) error {
	query := `INSERT INTO sales_till (till_no, open_float, teller, supervisor, branch) 
				VALUES ($1, $2, $3, $4, $5) RETURNING till_no`
	return db.QueryRow(ctx, query, arg.TillNO, arg.OpenFloat, arg.Teller, arg.Supervisor, arg.Branch).Scan(&arg.TillNO)
}

// Exists checks if a till already exists for the given teller
// Fetches daily_id and till_no from sales_till table for the given teller
// returns true if the till exists, false otherwise
func (arg *Till) Exists(ctx context.Context) bool {
	sql := "SELECT daily_id, till_no FROM sales_till WHERE teller = $1 AND close_time IS NULL"

	rows, err := database.PgPool.Query(ctx, sql, arg.Teller)
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&arg.DailyID, &arg.TillNO)
		if err != nil {
			log.Println("error, failed to scan till_no     err =", err)
			return false
		}
	}

	return arg.TillNO != 0
}

// GetTillNum generates a new till number
// search from sales_till table for the max till_id for the given date and branch
// returns an error if it fails
func (arg *Till) GetTillNum(ctx context.Context, db DBPool) error {
	sql := `SELECT 
				coalesce(max(daily_id),0) + 1 
			FROM sales_till 
			WHERE 
				open_time::date = now()::date AND branch = $1`

	rows, err := database.PgPool.Query(ctx, sql, arg.Branch)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&arg.DailyID)
		if err != nil {
			log.Println("error. failed to scan till_no     err =", err)
			return err
		}
	}

	t := time.Now()
	arg.TillNO, _ = strconv.ParseInt(fmt.Sprintf("%d%02d%02d%v", t.Year(), t.Month(), t.Day(), arg.DailyID), 10, 64)

	return nil
}

// OpenTill creates a new till
// Updates logins a new till and returns the till number
// returns an error if it fails
func (arg *Till) OpenTill(db DBPool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if arg.Teller == "" || arg.Teller == "nan" {
		return errors.New("teller is required")
	}

	if arg.Supervisor == "" || arg.Supervisor == "nan" {
		return errors.New("supervisor is required")
	}

	if arg.Exists(ctx) {
		fmt.Println("till already exists")
		return nil
	}

	err := arg.GetTillNum(ctx, db)
	if err != nil {
		log.Println("error, failed to fetch till_num    err =", err)
		return err
	}

	// create the new till
	err = arg.New(ctx, db)
	if err != nil {
		log.Println("error, failed to create till    err =", err)
		return err
	}

	// update till to user
	err = arg.UpdateTill(ctx)
	if err != nil {
		log.Println("error, failed to update till    err =", err)
		return err
	}

	return nil
}

// UpdateTill
func (arg *Till) UpdateTill(ctx context.Context) error {

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	address := os.Getenv("LOGIN_RPC_ADDR")

	loginService, err := grpc.NewTillService(address)
	if err != nil {
		fmt.Println("failed to create login service    err =", err)
		return err
	}
	fmt.Println("loginService created")

	resp, err := loginService.UpdateTill(ctx, &pb.UpdateTillRequest{Username: arg.Teller, TillNum: arg.TillNO})
	if err != nil {
		return err
	}
	fmt.Println("response =", resp)

	return nil
}
