package sales

import "log"

// // DBPool interface for database operations to allow mocking
// type DBPool interface {
// 	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
// 	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
// 	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
// 	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
// }

// var db DBPool = database.PgPool

func GenTables() error {
	err := genTillTbl()
	if err != nil {
		log.Fatalln("failed to generate till table err =", err)
	}
	err = genSalesTbl()
	if err != nil {
		log.Fatalln("failed to generate sales table err =", err)
	}
	err = genReceiptTbl()
	if err != nil {
		log.Fatalln("failed to generate receipts table err =", err)
	}
	err = genOrderTable()
	if err != nil {
		log.Fatalln("failed to generate order table err =", err)
	}
	return err
}
