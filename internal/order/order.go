package order

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/pkg/logins"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/sales"
	"github.com/gorilla/mux"
)

func Get(w http.ResponseWriter, r *http.Request) map[string]interface{} {
	respMap := make(map[string]interface{})
	fmt.Println("orders in bill")

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
	case "orders_in_bill":
		if !details.MakeSales {
			respMap["response"] = "forbidden"
			respMap["message"] = "forbidden"
			return respMap
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "error reading request body"
			return respMap
		}

		fmt.Println("body = ", string(b))

		recptStr := r.URL.Query().Get("receipt")

		receipt, _ := strconv.ParseInt(recptStr, 10, 64)

		ord := sales.Order{ReceiptNum: receipt}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		vals, err := ord.GetOrdersInBills(ctx)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "error fetching orders in bill"
			respMap["trace"] = err
			return respMap
		}

		respMap["response"] = "success"
		respMap["values"] = vals
		return respMap

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

	// ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	// defcer cancel()

	vars := mux.Vars(r)
	m := vars["module"]

	fmt.Println("cash post module = ", m)

	switch m {
	case "add-cart":
		fmt.Println("adding to order_cart")
		if !details.MakeSales {
			respMap["response"] = "forbidden"
			return respMap
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "wrong params"
			return respMap
		}

		ord := sales.Order{
			Branch:    details.Branch,
			CompanyID: details.CompanyID,
			Poster:    details.Username,
			TillNum:   details.TillNum,
		}
		err = json.Unmarshal(b, &ord)
		if err != nil {
			log.Println("failed to unmarshal body    err =", err)
			respMap["response"] = "error"
			respMap["message"] = "bad request"
			respMap["trace"] = err
			return respMap
		}

		fmt.Println("order =", string(b))

		fmt.Println("\t receipt_num =", ord.ReceiptNum)
		// log.Fatalln("\t order_cart =", ord.OrderItems)

		cart, total, err := ord.AddToOrder(ord.OrderItems[0])
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "failed to add order to cart"
			respMap["trace"] = err
			return respMap
		}

		respMap["response"] = "success"
		respMap["cart"] = cart
		respMap["total"] = total
		return respMap

	}

	return respMap
}
