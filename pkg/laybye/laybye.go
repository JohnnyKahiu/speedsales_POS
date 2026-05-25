package laybye

import (
	"context"
	"fmt"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
)

type Laybye struct {
	table     string    `name:"laybyes" type:"table"`
	LaybyeID  int64     `json:"laybye_id"  type:"field" sql:"BIGSERIAL PRIMARY KEY"`
	IDNumber  string    `json:"id_number"  type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	Telephone string    `json:"telephone"  type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	Name      string    `json:"name"       type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	Email     string    `json:"email"      type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	Location  string    `json:"location"   type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	State     string    `json:"state"      type:"field" sql:"VARCHAR NOT NULL DEFAULT 'active'"`
	Poster    string    `json:"poster"     type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	Branch    string    `json:"branch"     type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	CompanyID int64     `json:"company_id" type:"field" sql:"BIGINT NOT NULL DEFAULT '0'"`
	CreatedAt time.Time `json:"created_at" type:"field" sql:"TIMESTAMPTZ NOT NULL DEFAULT now()"`
}

func GenTable() error {
	return database.CreateFromStruct(Laybye{})
}

// Register inserts a new laybye record and populates LaybyeID.
func (arg *Laybye) Register(ctx context.Context) error {
	if arg.Name == "" {
		return fmt.Errorf("customer name is required")
	}
	if arg.Telephone == "" {
		return fmt.Errorf("telephone is required")
	}

	sql := `INSERT INTO laybyes(id_number, telephone, name, email, location, poster, branch, company_id)
			VALUES($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING laybye_id, created_at`

	rows, err := database.PgPool.Query(ctx, sql,
		arg.IDNumber, arg.Telephone, arg.Name, arg.Email,
		arg.Location, arg.Poster, arg.Branch, arg.CompanyID,
	)
	if err != nil {
		return fmt.Errorf("failed to register laybye: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		if err := rows.Scan(&arg.LaybyeID, &arg.CreatedAt); err != nil {
			return fmt.Errorf("failed to scan laybye result: %w", err)
		}
	}

	return nil
}
