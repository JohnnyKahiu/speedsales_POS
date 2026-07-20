package payment

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

func (arg *Payment) processAllPayments(ctx context.Context, tx pgx.Tx) error {
	// Claim Mpesa
	if len(arg.MpesaDetails) > 0 {
		if err := arg.ClaimMpesa(ctx, tx); err != nil {
			log.Printf("mpesa claim failed: %v", err)
			return fmt.Errorf("mpesa claim failed")
		}
	}

	// Claim E-Cards
	if len(arg.EcardDetails) > 0 {
		if err := arg.ClaimEcards(ctx, tx); err != nil {
			log.Printf("ecard claim failed: %w", err)
			return fmt.Errorf("ecard claim failed")
		}
	}

	// Claim Gift Vouchers
	if len(arg.GVoucherDetails) > 0 {
		total, err := arg.ClaimGiftVoucher(ctx, tx)
		if err != nil {
			log.Printf("voucher claim failed: %w", err)
			return fmt.Errorf("voucher claim failed")
		}
		arg.VoucherTotal = total
	}
	return nil
}

// claim all mpesa payments
func (arg *Payment) ClaimMpesa(ctx context.Context, tx pgx.Tx) error {
	var total float64
	sql := `UPDATE mobile_money SET trace_num = $1, claimed = True WHERE code = $2 RETURNING amount`
	for _, detail := range arg.MpesaDetails {
		var amount float64
		if err := tx.QueryRow(ctx, sql, arg.Receipt, detail.MpesaCode).Scan(&amount); err != nil {
			log.Println("error. failed to claim mobile_money     err =", err)
			return err
		}

		total += amount
	}

	arg.MpesaTendered = total
	return nil
}

// claim all ecard payments
func (arg *Payment) ClaimEcards(ctx context.Context, tx pgx.Tx) error {
	fmt.Println("\t\t\tstarted claim ecards")
	start := time.Now()
	defer fmt.Printf("\t\t\tClaimEcards took %v\n", time.Since(start))

	var total float64
	sql := `UPDATE card_transaction SET sales_receipt = $1 WHERE auto_id = $2 RETURNING amount`
	for _, detail := range arg.EcardDetails {
		fmt.Println("\t\t\t\t ecards detail =", detail)
		var ecard float64
		err := tx.QueryRow(ctx, sql, arg.Receipt, detail.TxnID).Scan(&ecard)
		if err != nil {
			log.Println("error. failed to claim ecards     err =", err)
			return err
		}

		total += ecard
	}
	arg.EcardTendered = total

	return nil
}

// claim all gift vouchers
func (arg *Payment) ClaimGiftVoucher(ctx context.Context, tx pgx.Tx) (float64, error) {
	sql := `UPDATE gift_voucher 
			SET 
				txn_receipt = $1
				, teller = $2
				, claimer_name = $3
				, claimer_tel = $4
				, claimer_id = $5
				, approvers = $6
			WHERE serial = $7
			`

	total := float64(0)
	for _, detail := range arg.GVoucherDetails {
		vcher := float64(0)
		err := tx.QueryRow(ctx, sql, arg.Receipt, arg.Teller, detail.ClaimerName, detail.ClaimerTel, detail.ClaimerID, arg.Approver, detail.SerialNum).Scan(&vcher)
		if err != nil {
			log.Println("error. failed to claim gift_voucher     err =", err)
			return total, err
		}

		total += vcher
	}
	return total, nil
}
