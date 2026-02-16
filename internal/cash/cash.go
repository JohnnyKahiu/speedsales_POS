package cash

import (
	"context"
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
	"github.com/gorilla/mux"
)

func Get(w http.ResponseWriter, r *http.Request) map[string]interface{} {
	// get token from headers
	token := fmt.Sprintf("%v", r.Header.Get("token"))

	respMap := make(map[string]interface{})

	// return authentication error if token is empty
	if token == "" {
		respMap["response"] = "error"
		respMap["message"] = "broken authentication"
		return respMap
	}
	// get token payload if authentic
	details, authentic := logins.ValidateJWT(token)

	if !authentic {
		respMap["response"] = "authentication error"
		respMap["message"] = "authentication error"
		return respMap
	}

	vars := mux.Vars(r)

	m := vars["module"]
	if m == "get-receipt" {
		if !details.MakeSales {
			respMap["response"] = "forbidden"
			respMap["message"] = "forbidden"
			return respMap
		}
		var a sales.ReceiptLog
		a.Poster = details.Username
		a.Branch = details.Branch
		a.CompanyID = details.CompanyID
		a.TillNum, _ = strconv.ParseInt(details.TillNum, 10, 64)
		a.SaleType = "Cash Sale"

		receiptNum, err := a.GenReceipt()
		fmt.Println("\nreceipt number =", receiptNum)

		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "failed to get receipt"
			respMap["trace"] = err
			return respMap
		}

		respMap["response"] = "success"
		respMap["receipt"] = receiptNum
		return respMap
	}
	if m == "cart" {
		fmt.Println("\t  == timing get cart request ==")
		start := time.Now()

		if !details.MakeSales {

			respMap["response"] = "forbidden"
			respMap["message"] = "forbidden"
			return respMap
		}
		fmt.Println("till num =", details.TillNum)

		rcpt := r.URL.Query().Get("receipt")
		var receiptNum int64
		var err error

		var a sales.ReceiptLog
		if rcpt == "" {
			a.Poster = details.Username
			a.Branch = details.Branch
			a.CompanyID = details.CompanyID
			a.TillNum, _ = strconv.ParseInt(details.TillNum, 10, 64)
			a.SaleType = "Cash Sale"

			receiptNum, _ = a.GenReceipt()
		} else {
			a.ReceiptNum, _ = strconv.ParseInt(rcpt, 10, 64)
		}
		fmt.Println("receipt num =", a.ReceiptNum)

		err = a.Fetch()
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
			cashInTill, _ := sales.CashInTill(details.TillNum)

			if poSett.Rollup <= cashInTill {
				reqRollup = true
			}
		}

		respMap["response"] = "success"
		respMap["receipt"] = fmt.Sprintf("%v", receiptNum)
		respMap["values"] = a.Cart
		respMap["total"] = a.Total
		respMap["rollup"] = reqRollup
		respMap["stage"] = a.State
		respMap["settings"] = poSett

		elapsed := time.Since(start)
		fmt.Printf("\nget cart for user %v \t time elapsed = %v\n", details.Username, elapsed)
		return respMap
	}

	return respMap
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

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	m := vars["module"]

	fmt.Println("cash post module = ", m)

	if m == "open-till" {
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

		// Unmarshal into login_info map
		var entry map[string]string
		err = json.Unmarshal(b, &entry)

		authDetails := logins.Users{Username: entry["approver"]}

		// fetch authorizer's details
		err = authDetails.FetchUser(ctx)
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
		if authDetails.Token != entry["ap_token"] && poSett.ApproveSales {
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

		till := sales.Till{
			Teller:     details.Username,
			Branch:     details.Branch,
			Supervisor: entry["approver"],
		}

		// open sales till
		err = till.OpenTill(database.PgPool)
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
	}

	if m == "receipt" {
		if !details.MakeSales {
			respMap["response"] = "error"
			respMap["message"] = "forbidden"
			return respMap
		}
	}

	if m == "add-cart" {
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
		cart := sales.Sales{}
		err = json.Unmarshal(b, &cart)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "bad request"

			return respMap
		}

		// validate cart
		err = cart.AddCart()
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "error\n failed while adding cart"
			respMap["trace"] = err

			return respMap
		}

		respMap["response"] = "success"
		respMap["cart"] = cart

		return respMap
	}

	return respMap
}
