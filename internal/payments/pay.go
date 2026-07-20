package payments

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	pb "github.com/JohnnyKahiu/speed_sales_proto/pay_gateway"
	"github.com/JohnnyKahiu/speedsales/poserver/database"
	posgrpc "github.com/JohnnyKahiu/speedsales/poserver/pkg/grpc"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/logins"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/payment"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/sales"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func POST(w http.ResponseWriter, r *http.Request) map[string]interface{} {
	respMap := make(map[string]interface{})

	userStr := r.Header.Get("user_details")
	if userStr == "" {
		respMap["response"] = "error"
		respMap["message"] = "user details not found"
		return respMap
	}

	details := logins.Users{}
	json.Unmarshal([]byte(userStr), &details)

	vars := mux.Vars(r)
	m := vars["module"]

	if !details.AcceptPayment {
		respMap["response"] = "forbidden"
		respMap["message"] = "forbidden"
		return respMap
	}

	switch m {
	case "commit":
		b, err := io.ReadAll(r.Body)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "params error"
			respMap["trace"] = err
			return respMap
		}

		pay := payment.Payment{TillNum: fmt.Sprintf("%v", details.TillNum)}
		err = json.Unmarshal(b, &pay)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "failed to encode data"
			respMap["trace"] = err
			return respMap
		}

		err = pay.CommitPay(r.Context())
		if err != nil {
			log.Println("commit Pay error     err =", err)
			respMap["response"] = "error"
			respMap["message"] = err
			respMap["trace"] = err
			return respMap
		}

		respMap["response"] = "success"
		respMap["tendered"] = pay.Tendered
		respMap["change"] = pay.Change
		respMap["values"] = pay
		return respMap

	case "stk_push":
		b, err := io.ReadAll(r.Body)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "bad request"
			return respMap
		}

		var req struct {
			Phone            string  `json:"phone"`
			Amount           float64 `json:"amount"`
			AccountReference string  `json:"account_reference"`
			Description      string  `json:"description"`
		}
		if err = json.Unmarshal(b, &req); err != nil || req.Phone == "" || req.Amount <= 0 {
			respMap["response"] = "error"
			respMap["message"] = "phone, amount and account_reference are required"
			return respMap
		}

		if strings.HasPrefix(req.Phone, "0") {
			req.Phone = "254" + req.Phone[1:]
		}

		// fetch mpesa_account from POS settings
		posSett, err := sales.FetchSettings()
		if err != nil || posSett.MpesaAccount == "" {
			respMap["response"] = "error"
			respMap["message"] = "mpesa_account not configured in POS settings"
			return respMap
		}

		mm := payment.MobileMoney{
			Telephone:   req.Phone,
			Amount:      req.Amount,
			RequestMode: "stk push",
		}
		if err := mm.AddPending(r.Context(), database.PgPool); err != nil {
			log.Println("stk_push: mobile_money pending insert error:", err)
			respMap["response"] = "error"
			respMap["message"] = "failed to initialise payment record"
			return respMap
		}

		gw, err := posgrpc.GetPayGatewayService()
		if err != nil {
			log.Println("stk_push: paygateway dial error:", err)
			respMap["response"] = "error"
			respMap["message"] = "failed to connect to payment gateway"
			return respMap
		}

		ref := req.AccountReference
		if ref == "" {
			ref = fmt.Sprintf("%v", details.TillNum)
		}
		desc := req.Description
		if desc == "" {
			desc = "Payment"
		}

		resp, err := gw.STKPush(&pb.STKPushRequest{
			AccountID:        posSett.MpesaAccount,
			Phone:            req.Phone,
			Amount:           req.Amount,
			AccountReference: ref,
			Description:      desc,
			PosID:            mm.ID.String(),
		})
		if err != nil {
			log.Println("stk_push error:", err)
			respMap["response"] = "error"
			respMap["message"] = "STK push failed: " + err.Error()
			return respMap
		}
		if resp.Error != "" {
			respMap["response"] = "error"
			respMap["message"] = resp.Error
			return respMap
		}

		respMap["response"] = "success"
		respMap["pos_id"] = mm.ID.String()
		respMap["transaction_id"] = resp.TransactionID
		respMap["checkout_request_id"] = resp.CheckoutRequestID
		respMap["customer_message"] = resp.CustomerMessage
		return respMap

	case "manual-mpesa":
		b, err := io.ReadAll(r.Body)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "bad request"
			return respMap
		}

		var req struct {
			Code      string  `json:"code"`
			Amount    float64 `json:"amount"`
			Telephone string  `json:"telephone"`
		}
		if err = json.Unmarshal(b, &req); err != nil || req.Amount <= 0 {
			respMap["response"] = "error"
			respMap["message"] = "amount is required"
			return respMap
		}

		mm := payment.MobileMoney{
			Code:      req.Code,
			Telephone: req.Telephone,
			Amount:    req.Amount,
		}
		// mobile_money.code is UNIQUE — a blank/repeated cashier-typed code
		// (the frontend doesn't require one) would collide, so mint a unique
		// placeholder the same way AddPending does when the real code isn't
		// known yet.
		if mm.Code == "" {
			mm.Code = uuid.New().String()
		}

		if err := mm.AddManual(r.Context(), database.PgPool); err != nil {
			respMap["response"] = "error"
			if strings.Contains(err.Error(), "duplicate key") {
				respMap["message"] = "that mpesa code has already been recorded"
			} else {
				respMap["message"] = "failed to record manual mpesa entry"
			}
			return respMap
		}

		respMap["response"] = "success"
		respMap["pos_id"] = mm.ID.String()
		respMap["code"] = mm.Code
		return respMap
	}

	return respMap
}

