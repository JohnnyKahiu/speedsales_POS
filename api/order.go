package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/JohnnyKahiu/speedsales/poserver/internal/order"
)

func OrderSalesGet(w http.ResponseWriter, r *http.Request) {
	respMap := order.Get(w, r)

	jStr, err := json.Marshal(respMap)
	if err != nil {
		log.Println("failed to marshal cashSalesGet()  err =", err)
	}

	EnableCors(&w)
	// write status code headers
	if respMap["response"] == "forbidden" {
		w.WriteHeader(http.StatusForbidden)
	}
	if respMap["response"] == "error" {
		w.WriteHeader(http.StatusInternalServerError)
	}
	if respMap["response"] == "success" {
		w.WriteHeader(http.StatusOK)
	}

	w.Write(jStr)
}

func OrderSalesPost(w http.ResponseWriter, r *http.Request) {
	respMap := order.Post(w, r)

	jStr, err := json.Marshal(respMap)
	if err != nil {
		log.Println("failed to marshal cashSalesGet()  err =", err)
	}

	EnableCors(&w)
	// write status code headers
	if respMap["response"] == "forbidden" {
		w.WriteHeader(http.StatusForbidden)
	}
	if respMap["response"] == "error" {
		w.WriteHeader(http.StatusInternalServerError)
	}
	if respMap["response"] == "success" {
		w.WriteHeader(http.StatusOK)
	}

	w.Write(jStr)
}
