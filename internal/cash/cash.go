package cash

import (
	"encoding/json"

	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/logins"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/sales"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/variables"
	"github.com/gorilla/mux"
)

func Get(w http.ResponseWriter, r *http.Request) map[string]interface{} {
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

	case "cart":
		fmt.Println("\t  == timing get cart request ==")
		start := time.Now()

		if !details.MakeSales {
			respMap["response"] = "forbidden"
			respMap["message"] = "forbidden"
			return respMap
		}

		fmt.Println("till num =", details.TillNum)

		rcpt := r.URL.Query().Get("receipt")
		var err error
		fmt.Println("\t receipt =", rcpt)

		var a sales.ReceiptLog
		if rcpt == "" {
			a.Poster = details.Username
			a.Branch = details.ResolveBranch(r.URL.Query().Get("branch"))
			a.CompanyID = details.CompanyID
			a.TillNum = details.TillNum
			a.SaleType = "Cash Sale"

			a.GenReceipt(r.Context())
		} else {
			a.ReceiptNum, _ = strconv.ParseInt(rcpt, 10, 64)
		}
		// log.Fatalln("receipt num =", a.ReceiptNum)

		err = a.Fetch(r.Context())
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "failed fetching sales cart"
			respMap["trace"] = err
			fmt.Println("error\t", err)

			return respMap
		}

		reqRollup := false

		// fetch system defaults
		poSett, _ := sales.FetchSettings()
		if a.Total <= 0 {
			// fetch current cash in till
			// cashInTill, _ := sales.CashInTill(details.TillNum)
			cashInTill := float64(0)

			if poSett.Rollup <= cashInTill {
				reqRollup = true
			}
		}

		respMap["response"] = "success"
		respMap["receipt"] = a.ReceiptNum
		respMap["cart"] = a.Cart
		respMap["total"] = a.Total
		respMap["rollup"] = reqRollup
		respMap["stage"] = a.State
		respMap["settings"] = poSett

		elapsed := time.Since(start)
		fmt.Printf("\nget cart for user %v \t time elapsed = %v\n", details.Username, elapsed)
		return respMap

	case "active-carts":
		fmt.Println("\t fetching all active carts")
		rcpt := sales.ReceiptLog{TillNum: details.TillNum}

		receipts, err := rcpt.GetActiveCarts(r.Context())
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "error getting active carts"
			respMap["trace"] = err

			return respMap
		}

		respMap["response"] = "success"
		respMap["till_num"] = details.TillNum
		respMap["values"] = receipts

		return respMap

	default:
		return respMap
	}

}

