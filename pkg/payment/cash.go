package payment

import (
	"fmt"
)

func (arg *Payment) validateCash() error {
	softCash := arg.MpesaTendered + arg.VoucherTotal + arg.EcardTendered + arg.PointsRedeemed

	// Rule: No change for non-cash
	if softCash > arg.Total {
		return fmt.Errorf("cannot issue change on non-cash transactions")
	}

	arg.Tendered = softCash + arg.CashTendered
	fmt.Printf("\t cash tendered = %v\n", arg.CashTendered)
	fmt.Printf("\t      tendered = %v\n", arg.Tendered)

	// Rule: Must pay enough
	if arg.Total > arg.Tendered {
		return fmt.Errorf("insufficient amount tendered: need %.2f, got %.2f", arg.Total, arg.Tendered)
	}

	arg.Change = arg.Tendered - arg.Total

	// Rule: Security limit on change
	if arg.Change > 1000 {
		return fmt.Errorf("change amount %.2f exceeds the 1000 limit", arg.Change)
	}

	return nil
}
