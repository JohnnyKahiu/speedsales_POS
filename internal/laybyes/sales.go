package laybyes

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/laybye"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/logins"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
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
	default:
		respMap["response"] = "error"
		respMap["message"] = "unknown module: " + m
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

	vars := mux.Vars(r)
	m := vars["module"]

	switch m {
	case "new":
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

		lay := laybye.Laybye{}
		if err := json.Unmarshal(b, &lay); err != nil {
			respMap["response"] = "error"
			respMap["message"] = "invalid request body"
			return respMap
		}

		lay.Poster = details.Username

		tx, err := database.PgPool.BeginTx(r.Context(), pgx.TxOptions{})
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "failed to start transaction"
			return respMap
		}
		defer tx.Rollback(r.Context())

		if err := lay.Register(r.Context(), tx); err != nil {
			respMap["response"] = "error"
			respMap["message"] = err.Error()
			return respMap
		}

		respMap["response"] = "success"
		respMap["laybye"] = lay
		return respMap

	case "add-item":
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

		item := laybye.LaybyeItem{}
		if err := json.Unmarshal(b, &item); err != nil {
			respMap["response"] = "error"
			respMap["message"] = "invalid request body"
			return respMap
		}

		tx, err := database.PgPool.BeginTx(r.Context(), pgx.TxOptions{})
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "failed to start transaction"
			return respMap
		}
		defer tx.Rollback(r.Context())

		if err := item.AddItem(r.Context(), tx); err != nil {
			respMap["response"] = "error"
			respMap["message"] = err.Error()
			return respMap
		}

		respMap["response"] = "success"
		respMap["item"] = item
		return respMap

	case "payment":
		if !details.AcceptPayment {
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

		pay := laybye.Payment{TillNum: details.TillNum, StkLocation: details.StkLocation}
		if err := json.Unmarshal(b, &pay); err != nil {
			respMap["response"] = "error"
			respMap["message"] = "invalid request body"
			return respMap
		}
		// Only a branch-less/"all" user's own request may pick the branch.
		pay.Branch = details.ResolveBranch(pay.Branch)

		tp, err := pay.Pay(r.Context())
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = err.Error()
			return respMap
		}

		respMap["response"] = "success"
		respMap["payment"] = tp
		return respMap

	default:
		respMap["response"] = "error"
		respMap["message"] = "unknown module: " + m
	}

	return respMap
}
