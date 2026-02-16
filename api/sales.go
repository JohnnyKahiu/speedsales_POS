package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/JohnnyKahiu/speedsales/poserver/internal/cash"
)

func CashSalesGet(w http.ResponseWriter, r *http.Request) {
	respMap := cash.Get(w, r)

	jStr, err := json.Marshal(respMap)
	if err != nil {
		log.Println("failed to marshal cashSalesGet()  err =", err)
	}

	EnableCors(&w)
	w.Write(jStr)
}

func CashSalesPost(w http.ResponseWriter, r *http.Request) {
	fmt.Println("cash sales post")
	respMap := cash.Post(w, r)

	jStr, err := json.Marshal(respMap)
	if err != nil {
		log.Println("failed to marshal cashSalesPost()  err =", err)
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

	// return response text
	w.Write(jStr)
}
