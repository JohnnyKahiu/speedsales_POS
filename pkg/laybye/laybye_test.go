package laybye_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/pkg/laybye"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

// registerArgs is the number of $N params in the INSERT statement:
// id_number, telephone, name, email, location, poster
const registerArgs = 6

// anyArgs returns n AnyArg matchers.
// pgxmock checks len(expected) == len(actual) before comparing values,
// so WithArgs must always supply the correct count or the expectation is
// silently skipped and later reported as unfulfilled.
func anyArgs(n int) []interface{} {
	args := make([]interface{}, n)
	for i := range args {
		args[i] = pgxmock.AnyArg()
	}
	return args
}

func TestRegisterLaybye(t *testing.T) {
	createdAt := time.Now()

	tests := []struct {
		name    string
		lay     laybye.Laybye
		setup   func(m pgxmock.PgxPoolIface) pgx.Tx
		wantErr bool
		wantID  int64
	}{
		{
			name: "success",
			lay: laybye.Laybye{
				IDNumber:  "12345678",
				Telephone: "0712345678",
				Name:      "Jane Doe",
				Email:     "jane@example.com",
				Location:  "Nairobi",
				Poster:    "cashier1",
			},
			setup: func(m pgxmock.PgxPoolIface) pgx.Tx {
				m.ExpectBegin()
				m.ExpectQuery(`INSERT INTO laybyes`).
					WithArgs("12345678", "0712345678", "Jane Doe", "jane@example.com", "Nairobi", "cashier1").
					WillReturnRows(m.NewRows([]string{"laybye_id", "created_at"}).AddRow(int64(101), createdAt))
				m.ExpectCommit()
				tx, _ := m.BeginTx(context.Background(), pgx.TxOptions{})
				return tx
			},
			wantErr: false,
			wantID:  101,
		},
		{
			name: "db error",
			lay: laybye.Laybye{
				IDNumber:  "12345678",
				Telephone: "0712345678",
				Name:      "Jane Doe",
			},
			setup: func(m pgxmock.PgxPoolIface) pgx.Tx {
				m.ExpectBegin()
				m.ExpectQuery(`INSERT INTO laybyes`).
					WithArgs(anyArgs(registerArgs)...).
					WillReturnError(fmt.Errorf("connection reset"))
				tx, _ := m.BeginTx(context.Background(), pgx.TxOptions{})
				return tx
			},
			wantErr: true,
		},
		{
			name: "no rows returned",
			lay: laybye.Laybye{
				IDNumber:  "12345678",
				Telephone: "0712345678",
				Name:      "Jane Doe",
			},
			setup: func(m pgxmock.PgxPoolIface) pgx.Tx {
				m.ExpectBegin()
				m.ExpectQuery(`INSERT INTO laybyes`).
					WithArgs(anyArgs(registerArgs)...).
					WillReturnRows(m.NewRows([]string{"laybye_id", "created_at"}))
				tx, _ := m.BeginTx(context.Background(), pgx.TxOptions{})
				return tx
			},
			wantErr: true,
		},
		{
			name:    "missing name",
			lay:     laybye.Laybye{Telephone: "0712345678"},
			setup:   func(m pgxmock.PgxPoolIface) pgx.Tx { return nil },
			wantErr: true,
		},
		{
			name:    "missing telephone",
			lay:     laybye.Laybye{Name: "Jane Doe"},
			setup:   func(m pgxmock.PgxPoolIface) pgx.Tx { return nil },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatalf("pgxmock.NewPool: %s", err)
			}
			defer mock.Close()

			tx := tt.setup(mock)

			err = tt.lay.Register(context.Background(), tx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.lay.LaybyeID != tt.wantID {
				t.Errorf("LaybyeID = %d, want %d", tt.lay.LaybyeID, tt.wantID)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled mock expectations: %s", err)
			}
		})
	}
}
