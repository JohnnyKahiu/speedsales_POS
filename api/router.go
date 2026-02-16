package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/JohnnyKahiu/speedsales/poserver/pkg/authentication"
	"github.com/gorilla/mux"
)

var mySigningKey = []byte(os.Getenv("SPEEDSALESJWTKEY"))

func EnableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Headers", "*")
}

func NewRouter() *mux.Router {
	fmt.Println("http routing")

	// rentals.CreateTables()

	r := mux.NewRouter()

	// r.HandleFunc("/ws", socketHandler)
	r.Use(JwtMiddleware)

	r.HandleFunc("/sales/cash/{module}", CashSalesGet).Methods("GET", "OPTIONS")
	r.HandleFunc("/sales/cash/{module}", CashSalesPost).Methods("POST", "OPTIONS")
	// r.HandleFunc("/sales/order/{module}", CashSalesPost).Methods("POST", "OPTIONS")

	// r.HandleFunc("/sales/payments/{module}", CashSalesPost).Methods("POST", "OPTIONS")
	// r.HandleFunc("/sales/credit/{module}", CashSalesPost).Methods("POST", "OPTIONS")
	// r.HandleFunc("/sales/laybye/{module}", CashSalesPost).Methods("POST", "OPTIONS")

	// r.HandleFunc("/sms", sms.Post).Methods("POST", "OPTIONS")

	return r
}

func JwtMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		EnableCors(&w)

		tokenString := r.Header.Get("token")
		if tokenString == "" {
			log.Println("\n\t token string not provided")

			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"response": "error", "message": "unauthorized"}`))
			return
		}

		user, authentic := authentication.ValidateJWT(tokenString)
		if !authentic {
			log.Println("\n\t token string is not valid")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"response": "error", "message": "unauthorized"}`))
			return
		}

		juser, _ := json.Marshal(user)

		r.Header.Set("user_details", string(juser))
		next.ServeHTTP(w, r)
	})
}
