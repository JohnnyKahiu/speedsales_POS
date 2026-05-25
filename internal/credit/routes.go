package credit

import (
	"encoding/json"
	"net/http"

	"github.com/JohnnyKahiu/speedsales/poserver/pkg/credit"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/logins"
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

	switch m {
	case "payment":
		if !details.AcceptPayment {
			w.WriteHeader(http.StatusForbidden)
			respMap["response"] = "forbidden"
			respMap["message"] = "forbidden"
			return respMap
		}

		acTrans := credit.AcTrans{}
		err := acTrans.AddAccountTxn(r.Context())
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "error"
			return respMap
		}

		respMap["response"] = "success"
		respMap["receipt"] = acTrans
		return respMap
	}

	return respMap
}
