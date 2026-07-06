package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/JohnnyKahiu/speedsales/poserver/internal/settings"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/variables"
)

func ConfigsGet(w http.ResponseWriter, r *http.Request) {
	fmt.Println("configs get")

	// get configs
	settings, err := variables.FetchDefaults()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Println("settings =", settings)

	respMap := make(map[string]interface{})
	respMap["response"] = "success"
	respMap["values"] = settings

	jstr, err := json.Marshal(respMap)
	if err != nil {
		log.Println("error failed to marshall into json   err =", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(jstr)
}

func ConfigsPOST(w http.ResponseWriter, r *http.Request) {
	respMap := settings.POST(w, r)

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
