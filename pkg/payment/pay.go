package payment

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/sales"
	"github.com/jackc/pgx/v5"
)

func (arg *Payment) finalizeTransaction(ctx context.Context, tx pgx.Tx) error {
	// Mark orders as claimed
	if err := arg.ClaimOrders(ctx, tx); err != nil {
		return err
	}

	// Perform Receipt Analysis
	rcptNum, _ := strconv.ParseInt(arg.Receipt, 10, 64)
	rcpt := sales.ReceiptLog{ReceiptNum: rcptNum}
	if err := rcpt.Analyze(); err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}
	arg.Analysis = rcpt.Analysis

	// Finalize the physical receipt record
	if err := arg.ClaimReceipt(ctx, tx); err != nil {
		return err
	}

	return nil
}

// CommitPay hadles logic for sale payment
// receives a context
// validates all payments and does a database commit
// returns an error if it fails
func (arg *Payment) CommitPay(ctxt context.Context) error {
	if arg.Receipt == "" {
		log.Println("null receipt")
		return fmt.Errorf("null receipt")
	}
	fmt.Println("]\t receipt =", arg.Receipt)
	ctx, cancel := context.WithTimeout(ctxt, 30*time.Second)
	defer cancel()

	tx, err := database.PgPool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		log.Println("database error,   failed to begin transaction    err =", err)
		return err
	}
	defer tx.Rollback(ctx)

	err = arg.FetchReceipt(ctx, tx)
	if err != nil {
		return err
	}

	err = arg.processAllPayments(ctx, tx)
	if err != nil {
		return err
	}

	err = arg.validateCash()
	if err != nil {
		return err
	}

	fmt.Printf("\t cash tendered = %v\n", arg.CashTendered)
	fmt.Printf("\t      tendered = %v\n", arg.Tendered)
	err = arg.finalizeTransaction(ctx, tx)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)

}