func Post(w http.ResponseWriter, r *http.Request) map[string]interface{} {

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

	fmt.Println("cash post module = ", m)

	switch m {
	case "open-till":
		fmt.Printf("\n\t Open till \n\t make_sales = %v \n\t accept_payments = %v \n\t, username = %v \n ", details.MakeSales, details.AcceptPayment, details.Username)

		if !details.MakeSales && !details.AcceptPayment {
			respMap["response"] = "error"
			respMap["message"] = "forbidden"

			return respMap
		}

		b, err := io.ReadAll(r.Body)
		fmt.Println("body =", r.Body)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "bad request"

			return respMap
		}

		var entry map[string]interface{}
		err = json.Unmarshal(b, &entry)

		strVal := func(k string) string {
			if v, ok := entry[k]; ok {
				return fmt.Sprintf("%v", v)
			}
			return ""
		}

		authDetails := logins.Users{Username: strVal("approver")}

		// fetch authorizer's details
		err = authDetails.FetchUser(r.Context())
		fmt.Printf("\nauthorizer details = %v\n", authDetails)
		if err != nil {
			log.Printf("\t error fetching user %v\t error = %v\n\n", entry["approver"], err)

			respMap["response"] = "error"
			respMap["message"] = "failed to get approver"

			return respMap
		}

		// authDetail := authDetails
		// companyID := fmt.Sprintf("%v", details.CompanyID)
		poSett, _ := sales.FetchSettings()

		fmt.Println("Approve sales = ", poSett.ApproveSales)
		log.Println("Cash rollups = ", authDetails.CashRollups)

		if !authDetails.CashRollups && poSett.ApproveSales {
			respMap["response"] = "error"
			respMap["message"] = "approval error \n approver is forbidden from opening till \n ensure you have 'Cash Rollups' rights to continue"

			return respMap
		}
		if authDetails.Token != strVal("ap_token") && poSett.ApproveSales {
			respMap["response"] = "error"
			respMap["message"] = "incorrect user or password \n ensure you have the correct approval token \n or you have selected the right user"

			return respMap
		}
		// get today's date and compare if token is expired
		today := time.Now()

		// check if token is expired
		if today.After(authDetails.TokenDate) && poSett.ApproveSales {
			respMap["response"] = "error"
			respMap["message"] = "approval error \n Token Expired \n Please renew your token to continue"

			return respMap
		}

		openFloat, _ := strconv.ParseFloat(strVal("open_float"), 64)

		till := sales.Till{
			Teller:     details.Username,
			Branch:     details.ResolveBranch(strVal("branch")),
			Supervisor: strVal("approver"),
			OpenFloat:  openFloat,
		}

		// open sales till
		err = till.OpenTill(r.Context(), database.PgPool)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "error\n failed while creating till"
			respMap["trace"] = err

			return respMap
		}

		respMap["response"] = "success"
		respMap["till_num"] = till.TillNO
		// respMap["token"] = newToken

		return respMap

	case "new_receipt":
		fmt.Println("\t new bill")
		if !details.MakeSales {
			respMap["response"] = "error"
			respMap["message"] = "forbidden"
			return respMap
		}

		var branchReq struct {
			Branch string `json:"branch"`
		}
		if b, err := io.ReadAll(r.Body); err == nil {
			json.Unmarshal(b, &branchReq)
		}

		receipt := sales.ReceiptLog{
			TillNum:   details.TillNum,
			Poster:    details.Username,
			SaleType:  "Cash Sale",
			Branch:    details.ResolveBranch(branchReq.Branch),
			CompanyID: details.CompanyID,
			AcNum:     "0",
		}

		err := receipt.GenReceipt(r.Context())
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "receipt number is null"
			respMap["trace"] = err

			return respMap
		}

		// get active carts
		receipts, err := receipt.GetActiveCarts(r.Context())
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "error getting active carts"
			respMap["trace"] = err

			return respMap
		}

		respMap["response"] = "success"
		respMap["carts"] = receipts
		respMap["receipt_num"] = receipt.ReceiptNum
		return respMap

	case "new_bill":
		if !details.MakeSales {
			respMap["response"] = "forbidden"
			respMap["message"] = "forbidden"
			return respMap
		}

		b, _ := io.ReadAll(r.Body)
		billReq := map[string]interface{}{}
		json.Unmarshal(b, &billReq)
		strField := func(k string) string {
			v, _ := billReq[k].(string)
			return v
		}

		custName := strField("customer_name")
		if custName == "" {
			custName = "walk_in"
		}

		receipt := sales.ReceiptLog{
			TillNum:   details.TillNum,
			Branch:    details.ResolveBranch(strField("branch")),
			Poster:    details.Username,
			SaleType:  "Cash Sale",
			CompanyID: 0,
			CustName:  custName,
		}

		// Always create a fresh receipt — skip CheckIfExists so that multiple
		// walk-in tabs can coexist (required for restaurant multi-tab mode).
		if _, err := receipt.CreateReceipt(r.Context()); err != nil {
			log.Println("new_bill CreateReceipt error     err =", err)
			respMap["response"] = "error"
			respMap["message"] = "failed to create new receipt"
			respMap["trace"] = err
			return respMap
		}

		// get active carts
		receipts, err := receipt.GetActiveCarts(r.Context())
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "error getting active carts"
			respMap["trace"] = err

			return respMap
		}

		respMap["response"] = "success"
		respMap["bills"] = receipts
		respMap["receipt_num"] = receipt.ReceiptNum
		return respMap

	case "suspend":
		if !details.MakeSales {
			respMap["response"] = "forbidden"
			respMap["message"] = "forbidden"
			return respMap
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "bad request"
			return respMap
		}

		rcpt := sales.ReceiptLog{}
		err = json.Unmarshal(b, &rcpt)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "failed to unmarshal json"
			return respMap
		}

		err = rcpt.Suspend(r.Context())
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "failed to suspend"
			respMap["trace"] = err
			return respMap
		}

		err = rcpt.GenReceipt(r.Context())
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "failed to gen new receipt"
			respMap["trace"] = err
			return respMap
		}

		respMap["response"] = "success"
		return respMap

	case "receipt":
		if !details.MakeSales {
			respMap["response"] = "forbidden"
			respMap["message"] = "forbidden"
			return respMap
		}

	case "settings":
		b, err := io.ReadAll(r.Body)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "bad request"
			return respMap
		}

		current, err := sales.FetchSettings()
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "failed to load current settings"
			return respMap
		}

		// merge only provided fields
		if err = json.Unmarshal(b, &current); err != nil {
			respMap["response"] = "error"
			respMap["message"] = "bad request"
			return respMap
		}

		if err = variables.UpdatePosSettings(current); err != nil {
			log.Println("settings update error:", err)
			respMap["response"] = "error"
			respMap["message"] = "failed to save settings"
			return respMap
		}

		respMap["response"] = "success"
		respMap["settings"] = current
		return respMap

	case "add-cart":
		if !details.MakeSales {
			respMap["response"] = "forbidden"
			respMap["message"] = "forbidden"
			return respMap
		}

		// get params
		b, err := io.ReadAll(r.Body)
		fmt.Println("body =", r.Body)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "bad request"

			return respMap
		}

		// unmarshal body
		item := sales.Sales{}
		err = json.Unmarshal(b, &item)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "bad request"

			return respMap
		}

		// fetch receiptNum if not provided
		if item.ReceiptNum == 0 {
			var branchReq struct {
				Branch string `json:"branch"`
			}
			json.Unmarshal(b, &branchReq)

			// Branch was never set here at all — any receipt implicitly
			// created through this fallback got branch = '' in salestrace.
			rcpt := sales.ReceiptLog{TillNum: details.TillNum, Poster: details.Username, Branch: details.ResolveBranch(branchReq.Branch)}
			err = rcpt.GenReceipt(r.Context())
			if err != nil {
				log.Println("failed to create receipt")
				respMap["response"] = "error"
				respMap["message"] = "receipt number is null"
				respMap["trace"] = err

				return respMap
			}

			item.ReceiptNum = rcpt.ReceiptNum
		}

		cart, err := item.AddCart(r.Context())
		if err != nil {
			log.Println("error. failed to add item to cart     err =", err)
			respMap["response"] = "error"
			respMap["message"] = "failed adding to cart"
			respMap["trace"] = err

			return respMap
		}

		// w.WriteHeader(http.StatusOK)
		respMap["response"] = "success"
		respMap["cart"] = cart

		return respMap

	case "VOID", "void":
		b, err := io.ReadAll(r.Body)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "bad request"
			return respMap
		}

		var req struct {
			ReceiptNum    int64  `json:"receipt_num"`
			Approver      string `json:"approver"`
			ApprovalToken string `json:"approval_token"`
			Reason        string `json:"reason"`
		}
		if err = json.Unmarshal(b, &req); err != nil || req.ReceiptNum == 0 {
			respMap["response"] = "error"
			respMap["message"] = "receipt_num, approver, approval_token and reason are required"
			return respMap
		}

		// validate approver token via login service
		authDetails := logins.Users{Username: req.Approver}
		if err = authDetails.FetchUser(r.Context()); err != nil {
			log.Printf("void: failed to fetch approver %v err=%v", req.Approver, err)
			respMap["response"] = "error"
			respMap["message"] = "failed to verify approver"
			return respMap
		}
		if !authDetails.ApproveSales {
			respMap["response"] = "error"
			respMap["message"] = "approver does not have sales approval rights"
			return respMap
		}
		if authDetails.Token != req.ApprovalToken {
			respMap["response"] = "error"
			respMap["message"] = "invalid approval token"
			return respMap
		}

		rcpt := sales.ReceiptLog{
			ReceiptNum: req.ReceiptNum,
			Approver:   req.Approver,
			Reason:     req.Reason,
		}
		if err = rcpt.VoidWithReason(r.Context()); err != nil {
			log.Println("void error:", err)
			respMap["response"] = "error"
			respMap["message"] = err.Error()
			return respMap
		}

		respMap["response"] = "success"
		return respMap

	case "close_bill":
		if !details.MakeSales {
			respMap["response"] = "error"
			respMap["message"] = "forbidden"
			return respMap
		}

		// get params
		b, err := io.ReadAll(r.Body)
		fmt.Println("body =", r.Body)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "bad request"

			return respMap
		}

		// unmarshal body
		receipt := sales.ReceiptLog{}
		err = json.Unmarshal(b, &receipt)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "bad request"

			return respMap
		}

		// fetch receiptNum if not provided
		if receipt.ReceiptNum == 0 {
			respMap["response"] = "error"
			respMap["message"] = "receipt number is null"
			return respMap
		}

		err = receipt.CloseBill(r.Context())
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "failed closing bill"
			respMap["trace"] = err

			return respMap
		}

		// populate cust_name from salestrace for the print receipt
		_ = receipt.FetchCustName(r.Context())

		respMap["response"] = "success"
		respMap["sales"] = receipt

		return respMap

	case "merge-bill":
		if !details.MakeSales {
			respMap["response"] = "error"
			respMap["message"] = "forbidden"
			return respMap
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "bad request"
			return respMap
		}

		var params struct {
			Bills   []int64 `json:"bills"`
			Receipt float64 `json:"receipt"`
		}
		err = json.Unmarshal(b, &params)
		if err != nil || params.Receipt == 0 || len(params.Bills) == 0 {
			respMap["response"] = "error"
			respMap["message"] = "bills and receipt are required"
			return respMap
		}

		targetReceipt := int64(params.Receipt)

		// exclude the target from the source list to prevent self-merge
		var sourceBills []int64
		for _, bill := range params.Bills {
			if bill != targetReceipt {
				sourceBills = append(sourceBills, bill)
			}
		}

		if len(sourceBills) == 0 {
			respMap["response"] = "error"
			respMap["message"] = "no source bills to merge"
			return respMap
		}

		receipt := sales.ReceiptLog{}
		receipt.ReceiptNum = targetReceipt

		err = receipt.Merge(r.Context(), sourceBills)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = err.Error()
			return respMap
		}

		respMap["response"] = "success"
		respMap["sales"] = receipt
		return respMap

	case "close-till":
		b, err := io.ReadAll(r.Body)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "params error"
			return respMap
		}

		till := sales.Till{}
		json.Unmarshal(b, &till)

		till.Supervisor = till.CloseSupervisor
		till.TillNO = details.TillNum
		till.Teller = details.Username
		fmt.Printf("till = %s\n", b)

		err = till.CloseTill(r.Context())
		if err != nil {
			log.Println("fatal error. closing till failed")
			respMap["response"] = "error"
			respMap["message"] = fmt.Sprintf("%v", err)
			respMap["trace"] = err
			return respMap
		}

		respMap["response"] = "success"
		respMap["till_num"] = till.TillNO
		// respMap["token"] = newToken

		return respMap
	}
	return respMap
}
