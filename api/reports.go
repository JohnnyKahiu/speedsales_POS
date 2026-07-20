package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/JohnnyKahiu/speedsales/poserver/internal/reports"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/logins"
)

func TillReportGet(w http.ResponseWriter, r *http.Request) {
	EnableCors(&w)
	w.Header().Set("Content-Type", "application/json")

	userStr := r.Header.Get("user_details")
	details := logins.Users{}
	json.Unmarshal([]byte(userStr), &details)

	canSeeAll := details.AccessSalesReports

	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	cashier := r.URL.Query().Get("cashier")
	tillStr := r.URL.Query().Get("till")
	tillNo, _ := strconv.ParseInt(tillStr, 10, 64)

	if start == "" || end == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"response":"error","message":"start and end dates are required"}`))
		return
	}

	// Non-managers can only view their own data
	if !canSeeAll {
		cashier = details.Username
		tillNo = 0
	}

	data, err := reports.FetchTillReport(r.Context(), start, end, cashier, tillNo)
	if err != nil {
		log.Println("TillReportGet error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"response":"error","message":"failed to fetch report"}`))
		return
	}

	resp := map[string]interface{}{
		"response": "success",
		"values":   data,
	}
	json.NewEncoder(w).Encode(resp)
}

func ProductReportGet(w http.ResponseWriter, r *http.Request) {
	EnableCors(&w)
	w.Header().Set("Content-Type", "application/json")

	userStr := r.Header.Get("user_details")
	details := logins.Users{}
	json.Unmarshal([]byte(userStr), &details)

	canSeeAll := details.AccessSalesReports

	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	search := r.URL.Query().Get("search")
	cashier := r.URL.Query().Get("cashier")

	if start == "" || end == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"response":"error","message":"start and end dates are required"}`))
		return
	}

	// Non-managers can only view their own sales
	if !canSeeAll {
		cashier = details.Username
	}

	data, err := reports.FetchProductSales(r.Context(), start, end, cashier, search)
	if err != nil {
		log.Println("ProductReportGet error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"response":"error","message":"failed to fetch report"}`))
		return
	}

	resp := map[string]interface{}{
		"response": "success",
		"values":   data,
	}
	json.NewEncoder(w).Encode(resp)
}

func ReceiptsReportGet(w http.ResponseWriter, r *http.Request) {
	EnableCors(&w)
	w.Header().Set("Content-Type", "application/json")

	userStr := r.Header.Get("user_details")
	details := logins.Users{}
	json.Unmarshal([]byte(userStr), &details)

	canSeeAll := details.AccessSalesReports

	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	cashier := r.URL.Query().Get("cashier")

	if start == "" || end == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"response":"error","message":"start and end dates are required"}`))
		return
	}

	if !canSeeAll {
		cashier = details.Username
	}

	data, err := reports.FetchReceiptList(r.Context(), start, end, cashier)
	if err != nil {
		log.Println("ReceiptsReportGet error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"response":"error","message":"failed to fetch receipts"}`))
		return
	}

	resp := map[string]interface{}{
		"response": "success",
		"values":   data,
	}
	json.NewEncoder(w).Encode(resp)
}
