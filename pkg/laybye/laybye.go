package laybye

import (
	"context"
	"fmt"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
	"github.com/jackc/pgx/v5"
)

// DBPool is the minimal database interface required by this package.
type DBPool interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type Laybye struct {
	table     string    `name:"laybyes" type:"table"`
	LaybyeID  int64     `json:"laybye_id"  type:"field" sql:"BIGSERIAL NOT NULL"`
	IDNumber  string    `json:"id_number"  type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	Telephone string    `json:"telephone"  type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	Name      string    `json:"name"       type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	Email     string    `json:"email"      type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	Location  string    `json:"location"   type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	State     string    `json:"state"      type:"field" sql:"VARCHAR NOT NULL DEFAULT 'initiated'"`
	Poster    string    `json:"poster"     type:"field" sql:"VARCHAR NOT NULL DEFAULT ''"`
	CreatedAt time.Time `json:"created_at" type:"field" sql:"TIMESTAMPTZ NOT NULL DEFAULT now()"`
	pKey      string    `type:"constraint" name:"laybye_pk" sql:"PRIMARY KEY (laybye_id)"`
}

func GenTable() error {
	return database.CreateFromStruct(Laybye{})
}

// Register inserts a new laybye record and populates LaybyeID and CreatedAt.
// Returns an error if it fails
func (arg *Laybye) Register(ctxt context.Context, tx pgx.Tx) error {
	if arg.Name == "" {
		return fmt.Errorf("customer name is required")
	}
	if arg.Telephone == "" {
		return fmt.Errorf("telephone is required")
	}

	ctx, cancel := context.WithTimeout(ctxt, 15*time.Second)
	defer cancel()

	sql := `INSERT INTO laybyes(id_number, telephone, name, email, location, poster)
			VALUES($1, $2, $3, $4, $5, $6)
			RETURNING laybye_id, created_at`

	if err := tx.QueryRow(ctx, sql, arg.IDNumber, arg.Telephone, arg.Name, arg.Email, arg.Location, arg.Poster).Scan(&arg.LaybyeID, &arg.CreatedAt); err != nil {
		return fmt.Errorf("failed to register laybye: %w", err)
	}

	return tx.Commit(ctx)
}
