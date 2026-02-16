package authentication

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/pkg/grpc"
)

// User structure of user record
type User struct {
	table                string    `name:"users" type:"table"`
	AutoId               int64     `json:"auto_id" name:"auto_id" type:"field" sql:"BIGSERIAL PRIMARY KEY"`
	FirstName            string    `json:"first_name" name:"first_name" type:"field" sql:"VARCHAR"`
	LastName             string    `json:"last_name" name:"last_name" type:"field" sql:"VARCHAR"`
	OtherName            string    `json:"other_name" name:"other_name" type:"field" sql:"VARCHAR"`
	Telephone            string    `json:"telephone" name:"telephone" type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	Status               string    `json:"status" name:"status" type:"field" sql:"VARCHAR"`
	Username             string    `json:"username" name:"username" type:"field" sql:"VARCHAR NOT NULL UNIQUE"`
	Email                string    `json:"email" name:"email" type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	CompanyID            int64     `json:"company_id" name:"company_id" type:"field" sql:"BIGINT NOT NULL DEFAULT '0'"`
	UserClass            string    `json:"user_class" name:"user_class" type:"field" sql:"VARCHAR NOT NULL DEFAULT 'user'"`
	password             string    `name:"password" type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	RemoteLogin          bool      `json:"remote_login" name:"remote_login" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	AdoptStockcount      bool      `json:"adopt_stockcount" name:"adopt_stockcount" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	CompleteStockcount   bool      `json:"complete_stockcount" name:"complete_stockcount" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	StkLocation          string    `json:"stk_location" name:"stk_location" type:"field" sql:"VARCHAR NOT NULL DEFAULT 'shop'"`
	SessionID            string    `json:"session_id" name:"session_id" type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	MakeSales            bool      `json:"make_sales"`
	ApproveMakeSales     bool      `json:"approve_make_sales"`
	AcceptPayment        bool      `json:"accept_payment"`
	CashOffice           bool      `json:"cash_office"`
	CashRollups          bool      `json:"cash_rollups"`
	ApproveAcceptPayment bool      `json:"approve_accept_payment"`
	PostDispatch         bool      `json:"post_dispatch" name:"post_dispatch" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	ApproveDispatch      bool      `json:"approve_dispatch" name:"approve_dispatch" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	PostReceive          bool      `json:"post_receive" name:"post_receive" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	ApproveReceive       bool      `json:"approve_receive" name:"approve_receive" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	PostOrders           bool      `json:"post_orders" name:"post_orders" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	ApproveOrders        bool      `json:"approve_orders" name:"approve_orders" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	PriceChange          bool      `json:"price_change" name:"price_change" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantPriceChange     bool      `json:"grant_price_change" name:"grant_price_change" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	CreateStock          bool      `json:"create_stock" name:"create_stock" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	LinkStock            bool      `json:"link_stock" name:"link_stock" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	CompleteStockTake    bool      `json:"complete_stock_take" name:"complete_stock_take" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	Produce              bool      `json:"produce" name:"produce" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	TillOpened           time.Time `json:"till_opened" name:"till_opened" type:"field" sql:"TIMESTAMP "`
	Till                 bool      `json:"till" name:"till" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	TillNum              string    `json:"till_num" name:"till_num" type:"field" sql:"VARCHAR(50) "`
	Device               string    `json:"device" name:"device" type:"field" sql:"VARCHAR(50)"`
	Token                string    `json:"token" name:"token" type:"field" sql:"VARCHAR(150)"`
	TokenDate            time.Time `json:"token_date" name:"token_date" type:"field" sql:"TIMESTAMP"`
	Reset                bool      `json:"reset" name:"reset" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	Passcode             string    `json:"passcode"`
	SessionIDs           []string  `name:"session_ids" `
}

var mySigningKey = os.Getenv("JWT_KEY")

func ValidateJWT(tokenStr string) (User, bool) {
	address := os.Getenv("LOGIN_RPC_ADDR")

	loginSvc, err := grpc.NewAuthService(address)
	if err != nil {
		log.Println("failed to create login service: %v", err)
		return User{}, false
	}

	rights, isValid := loginSvc.ValidateUserToken(context.Background(), fmt.Sprintf("%v", tokenStr))
	if !isValid {
		log.Println("authorization failed: %v", err)
		return User{}, false
	}

	usr := User{}
	err = json.Unmarshal([]byte(rights), &usr)
	if err != nil {
		log.Println("failed to unmarshal user rights: %v", err)
		return User{}, false
	}

	fmt.Println("\n\n\t accept_payment =", usr.AcceptPayment)
	fmt.Println("\t make_sales =", usr.MakeSales)

	return usr, true
}
