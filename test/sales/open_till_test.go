package sales_test

import (
	"testing"

	"github.com/JohnnyKahiu/speedsales/poserver/pkg/sales"
	"github.com/pashagolub/pgxmock/v4"
)

func TestOpenTill(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close()

	// data
	arg := sales.Till{
		OpenFloat:  5000,
		Teller:     "JTELLER",
		Supervisor: "Admin",
		Branch:     "Main",
	}

	// expectation
	mock.ExpectQuery(`INSERT INTO sales_till(open_float, teller, supervisor, branch) VALUES ($1, $2, $3, $4) RETURNING till_no`).
		WithArgs(arg.OpenFloat, arg.Teller, arg.Supervisor, arg.Branch).
		WillReturnRows(mock.NewRows([]string{"till_no"}).AddRow(int64(1)))

	// execution
	err = arg.OpenTill(mock)

	// validation
	if err != nil {
		t.Errorf("error was not expected while open till: %s", err)
	}

	if arg.TillNO != 1 {
		t.Errorf("expected till number 1, got %d", arg.TillNO)
	}

	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
