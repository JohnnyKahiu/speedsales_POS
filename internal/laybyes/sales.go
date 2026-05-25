package laybyes

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/JohnnyKahiu/speedsales/poserver/pkg/laybye"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/logins"
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
		lay.Branch = details.Branch
		lay.CompanyID = details.CompanyID

		if err := lay.Register(r.Context()); err != nil {
			respMap["response"] = "error"
			respMap["message"] = err.Error()
			return respMap
		}

		respMap["response"] = "success"
		respMap["laybye"] = lay
		return respMap

	default:
		respMap["response"] = "error"
		respMap["message"] = "unknown module: " + m
	}

	return respMap
}
