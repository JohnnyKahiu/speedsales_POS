package settings

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/JohnnyKahiu/speedsales/poserver/pkg/logins"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/variables"
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

	body, err := io.ReadAll(r.Body)
	if err != nil {
		respMap["response"] = "error"
		respMap["message"] = "failed to read request body"
		return respMap
	}

	vars := mux.Vars(r)

	switch vars["module"] {
	case "pos_settings":
		s := variables.PosSettings{}
		if err := json.Unmarshal(body, &s); err != nil {
			respMap["response"] = "error"
			respMap["message"] = "invalid body"
			return respMap
		}
		if err := variables.UpdatePosSettings(s); err != nil {
			log.Println("error updating pos_settings    err =", err)
			respMap["response"] = "error"
			respMap["message"] = "failed to update pos settings"
			return respMap
		}

	case "vat_settings":
		codes := map[string]float32{}
		if err := json.Unmarshal(body, &codes); err != nil {
			respMap["response"] = "error"
			respMap["message"] = "invalid body"
			return respMap
		}
		if err := variables.UpdateVatCodes(codes); err != nil {
			log.Println("error updating vat_settings    err =", err)
			respMap["response"] = "error"
			respMap["message"] = "failed to update vat codes"
			return respMap
		}

	case "doc_head":
		h := variables.DocHead{}
		if err := json.Unmarshal(body, &h); err != nil {
			respMap["response"] = "error"
			respMap["message"] = "invalid body"
			return respMap
		}
		if err := variables.UpdateDocHeader(h); err != nil {
			log.Println("error updating doc_head    err =", err)
			respMap["response"] = "error"
			respMap["message"] = "failed to update doc header"
			return respMap
		}

	case "doc_footer":
		f := variables.DocFooter{}
		if err := json.Unmarshal(body, &f); err != nil {
			respMap["response"] = "error"
			respMap["message"] = "invalid body"
			return respMap
		}
		if err := variables.UpdateDocFooter(f); err != nil {
			log.Println("error updating doc_footer    err =", err)
			respMap["response"] = "error"
			respMap["message"] = "failed to update doc footer"
			return respMap
		}

	default:
		respMap["response"] = "error"
		respMap["message"] = "unknown module"
		return respMap
	}

	respMap["response"] = "success"
	return respMap
}
