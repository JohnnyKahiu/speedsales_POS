package order

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

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

		recptStr := r.URL.Query().Get("receipt")
		fmt.Println("receipt = ", recptStr)

		receipt, _ := strconv.ParseInt(recptStr, 10, 64)

		ord := sales.Order{ReceiptNum: receipt}
		fmt.Println("bill_num =", ord.ReceiptNum)

		vals, err := ord.GetOrdersInBills(r.Context())
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "error fetching orders in bill"
			respMap["trace"] = err
			return respMap
		}

		respMap["response"] = "success"
		respMap["values"] = vals
		respMap["total"] = ord.Total
		return respMap

	case "cart":
		if !details.MakeSales {
			respMap["response"] = "forbidden"
			respMap["message"] = "forbidden"
			return respMap
		}

		orderNum := r.URL.Query().Get("order_num")
		if orderNum == "" {
			respMap["response"] = "error"
			respMap["message"] = "order_num is null"
			return respMap
		}

		ordNum, _ := strconv.ParseInt(orderNum, 10, 64)

		ord := sales.Order{OrderNum: ordNum}
		err := ord.Fetchtems(r.Context())
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "failed to fetch cart items"
			return respMap
		}

		respMap["response"] = "success"
		respMap["values"] = ord.OrderItems
		respMap["total"] = ord.CalcTotal()

		fmt.Println("order_num =", orderNum)
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
	// log.Printf("user_details = %s", userStr)

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

		stkLoc := details.StkLocation
		if stkLoc == "" {
			stkLoc = "Store"
		}

		ord := sales.Order{
			StkLocation: stkLoc,
			CompanyID:   details.CompanyID,
			Poster:      details.Username,
			TillNum:     details.TillNum,
		}
		err = json.Unmarshal(b, &ord)
		if err != nil {
			log.Println("failed to unmarshal body    err =", err)
			respMap["response"] = "error"
			respMap["message"] = "bad request"
			respMap["trace"] = err
			return respMap
		}
		// Only a branch-less/"all" user's own request may pick the branch
		// (via the frontend's operating-branch modal) — a fixed-branch
		// cashier's branch always wins, no matter what the body says.
		ord.Branch = details.ResolveBranch(ord.Branch)

		// add item to cart
		cart, total, err := ord.AddToOrder(r.Context(), ord.OrderItems[0])
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

	case "update-qty":
		b, err := io.ReadAll(r.Body)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "bad request"
			return respMap
		}

		var req struct {
			ReceiptItem string  `json:"receipt_item"`
			OrderNum    string  `json:"order_num"`
			Quantity    float64 `json:"quantity"`
		}
		if err = json.Unmarshal(b, &req); err != nil || req.ReceiptItem == "" || req.OrderNum == "" {
			respMap["response"] = "error"
			respMap["message"] = "receipt_item, order_num and quantity are required"
			return respMap
		}

		cart, total, err := sales.UpdateOrderItemQty(r.Context(), req.ReceiptItem, req.OrderNum, req.Quantity)
		if err != nil {
			log.Println("update-qty error:", err)
			respMap["response"] = "error"
			respMap["message"] = "failed to update quantity"
			return respMap
		}

		respMap["response"] = "success"
		respMap["cart"] = cart
		respMap["total"] = total
		return respMap

	case "complete":
		b, err := io.ReadAll(r.Body)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "wrong params"
			return respMap
		}

		stkLoc := details.StkLocation
		if stkLoc == "" {
			stkLoc = "Store"
		}
		ord := sales.Order{StkLocation: stkLoc}
		err = json.Unmarshal(b, &ord)
		if err != nil {
			log.Println("failed to unmarshal body    err =", err)
			respMap["response"] = "error"
			respMap["message"] = "bad request"
			respMap["trace"] = err
			return respMap
		}
		// See add-cart above: only a branch-less/"all" user's own request
		// may pick the branch.
		ord.Branch = details.ResolveBranch(ord.Branch)

		cart, err := ord.CompleteOrder(r.Context())
		if err != nil {
			log.Println("completeOrder failure     err =", err)
			respMap["response"] = "error"
			respMap["message"] = "failed to complete order"
			respMap["trace"] = err
			return respMap
		}

		fmt.Println("\t order complete successful")
		respMap["response"] = "success"
		respMap["cart"] = cart
		return respMap
	}

	return respMap
}

func Delete(w http.ResponseWriter, r *http.Request) map[string]interface{} {
	respMap := make(map[string]interface{})

	vars := mux.Vars(r)
	m := vars["module"]

	fmt.Println("order delete route = ", m)

	switch m {
	case "order-item":
		b, err := io.ReadAll(r.Body)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "params error"
			return respMap
		}

		fmt.Printf("\t body = %s \n", b)

		itm := struct {
			AutoID   string `json:"auto_id"`
			Approver string `json:"approver"`
			Token    string `json:"auth_token"`
			Receipt  string `json:"receipt"`
			OrderNum string `json:"order_num"`
		}{}

		err = json.Unmarshal(b, &itm)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "json error"
			return respMap
		}

		fmt.Printf("\t receipt_item = %v \t order_num = %v", itm.AutoID, itm.OrderNum)

		cart, total, err := sales.DelOrderItem(r.Context(), itm.AutoID, itm.OrderNum)
		if err != nil {
			respMap["response"] = "error"
			respMap["message"] = "failed to delete order item"
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
