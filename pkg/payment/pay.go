package payment

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	pb "github.com/JohnnyKahiu/speed_sales_proto/etims"
	"github.com/JohnnyKahiu/speedsales/poserver/database"
	posgrpc "github.com/JohnnyKahiu/speedsales/poserver/pkg/grpc"
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
	if err := rcpt.Analyze(ctx); err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}
	arg.Analysis = rcpt.Analysis

	// Sign the sale with eTIMS — non-fatal: log the error but do not abort
	if err := arg.signEtims(ctx); err != nil {
		log.Printf("etims sign failed for receipt %s: %v", arg.Receipt, err)
	}

	// Finalize the physical receipt record
	if err := arg.ClaimReceipt(ctx, tx); err != nil {
		return err
	}

	return nil
}

// signEtims builds an InvoiceRequest from the Payment, calls the ETIMS gRPC
// service, and stores the returned receipt signature on arg.Etr.
func (arg *Payment) signEtims(ctx context.Context) error {
	svc, err := posgrpc.GetEtimsService()
	if err != nil {
		return fmt.Errorf("etims client unavailable: %w", err)
	}

	req := &pb.InvoiceRequest{
		InvoiceId:  arg.Receipt,
		SaleDate:   arg.TransDate.Format("20060102150405"),
		StockRlsDt: arg.TransDate.Format("20060102150405"),
		CustTin:    arg.CustomerPin,
		CustNm:     arg.AcName,
		RcptTyCd:   "S",
		PmtTyCd:    arg.pmtTyCd(),
		SaleStCd:   "02",
		TotAmt:     arg.Total,
	}

	for i, s := range arg.Items {
		taxbl := s.Total - s.Vat
		req.Items = append(req.Items, &pb.LineItem{
			ItemSeq:  int32(i + 1),
			ItemCd:   s.ItemCode,
			ItemNm:   s.ItemName,
			Qty:      s.Quantity,
			Prc:      s.Price,
			SplyAmt:  s.Quantity*s.Price + s.Discount,
			DcAmt:    s.Discount,
			TaxblAmt: taxbl,
			TaxAmt:   s.Vat,
			TotAmt:   s.Total,
			TaxTyCd:  s.VatAlpha,
		})
		req.TaxblAmtA += taxbl
		req.TaxAmt += s.Vat
	}

	resp, err := svc.SubmitInvoice(ctx, req)
	if err != nil {
		return fmt.Errorf("SubmitInvoice: %w", err)
	}
	if resp.Status == "error" {
		return fmt.Errorf("etims error: %s", resp.Message)
	}

	arg.Etr.Seal = resp.RcptSign
	arg.Etr.CUSN = resp.CuSn
	arg.Etr.TSIN = resp.InvoiceId
	arg.Etr.DATE = arg.TransDate.Format("02/01/2006 15:04:05")
	return nil
}

// pmtTyCd returns the KRA payment type code based on tendered amounts.
func (arg *Payment) pmtTyCd() string {
	hasCash := arg.CashTendered > 0
	hasMpesa := arg.MpesaTendered > 0
	hasCard := arg.EcardTendered > 0

	switch {
	case hasMpesa && !hasCash && !hasCard:
		return "04" // mobile money
	case hasCard && !hasCash && !hasMpesa:
		return "03" // card
	default:
		return "01" // cash / mixed
	}
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

	if err := arg.FetchReceipt(ctx, tx); err != nil {
		return err
	}

	if err := arg.processAllPayments(ctx, tx); err != nil {
		return err
	}

	if err := arg.validateCash(); err != nil {
		return err
	}

	fmt.Printf("\t cash tendered = %v\n", arg.CashTendered)
	fmt.Printf("\t      tendered = %v\n", arg.Tendered)
	if err = arg.finalizeTransaction(ctx, tx); err != nil {
		return err
	}

	if err := arg.publishPaymentEvent(ctx); err != nil {
		return err
	}

	// tx.Rollback(ctx)/

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

// publishPaymentEvent records and publishes the cash-sale payment as a payments_in
// event. Failures are logged, not returned: the sale itself is already committed.
func (arg *Payment) publishPaymentEvent(ctx context.Context) error {
	receiptNum, _ := strconv.ParseInt(arg.Receipt, 10, 64)
	tillNum, _ := strconv.ParseInt(arg.TillNum, 10, 64)

	tp := sales.TillPayment{
		PaymentFor: "cash_sale",
		TillNum:    tillNum,
		SaleID:     receiptNum,
		Cash:       arg.CashTendered - arg.Change,
		Mpesa:      arg.MpesaTendered,
		Ecard:      arg.EcardTendered,
		Cheque:     arg.CheckTendered,
	}
	if err := tp.RecordAndPublish(ctx); err != nil {
		log.Println("error. failed to record till_payment for cash_sale    err =", err)
		return err
	}
	return nil
}
