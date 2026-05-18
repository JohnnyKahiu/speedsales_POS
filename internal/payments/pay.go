package payments

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/JohnnyKahiu/speedsales/poserver/pkg/logins"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/payment"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/sales"
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
			respMap["message"] = "failed to commit payment"
			respMap["trace"] = err
			return respMap
		}

		respMap["response"] = "success"
		respMap["tendered"] = pay.Tendered
		respMap["change"] = pay.Change
		respMap["values"] = pay
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
	}

	return respMap
}
