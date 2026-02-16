package variables

import (
	"github.com/JohnnyKahiu/speedsales/poserver/database"
	"github.com/go-redis/redis"
)

// Fpath stores path to files
var Fpath string

// ProductionDisp tells whether to allow kitchen to dispatch it's own orders
var ProductionDisp bool

// Cache variable defines whether to cache operations data or not
var Cache bool

// RdbCon holds database connection string
var RdbCon *redis.Client

var ServerID int64

// PosSettings structure holds pos settings
type PosSettings struct {
	MinRedeem          int32   `json:"min_redeem"`
	Points             int32   `json:"points"`
	PriceTag           bool    `json:"price_tag"`
	RedPerc            float64 `json:"red_perc"`
	Redeem             bool    `json:"redeem"`
	Rollup             float64 `json:"rollup"`
	AllowProductSearch bool    `json:"allow_product_search"`
	ProductionDispatch bool    `json:"production_dispatch"`
	AllowNegSale       bool    `json:"allow_negative_sale"`
	ApproveSales       bool    `json:"approve_sales"`
	AllowSuspend       bool    `json:"allow_suspend"`
	HasCategories      bool    `json:"has_categories"`
	AuthValidity       int     `json:"auth_validity"`
	MpesaExpiry        int     `json:"mpesa_expiry"`
	ManualAddMpesa     bool    `json:"manual_add_mpesa"`
}

// DocHead holds company's information for printed documents
type DocHead struct {
	CompanyName string `json:"company_name"`
	CompanyPin  string `json:"company_pin"`
	Telephone   string `json:"telephone"`
	Email       string `json:"email"`
	Box         string `json:"box"`
	Location    string `json:"location"`
}

// SysSettings  structure holds system settings
type SysSettings struct {
	PosDefaults PosSettings            `json:"pos_defaults"`
	DocHead     DocHead                `json:"doc_heading"`
	VatCodes    map[string]float32     `json:"vat_codes"`
	SysSettings map[string]interface{} `json:"sys_settings"`
}

type Settings struct {
	table       string                 `name:"settings" type:"table"`
	PosDefaults PosSettings            `json:"pos_defaults" type:"field" sql:"JSONB NOT NULL DEFAULT '{}'"`
	DocHead     DocHead                `json:"doc_heading" type:"field" sql:"JSONB NOT NULL DEFAULT '{}'"`
	VatCodes    map[string]float32     `json:"vat_codes" type:"field" sql:"JSONB NOT NULL DEFAULT '{}'"`
	SysSettings map[string]interface{} `json:"sys_settings" type:"field" sql:"JSONB NOT NULL DEFAULT '{}'"`
}

func GenSettingsTbl() error {
	var tblStruct Settings
	return database.CreateFromStruct(tblStruct)
}
