package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/JohnnyKahiu/speedsales/poserver/internal/laybyes"
)

func LaybyeSalesGet(w http.ResponseWriter, r *http.Request) {
	respMap := laybyes.Get(w, r)

	jStr, err := json.Marshal(respMap)
	if err != nil {
		log.Println("failed to marshal LaybyeSalesGet() err =", err)
	}

	EnableCors(&w)
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

func LaybyeSalesPost(w http.ResponseWriter, r *http.Request) {
	respMap := laybyes.Post(w, r)

	jStr, err := json.Marshal(respMap)
	if err != nil {
		log.Println("failed to marshal LaybyeSalesPost() err =", err)
	}

	EnableCors(&w)
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
