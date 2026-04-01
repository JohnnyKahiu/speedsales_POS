package variables

import (
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
)

type Branch struct {
	table           string    `name:"branches" type:"table"`
	BranchID        int64     `json:"branch_id" type:"field" sql:"SERIAL NOT NULL "`
	BranchName      string    `json:"branch_name" type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	BranchCode      string    `json:"branch_code" type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	BranchAddress   string    `json:"branch_address" type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	BranchPhone     string    `json:"branch_phone" type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	BranchEmail     string    `json:"branch_email" type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	BranchLogo      string    `json:"branch_logo" type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	BranchStatus    string    `json:"branch_status" type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	BranchCreatedAt time.Time `json:"branch_created_at" type:"field" sql:"TIMESTAMPTZ NOT NULL DEFAULT now()"`
	BranchUpdatedAt time.Time `json:"branch_updated_at" type:"field" sql:"TIMESTAMPTZ NOT NULL DEFAULT now()"`
	pkey            string    `name:"branch_pkey" type:"constraint" sql:"PRIMARY KEY (branch_id)"`
}

func GenBranchTable() error {
	var b Branch
	return database.CreateFromStruct(b)
}
