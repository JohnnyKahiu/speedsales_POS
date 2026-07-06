package sales_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
	"github.com/JohnnyKahiu/speedsales/poserver/internal/cash"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/logins"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/sales"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

// Test fixtures — use recognisable values so they are easy to spot and clean up.
const (
	testTillNo     = int64(9999999999)
	testReceiptNum = int64(9999000001)
	testReceiptItem = "test_receipt_item_delete_001"
)

// TestMain sets up a real PostgreSQL connection, seeds the test tables,
// runs all tests in this package, then removes the test rows.
func TestMain(m *testing.M) {
	// load env so DB credentials are available
	if err := godotenv.Load("../../.env"); err != nil {
		fmt.Println("warning: could not load .env file:", err)
	}

	conf := database.DBConf{
		Server: os.Getenv("DB_HOST"),
		Port:   os.Getenv("DB_PORT"),
		DbName: os.Getenv("DB_NAME"),
	}

	pool, err := conf.NewPgPool()
	if err != nil {
		fmt.Println("FATAL: could not connect to test database:", err)
		os.Exit(1)
	}
	database.PgPool = pool
	defer pool.Close()

	if err := seedTestData(); err != nil {
		fmt.Println("FATAL: could not seed test data:", err)
		os.Exit(1)
	}

	code := m.Run()

	cleanupTestData()
	os.Exit(code)
}

// seedTestData inserts the minimum rows needed by the tests.
// sales_till is seeded first because salestrace has a FK to it.
func seedTestData() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// test till — ON CONFLICT DO NOTHING is safe if it already exists
	_, err := database.PgPool.Exec(ctx,
		`INSERT INTO sales_till(till_no, teller, supervisor, branch, open_float)
		 VALUES($1, 'test_teller', 'test_supervisor', 'Test', 0)
		 ON CONFLICT DO NOTHING`,
		testTillNo,
	)
	if err != nil {
		return fmt.Errorf("seed sales_till: %w", err)
	}

	// build a cart with one pending item that we will delete in tests
	cart := []sales.Sales{
		{
			ItemCode:    "TEST_ITEM_001",
			ItemName:    "Test Item",
			Quantity:    2,
			Price:       100,
			State:       "pending",
			ReceiptItem: testReceiptItem,
		},
	}
	cartJSON, _ := json.Marshal(cart)

	_, err = database.PgPool.Exec(ctx,
		`INSERT INTO salestrace(receipt_num, till_num, poster, branch, state, cart)
		 VALUES($1, $2, 'test_poster', 'Test', 'pending', $3)
		 ON CONFLICT DO NOTHING`,
		testReceiptNum, testTillNo, string(cartJSON),
	)
	if err != nil {
		return fmt.Errorf("seed salestrace: %w", err)
	}

	return nil
}

// cleanupTestData removes all rows inserted by seedTestData.
func cleanupTestData() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	database.PgPool.Exec(ctx,
		`DELETE FROM salestrace WHERE receipt_num = $1`, testReceiptNum)
	database.PgPool.Exec(ctx,
		`DELETE FROM sales_till WHERE till_no = $1`, testTillNo)
}

// resetCart restores the test cart to its original state so each test
// that reads or mutates the cart starts from a known position.
func resetCart(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cart := []sales.Sales{
		{
			ItemCode:    "TEST_ITEM_001",
			ItemName:    "Test Item",
			Quantity:    2,
			Price:       100,
			State:       "pending",
			ReceiptItem: testReceiptItem,
		},
	}
	cartJSON, _ := json.Marshal(cart)

	_, err := database.PgPool.Exec(ctx,
		`UPDATE salestrace SET cart = $1, state = 'pending' WHERE receipt_num = $2`,
		string(cartJSON), testReceiptNum,
	)
	if err != nil {
		t.Fatalf("resetCart: %v", err)
	}
}

