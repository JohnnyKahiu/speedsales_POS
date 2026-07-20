package credit

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/JohnnyKahiu/speedsales/poserver/pkg/credit"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/logins"
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

	switch m {
	case "payment":
		if !details.AcceptPayment {
			w.WriteHeader(http.StatusForbidden)
			respMap["response"] = "forbidden"
			respMap["message"] = "forbidden"
			return respMap
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "params error"
			return respMap
		}

		acTrans := credit.AcTrans{}
		if err := json.Unmarshal(b, &acTrans); err != nil {
			respMap["response"] = "error"
			respMap["message"] = "invalid request body"
			return respMap
		}

		acTrans.TransType = "payment"
		acTrans.Amount = 0
		acTrans.TillNum = details.TillNum
		acTrans.ServedBy = details.Username

		if err := acTrans.AddAccountTxn(r.Context()); err != nil {
			respMap["response"] = "error"
			respMap["message"] = "error"
			return respMap
		}

		tp := sales.TillPayment{
			PaymentFor: "credit_pay",
			TillNum:    details.TillNum,
			CreditID:   acTrans.AutoID,
			Cash:       acTrans.PayDetails.Cash,
			Mpesa:      acTrans.PayDetails.Mpesa,
			Ecard:      acTrans.PayDetails.Ecard,
			Cheque:     acTrans.PayDetails.Check,
		}
		if err := tp.RecordAndPublish(r.Context()); err != nil {
			log.Println("error. failed to record till_payment for credit_pay    err =", err)
		}

		respMap["response"] = "success"
		respMap["receipt"] = acTrans
		return respMap
	}

	return respMap
}
