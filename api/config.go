package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

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