// cartItemState queries the DB and returns the state of a specific receipt_item.
func cartItemState(t *testing.T, receiptItem string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	row := database.PgPool.QueryRow(ctx,
		`SELECT coalesce(cart::varchar, '[]') FROM salestrace WHERE receipt_num = $1`,
		testReceiptNum,
	)

	cartStr := ""
	if err := row.Scan(&cartStr); err != nil {
		t.Fatalf("cartItemState scan: %v", err)
	}

	var cart []sales.Sales
	json.Unmarshal([]byte(cartStr), &cart)

	for _, item := range cart {
		if item.ReceiptItem == receiptItem {
			return item.State
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// pkg/sales DeleteCartItem tests
// ---------------------------------------------------------------------------

// TestDeleteCartItem_Success verifies that a pending cart item is marked DELETED
// in the database after calling DeleteCartItem with a valid receipt_item.
func TestDeleteCartItem_Success(t *testing.T) {
	resetCart(t)

	rcpt := sales.ReceiptLog{}
	err := rcpt.DeleteCartItem(context.Background(), testReceiptItem)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	state := cartItemState(t, testReceiptItem)
	if state != "DELETED" {
		t.Errorf("expected item state DELETED, got %q", state)
	}
}

// TestDeleteCartItem_NotFound verifies that DeleteCartItem returns an error
// when the receipt_item does not exist in any active receipt.
func TestDeleteCartItem_NotFound(t *testing.T) {
	rcpt := sales.ReceiptLog{}
	err := rcpt.DeleteCartItem(context.Background(), "non_existent_item_xyz")
	if err == nil {
		t.Fatal("expected an error for a missing receipt_item, got nil")
	}
}

// ---------------------------------------------------------------------------
// internal/cash Delete handler tests
// ---------------------------------------------------------------------------

// newCashDeleteRequest builds a DELETE request routed through a minimal mux
// so mux.Vars(r) returns {"module": "cart"} as it would in production.
func newCashDeleteRequest(url string, userDetails logins.Users) (*httptest.ResponseRecorder, *http.Request) {
	req := httptest.NewRequest(http.MethodDelete, url, nil)

	userJSON, _ := json.Marshal(userDetails)
	req.Header.Set("user_details", string(userJSON))

	rr := httptest.NewRecorder()
	return rr, req
}

// routedCashDelete serves the request through a real gorilla/mux router so
// path variables are populated exactly as in production.
func routedCashDelete(rr *httptest.ResponseRecorder, req *http.Request) {
	r := mux.NewRouter()
	r.HandleFunc("/sales/cash/{module}", func(w http.ResponseWriter, r *http.Request) {
		respMap := cash.Delete(w, r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(respMap)
	}).Methods(http.MethodDelete)
	r.ServeHTTP(rr, req)
}

// TestCashDelete_Success verifies the full DELETE /sales/cash/cart handler
// returns HTTP 200 and response "success" for a valid user and existing item.
func TestCashDelete_Success(t *testing.T) {
	resetCart(t)

	url := fmt.Sprintf("/sales/cash/cart?id=%s", testReceiptItem)
	rr, req := newCashDeleteRequest(url, logins.Users{MakeSales: true})
	routedCashDelete(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["response"] != "success" {
		t.Errorf("expected response=success, got %v", resp["response"])
	}
}

// TestCashDelete_MissingID verifies the handler returns HTTP 500 and an error
// message when the required ?id query parameter is absent.
func TestCashDelete_MissingID(t *testing.T) {
	rr, req := newCashDeleteRequest("/sales/cash/cart", logins.Users{MakeSales: true})
	routedCashDelete(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["response"] != "error" {
		t.Errorf("expected response=error, got %v", resp["response"])
	}
}

// TestCashDelete_Forbidden verifies the handler returns HTTP 403 when the
// requesting user does not have the MakeSales permission.
func TestCashDelete_Forbidden(t *testing.T) {
	rr, req := newCashDeleteRequest(
		fmt.Sprintf("/sales/cash/cart?id=%s", testReceiptItem),
		logins.Users{MakeSales: false},
	)
	routedCashDelete(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["response"] != "forbidden" {
		t.Errorf("expected response=forbidden, got %v", resp["response"])
	}
}

// TestCashDelete_ItemNotFound verifies the handler returns HTTP 500 when
// the receipt_item ID does not exist in any active receipt.
func TestCashDelete_ItemNotFound(t *testing.T) {
	rr, req := newCashDeleteRequest(
		"/sales/cash/cart?id=ghost_item_does_not_exist",
		logins.Users{MakeSales: true},
	)
	routedCashDelete(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}
