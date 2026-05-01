package payments

import (
	"encoding/json"
	"net/http"

	"github.com/JohnnyKahiu/speedsales/poserver/pkg/logins"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/sales"
	"github.com/gorilla/mux"
)

func POST(w http.ResponseWriter, r *http.Request) map[string]interface{} {
	respMap := make(map[string]interface{})

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
