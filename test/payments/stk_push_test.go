package payments_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JohnnyKahiu/speedsales/poserver/pkg/payment"
	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
)

var errDuplicate = errors.New("duplicate key value violates unique constraint")

func TestAddPending_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	arg := payment.MobileMoney{
		Telephone:   "254712345678",
		Amount:      500,
		RequestMode: "stk push",
	}

	// UUID is generated inside AddPending, so match any value for id and id.String()
	mock.ExpectExec(`INSERT INTO mobile_money`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), arg.Telephone, arg.Amount, arg.RequestMode).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := arg.AddPending(context.Background(), mock); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if arg.ID == uuid.Nil {
		t.Error("expected arg.ID to be set after AddPending, got nil UUID")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestAddPending_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	arg := payment.MobileMoney{
		Telephone:   "254712345678",
		Amount:      500,
		RequestMode: "stk push",
	}

	mock.ExpectExec(`INSERT INTO mobile_money`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), arg.Telephone, arg.Amount, arg.RequestMode).
		WillReturnError(errDuplicate)

	if err := arg.AddPending(context.Background(), mock); err == nil {
		t.Error("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestAddPending_PhoneNormalisation(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mock.Close()

	tests := []struct {
		name  string
		phone string
	}{
		{"international format", "254712345678"},
		{"local format stored as-is", "0712345678"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			arg := payment.MobileMoney{
				Telephone:   tc.phone,
				Amount:      100,
				RequestMode: "stk push",
			}

			mock.ExpectExec(`INSERT INTO mobile_money`).
				WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), tc.phone, arg.Amount, arg.RequestMode).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))

			if err := arg.AddPending(context.Background(), mock); err != nil {
				t.Errorf("unexpected error for %s: %v", tc.phone, err)
			}

			if arg.ID == uuid.Nil {
				t.Errorf("expected arg.ID to be set for %s", tc.phone)
			}
		})
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