func GET(w http.ResponseWriter, r *http.Request) map[string]interface{} {
	respMap := make(map[string]interface{})

	userStr := r.Header.Get("user_details")
	if userStr == "" {
		respMap["response"] = "error"
		respMap["message"] = "user details not found"
		return respMap
	}

	details := logins.Users{}
	json.Unmarshal([]byte(userStr), &details)

	vars := mux.Vars(r)
	m := vars["module"]

	switch m {
	case "receipts":
		if !details.AcceptPayment {
			respMap["response"] = "forbidden"
			respMap["message"] = "forbidden"
			return respMap
		}

		rcpt := sales.ReceiptLog{Branch: details.Branch}
		vals, err := rcpt.GetPayingRcpts(r.Context())
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "failed fetching paying receipts"
			respMap["trace"] = err
			return respMap
		}

		respMap["response"] = "success"
		respMap["values"] = vals
		return respMap

	case "active-mpesa":
		if !details.AcceptPayment {
			respMap["response"] = "forbidden"
			respMap["message"] = "forbidden"
			return respMap
		}

		vals, err := payment.FetchActiveMpesa(r.Context())
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "error fetching"
			return respMap
		}

		respMap["response"] = "success"
		respMap["values"] = vals
		return respMap

	case "mpesa-status":
		id, err := uuid.Parse(r.URL.Query().Get("id"))
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "uuid parse error"
			respMap["trace"] = err
			return respMap
		}

		mm := payment.MobileMoney{
			ID: id,
		}

		err = mm.Fetch(r.Context())
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "failed to fetch txns"
			respMap["trace"] = err
			return respMap
		}

		respMap["response"] = "success"
		respMap["values"] = mm
		return respMap

	case "check-mpesa":
		code := r.URL.Query().Get("code")
		if code == "" {
			respMap["response"] = "error"
			respMap["message"] = "code is required"
			return respMap
		}

		mm, err := payment.FetchByCode(r.Context(), code)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "transaction not found"
			return respMap
		}

		respMap["response"] = "success"
		respMap["values"] = mm
		return respMap
	}

	return respMap
}
