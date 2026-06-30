package sales

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/pkg/logins"
)

type Approver struct {
	Approver       string
	Token          string
	ApproverRights string
}

func (arg *Approver) Validate(ctx context.Context) error {
	authDetails := logins.Users{Username: arg.Approver}
	// fetch authorizer's details
	err := authDetails.FetchUser(ctx)
	if err != nil {
		return fmt.Errorf("cannot fetch user")
	}

	poSett, _ := FetchSettings()
	// return approved when ApproveSales settings not set as true
	if !poSett.ApproveSales {
		return nil
	}

	if arg.ApproverRights == "cash" {
		if !authDetails.CashRollups {
			return errors.New("forbidden")
		}
	}

	if arg.ApproverRights == "sales" {
		if !authDetails.ApproveSales {
			return errors.New("forbidden")
		}
	}

	today := time.Now()
	if today.After(authDetails.TokenDate) {
		return errors.New("expired")
	}

	if arg.Token != authDetails.Token {
		return errors.New("invalid")
	}

	return nil
}
