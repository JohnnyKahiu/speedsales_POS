package api

import (
	"encoding/json"
	"net/http"

	"github.com/JohnnyKahiu/speedsales/poserver/internal/payments"
)

func PaymentGet(w http.ResponseWriter, r *http.Request) {
	EnableCors(&w)

	respMap := payments.GET(w, r)
	if respMap["response"] == "forbidden" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	jstr, err := json.Marshal(respMap)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(jstr)
}

func PaymentPOST(w http.ResponseWriter, r *http.Request) {
	EnableCors(&w)

	respMap := payments.POST(w, r)
	if respMap["response"] == "forbidden" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	jstr, err := json.Marshal(respMap)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(jstr)
}
