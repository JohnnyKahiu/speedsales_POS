package logins

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/variables"
	"github.com/dgrijalva/jwt-go"
	"github.com/go-redis/redis"
)

// PasswordConfig password configuration
type PasswordConfig struct {
	Time    uint32
	Memory  uint32
	Threads uint8
	KeyLen  uint32
}

// Users structure of user record
type Users struct {
	table              string `name:"users" type:"table"`
	AutoId             int64  `json:"auto_id" name:"auto_id" type:"field" sql:"BIGSERIAL PRIMARY KEY"`
	FirstName          string `json:"first_name" name:"first_name" type:"field" sql:"VARCHAR"`
	LastName           string `json:"last_name" name:"last_name" type:"field" sql:"VARCHAR"`
	OtherName          string `json:"other_name" name:"other_name" type:"field" sql:"VARCHAR"`
	Telephone          string `json:"telephone" name:"telephone" type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	Status             string `json:"status" name:"status" type:"field" sql:"VARCHAR"`
	Username           string `json:"username" name:"username" type:"field" sql:"VARCHAR NOT NULL UNIQUE"`
	Email              string `json:"email" name:"email" type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	CompanyID          int64  `json:"company_id" name:"company_id" type:"field" sql:"BIGINT NOT NULL DEFAULT '0'"`
	UserClass          string `json:"user_class" name:"user_class" type:"field" sql:"VARCHAR NOT NULL DEFAULT 'user'"`
	Branch             string `json:"branch" name:"branch" type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	Department         string `json:"department" name:"department" type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	Role               string `json:"role" name:"role" type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	SubRole            string `json:"sub_role" name:"sub_role" type:"field" sql:"VARCHAR"`
	AccessLevel        string `json:"access_level" name:"access_level" type:"field" sql:"VARCHAR"`
	password           string `name:"password" type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	RemoteLogin        bool   `json:"remote_login" name:"remote_login" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	AdoptStockcount    bool   `json:"adopt_stockcount" name:"adopt_stockcount" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	CompleteStockcount bool   `json:"complete_stockcount" name:"complete_stockcount" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	PosSettings        bool   `json:"pos_settings" name:"pos_settings" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	StkLocation        string `json:"stk_location" name:"stk_location" type:"field" sql:"VARCHAR NOT NULL DEFAULT 'shop'"`
	SessionID          string `json:"session_id" name:"session_id" type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`

	PostDispatch         bool `json:"post_dispatch" name:"post_dispatch" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	ApproveDispatch      bool `json:"approve_dispatch" name:"approve_dispatch" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantPostDispatch    bool `json:"grant_post_dispatch" name:"grant_post_dispatch" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantApproveDispatch bool `json:"grant_approve_dispatch" name:"grant_approve_dispatch" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`

	PostReceive         bool `json:"post_receive" name:"post_receive" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	ApproveReceive      bool `json:"approve_receive" name:"approve_receive" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantPostReceive    bool `json:"grant_post_receive" name:"grant_post_receive" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantApproveReceive bool `json:"grant_approve_receive" name:"grant_approve_receive" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`

	PostOrders         bool `json:"post_orders" name:"post_orders" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	ApproveOrders      bool `json:"approve_orders" name:"approve_orders" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantPostOrders    bool `json:"grant_post_orders" name:"grant_post_orders" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantApproveOrders bool `json:"grant_approve_orders" name:"grant_approve_orders" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`

	AccessSalesReports      bool `json:"access_sales_reports" name:"access_sales_reports" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantAccessSalesReports bool `json:"grant_access_sales_reports" name:"grant_access_sales_reports" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`

	MakeSales          bool `json:"make_sales" name:"make_sales" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	ApproveSales       bool `json:"approve_sales" name:"approve_sales" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	AcceptPayment      bool `json:"accept_payment" name:"accept_payment" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantMakeSales     bool `json:"grant_make_sales" name:"grant_make_sales" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantApproveSales  bool `json:"grant_approve_sales" name:"grant_approve_sales" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantAcceptPayment bool `json:"grant_accept_payment" name:"grant_accept_payment" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`

	CashOffice              bool `json:"cash_office" name:"cash_office" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	CashRollups             bool `json:"cash_rollups" name:"cash_rollups" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	ApproveCashRollups      bool `json:"approve_cash_rollups" name:"approve_cash_rollups" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantCashRollups        bool `json:"grant_cash_rollups" name:"grant_cash_rollups" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantApproveCashRollups bool `json:"grant_approve_cash_rollups" name:"grant_approve_cash_rollups" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantCashOffice         bool `json:"grant_cash_office" name:"grant_cash_office" type:"field" sql:"BOOL NOT NULL DEFAULT 'false'"`

	SalesReturns             bool `json:"sales_returns" name:"sales_returns" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	ApproveSalesReturns      bool `json:"approve_sales_returns" name:"approve_sales_returns" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantSalesReturns        bool `json:"grant_sales_returns" name:"grant_sales_returns" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantApproveSalesReturns bool `json:"grant_approve_sales_returns" name:"grant_approve_sales_returns" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`

	Laybyes                 bool `json:"laybyes" name:"laybyes" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	ApproveCreditSales      bool `json:"approve_credit_sales" name:"approve_credit_sales" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantLaybyes            bool `json:"grant_laybyes" name:"grant_laybyes" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantApproveCreditSales bool `json:"grant_approve_credit_sales" name:"grant_approve_credit_sales" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`

	ActivateMpesa      bool `json:"activate_mpesa" name:"activate_mpesa" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantActivateMpesa bool `json:"grant_activate_mpesa" name:"grant_activate_mpesa" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`

	PostCheques         bool `json:"post_cheques" name:"post_cheques" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	ApproveCheques      bool `json:"approve_cheques" name:"approve_cheques" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantPostCheques    bool `json:"grant_post_cheques" name:"grant_post_cheques" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantApproveCheques bool `json:"grant_approve_cheques" name:"grant_approve_cheques" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`

	CreateUsers      bool `json:"create_users" name:"create_users" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	DeleteUsers      bool `json:"delete_users" name:"delete_users" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantCreateUsers bool `json:"grant_create_users" name:"grant_create_users" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantDeleteUsers bool `json:"grant_delete_users" name:"grant_delete_users" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`

	PriceChange        bool `json:"price_change" name:"price_change" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantPriceChange   bool `json:"grant_price_change" name:"grant_price_change" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	AmmendInvoice      bool `json:"ammend_invoice" name:"ammend_invoice" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantAmmendInvoice bool `json:"grant_ammend_invoice" name:"grant_ammend_invoice" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`

	CreateStock       bool `json:"create_stock" name:"create_stock" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantCreateStock  bool `json:"grant_create_stock" name:"grant_create_stock" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	LinkStock         bool `json:"link_stock" name:"link_stock" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	CompleteStockTake bool `json:"complete_stock_take" name:"complete_stock_take" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`

	Accounts            bool `json:"accounts" name:"accounts" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantAccounts       bool `json:"grant_accounts" name:"grant_accounts" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantAproveAccounts bool `json:"grant_aprove_accounts" name:"grant_aprove_accounts" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	AproveAccounts      bool `json:"aprove_accounts" name:"aprove_accounts" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`

	Ledger           bool `json:"ledger" name:"ledger" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	ReconLedger      bool `json:"recon_ledger" name:"recon_ledger" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantLedger      bool `json:"grant_ledger" name:"grant_ledger" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantReconLedger bool `json:"grant_recon_ledger" name:"grant_recon_ledger" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`

	CreateScard      bool `json:"create_scard" name:"create_scard" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantCreateScard bool `json:"grant_create_scard" name:"grant_create_scard" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`

	Produce      bool `json:"produce" name:"produce" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	GrantProduce bool `json:"grant_produce" name:"grant_produce" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`

	TillOpened time.Time `json:"till_opened" name:"till_opened" type:"field" sql:"TIMESTAMP "`
	Till       bool      `json:"till" name:"till" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`
	TillNum    string    `json:"till_num" name:"till_num" type:"field" sql:"VARCHAR(50) "`
	Device     string    `json:"device" name:"device" type:"field" sql:"VARCHAR(50)"`

	Token     string    `json:"token" name:"token" type:"field" sql:"VARCHAR(150)"`
	TokenDate time.Time `json:"token_date" name:"token_date" type:"field" sql:"TIMESTAMP"`
	Reset     bool      `json:"reset" name:"reset" type:"field" sql:"BOOL NOT NULL DEFAULT 'FALSE'"`

	Passcode   string   `json:"passcode"`
	SessionIDs []string `name:"session_ids" `
}

var mySigningKey = os.Getenv("speedsales_key")

// getSessionID fetches sessionID
func getSessionID(username string) ([]string, error) {
	sql := `SELECT 
				session_id::varchar
			FROM users 
			WHERE username = $1`

	rows, err := database.PgPool.Query(context.Background(), sql, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessionIDstr := ""
	for rows.Next() {
		rows.Scan(&sessionIDstr)
	}

	var sessionID []string
	json.Unmarshal([]byte(sessionIDstr), &sessionID)
	return sessionID, nil
}

// ValidateJWT validates whether token is valid
func ValidateJWT(tokenStr string) (Users, bool) {
	var user Users
	token, _ := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", "HMAC")
		}

		// hmacSampleSecret is a []byte containing your secret, e.g. []byte("my_secret_key")
		return mySigningKey, nil
	})

	// check if token is valid and get claims
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		var err error

		// get today's date to compare with token's expiry
		today := time.Now()

		// get expiry date to compare
		expiry, _ := time.Parse("2006-01-02 15:04", fmt.Sprintf("%v", claims["exp"]))

		// check if token is expired
		if today.After(expiry) {
			return user, false
		}

		username := fmt.Sprintf("%v", claims["username"])

		var sessionIDs []string
		if variables.Cache {
			// get users sessionID from redis
			sessionIDstr, err := variables.RdbCon.Get(username).Result()
			if err == redis.Nil {
				return user, false
			} else if err != nil {
				return user, false
			}

			json.Unmarshal([]byte(sessionIDstr), &sessionIDs)
		} else {
			sessionIDs, err = getSessionID(username)
			if err != nil {
				return user, false
			}
		}

		// check if session key exists for user
		for _, sessionID := range sessionIDs {
			if fmt.Sprintf("%v", claims["session"]) != sessionID {
				log.Printf("\tsession id: %v is not same as \n\t claims id: %v\n\n", sessionID, claims["session"])
				continue
			}

			// convert map to json
			jsonStr, _ := json.Marshal(claims["rights"])

			// convert json to struct
			json.Unmarshal(jsonStr, &user)

			return user, true
		}
	}
	return user, false
}
