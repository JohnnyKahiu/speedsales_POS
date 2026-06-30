package cash

import (
	"encoding/json"
	"net/http"

	"github.com/JohnnyKahiu/speedsales/poserver/pkg/logins"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/sales"
	"github.com/gorilla/mux"
)

func Delete(w http.ResponseWriter, r *http.Request) map[string]interface{} {
	respMap := make(map[string]interface{})

	userStr := r.Header.Get("user_details")
	if userStr == "" {
		respMap["response"] = "error"
		respMap["message"] = "user details not found"
		return respMap
	}

	details := logins.Users{}
	json.Unmarshal([]byte(userStr), &details)

	if !details.MakeSales {
		respMap["response"] = "forbidden"
		respMap["message"] = "forbidden"
		return respMap
	}

	vars := mux.Vars(r)
	switch vars["module"] {

	// DELETE /sales/cash/cart?id={receipt_item}
	case "cart":
		id := r.URL.Query().Get("id")
		if id == "" {
			respMap["response"] = "error"
			respMap["message"] = "id is required"
			return respMap
		}

		rcpt := sales.ReceiptLog{}
		if err := rcpt.DeleteCartItem(r.Context(), id); err != nil {
			respMap["response"] = "error"
			respMap["message"] = err.Error()
			return respMap
		}

		respMap["response"] = "success"
		respMap["cart"] = rcpt.Cart
		return respMap

	}

	return respMap
}
