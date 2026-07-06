# POS Microservice — Performance Report

**Scope:** `POS/pkg/sales/`, `POS/internal/cash/`, `POS/internal/order/`, `POS/pkg/broker/`  
**Date:** 2026-06-30  

---

## Table of Contents

1. [External HTTP call inside an open DB transaction](#1-external-http-call-inside-an-open-db-transaction)
2. [Kafka Writer created and destroyed per order](#2-kafka-writer-created-and-destroyed-per-order)
3. [CashInTill — five-way JOIN on every cart load](#3-cashintill--five-way-join-on-every-cart-load)
4. [CreateReceipt — MAX full scan and concurrent receipt race](#4-createreceipt--max-full-scan-and-concurrent-receipt-race)
5. [GetOrdersInBills / SetOrderPay — JSONB expansion without index](#5-getordersinbills--setorderpay--jsonb-expansion-without-index)
6. [DelOrderItem / UpdateOrderItemQty — read-modify-write via Go](#6-delorderitem--updateorderitemqty--read-modify-write-via-go)
7. [log.Fatalln inside an HTTP handler](#7-logfatalln-inside-an-http-handler)
8. [AddToOrder — wrong nil-check, OrderNum never validated](#8-addtoorder--wrong-nil-check-ordernum-never-validated)
9. [70+ fmt.Print calls in request hot paths](#9-70-fmtprint-calls-in-request-hot-paths)
10. [Kafka.NewConn calls log.Fatalf on broker failure](#10-kafkanewconn-calls-logfatalf-on-broker-failure)

---

## 1. External HTTP call inside an open DB transaction

**File:** `pkg/sales/orders.go` — `AddToOrder()`  
**Severity:** High  

### What is happening

`AddToOrder` fetches product details from the inventory microservice before opening a
Postgres transaction. Both the remote fetch and the transaction share a single 30-second
`context.WithTimeout`:

```go
func (ord *Order) AddToOrder(ctx context.Context, args Sales) ([]Sales, float64, error) {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // ← HTTP/gRPC call to inventory service (can take 2–15 s under load)
    p := products.StockMaster{ItemCode: args.ItemCode}
    err := p.Fetch(ctx)
    ...

    // ← transaction opened with the same (already partially consumed) context
    tx, err := db.PgPool.Begin(ctx)
```

If the inventory service takes 10 seconds, the database transaction has at most 20 seconds
remaining. During an inventory service degradation this cascades: DB connections stay open
and idle waiting for a context that is close to expiring, DB connection pool slots are
exhausted, and unrelated requests begin to queue.

### Suggestion

Use separate, independently-scoped contexts: a short one (5–8 s) for the network call and
a clean one (full `r.Context()`) for the transaction:

```go
func (ord *Order) AddToOrder(ctx context.Context, args Sales) ([]Sales, float64, error) {
    // 1. fetch product details — short deadline, no DB connection held
    fetchCtx, fetchCancel := context.WithTimeout(ctx, 6*time.Second)
    defer fetchCancel()

    p := products.StockMaster{ItemCode: args.ItemCode}
    if err := p.Fetch(fetchCtx); err != nil {
        return nil, 0, err
    }

    // 2. open transaction with remaining request budget
    txCtx, txCancel := context.WithTimeout(ctx, 20*time.Second)
    defer txCancel()

    tx, err := db.PgPool.Begin(txCtx)
    ...
}
```

Additionally, consider a short-lived in-process product price cache (e.g. `sync.Map` with
a 60-second TTL per `item_code`) to skip the network call on repeated scans of the same
item within a single till session.

---

## 2. Kafka Writer created and destroyed per order

**File:** `pkg/broker/kafka.go` — `Produce()`, called from `pkg/sales/orders.go` — `CompleteOrder()`  
**Severity:** High  

### What is happening

Every call to `CompleteOrder` creates a new `kafka.Writer`, which performs a TCP
handshake and topic-metadata request before writing, then closes the connection:

```go
func (b *Kafka) Produce(ctx context.Context) error {
    writer := &kafka.Writer{          // ← new TCP connection on every order
        Addr:                   kafka.TCP(b.Broker),
        Topic:                  b.Topic,
        Balancer:               &kafka.LeastBytes{},
        AllowAutoTopicCreation: true,
    }
    defer writer.Close()              // ← connection torn down immediately after

    err := writer.WriteMessages(ctx, kafka.Message{...})
```

This adds 20–200 ms of connection overhead to every order completion, and `WriteMessages`
runs **synchronously in the HTTP handler** — a slow or unreachable Kafka broker stalls the
response and holds the goroutine.

### Suggestion

**Part 1 — Shared writer with connection reuse:**

```go
// pkg/broker/kafka.go

var sharedWriter *kafka.Writer
var writerOnce  sync.Once

func SharedWriter(addr, topic string) *kafka.Writer {
    writerOnce.Do(func() {
        sharedWriter = &kafka.Writer{
            Addr:         kafka.TCP(addr),
            Topic:        topic,
            Balancer:     &kafka.LeastBytes{},
            BatchTimeout: 5 * time.Millisecond,
        }
    })
    return sharedWriter
}

func (b *Kafka) Produce(ctx context.Context) error {
    w := SharedWriter(b.Broker, b.Topic)
    return w.WriteMessages(ctx, kafka.Message{
        Key:   []byte(b.Key),
        Value: b.Payload,
    })
}
```

**Part 2 — Fire-and-forget from the handler:**

```go
// pkg/sales/orders.go — CompleteOrder()

// publish asynchronously; handler responds immediately
go func(payload []byte, orderNum string) {
    kf := broker.Kafka{
        Broker:  os.Getenv("KAFKA_BROKER"),
        Topic:   "sales_orders",
        Key:     orderNum,
        Payload: payload,
    }
    if err := kf.Produce(context.Background()); err != nil {
        log.Printf("kafka: failed to publish order %s: %v", orderNum, err)
    }
}(payLoad, fmt.Sprintf("%v", ord.OrderNum))
```

The order is already written to `salesorders` before the publish; a downstream consumer
catching up on a restart is sufficient for kitchen display / production flows.

---

## 3. CashInTill — five-way JOIN on every cart load

**File:** `pkg/sales/sales.go` — `CashInTill()`  
**Severity:** High  

### What is happening

`CashInTill` is a five-table join that aggregates payment totals, rollups, credit
transactions, and laybye payments in one query:

```sql
SELECT
    coalesce(c.cash, 0) + coalesce(lay_payments.cash, 0) + coalesce(credit.cash, 0)
    - coalesce(rolls.amount, 0) cash_in_till
FROM
    (SELECT pay_till, SUM(...) FROM salestrace  WHERE pay_till = $1 AND state = 'POSTED' GROUP BY pay_till) as c
    LEFT JOIN (SELECT ... FROM cash_movement   WHERE type = 'cash rollup' GROUP BY till_num) as rolls ...
    LEFT JOIN (SELECT ... FROM accounts_txn    GROUP BY till_num) as credit ...
    LEFT JOIN (SELECT ... FROM laybye_trans    WHERE pay_type = 'cash' ...) as cash ...
    LEFT JOIN (SELECT ... FROM laybye_trans    WHERE pay_type = 'mpesa' ...) ...
    ...
```

Even though the call is currently bypassed in the `cart` handler
(`cashInTill := float64(0)`), the function is still called from other paths. When active,
this query runs on every cart-load request — the highest frequency endpoint — often several
times per minute per till.

### Suggestion

**Option A — Result cache per till (recommended):**

```go
// pkg/sales/sales.go

type tillCacheEntry struct {
    balance   float64
    expiresAt time.Time
}

var tillCache   = sync.Map{}
const tillCacheTTL = 45 * time.Second

func CashInTill(ctx context.Context, till int64) (float64, error) {
    key := fmt.Sprintf("%d", till)
    if v, ok := tillCache.Load(key); ok {
        e := v.(tillCacheEntry)
        if time.Now().Before(e.expiresAt) {
            return e.balance, nil
        }
    }

    // run the actual query
    balance, err := queryCashInTill(ctx, till)
    if err != nil {
        return 0, err
    }

    tillCache.Store(key, tillCacheEntry{balance: balance, expiresAt: time.Now().Add(tillCacheTTL)})
    return balance, nil
}
```

Invalidate the cache entry for `till` after a successful payment post.

**Option B — Materialised view updated on write:**

```sql
CREATE MATERIALIZED VIEW mv_till_balance AS
SELECT pay_till, SUM(...) AS cash_in_till FROM salestrace
WHERE state = 'POSTED' GROUP BY pay_till;

-- refresh on payment post (non-concurrently is fine; < 1 ms for small tills)
REFRESH MATERIALIZED VIEW CONCURRENTLY mv_till_balance;
```

---

## 4. CreateReceipt — MAX full scan and concurrent receipt race

**File:** `pkg/sales/sales.go` — `CreateReceipt()`  
**Severity:** High  

### What is happening

The receipt number is generated by scanning `MAX(daily_count)` across all of today's
`salestrace` rows:

```sql
SELECT CAST(CONCAT(
    cast(1 as varchar),
    extract(YEAR FROM now()),
    LPAD(EXTRACT(MONTH FROM now())::text, 2, '0'),
    LPAD(EXTRACT(DAY FROM now())::text, 2, '0'),
    '0',
    cast(coalesce(max(daily_count), 0) + 1 as varchar)
) AS BIGINT)
, coalesce(max(daily_count), 0) + 1
FROM salestrace WHERE trans_date::date = (SELECT now()::date)
```

**Two problems:**

1. **Index cannot be used.** The predicate `trans_date::date = now()::date` applies a
   function cast to the indexed column, which prevents B-tree index seeks. PostgreSQL
   performs a sequential scan on the entire `salestrace` table filtered by today's rows.

2. **Race condition on concurrent creates.** Two cashiers opening tills at the same
   instant both read `MAX = 0`, both generate `daily_count = 1`, and both try to
   `INSERT` with the same receipt number. The second insert fails with a primary-key
   violation. There is no lock or `SERIALIZABLE` transaction guarding this.

### Suggestion

**Fix the index — use a range predicate instead of a cast:**

```sql
WHERE trans_date >= date_trunc('day', now())
  AND trans_date <  date_trunc('day', now()) + interval '1 day'
```

Then create:

```sql
CREATE INDEX idx_salestrace_trans_date ON salestrace (trans_date);
```

**Fix the race — use a Postgres sequence per day:**

```sql
CREATE SEQUENCE IF NOT EXISTS daily_receipt_seq START 1;

-- reset at start of each business day via pg_cron or application startup
SELECT setval('daily_receipt_seq', 1, false);
```

```go
sql := `SELECT nextval('daily_receipt_seq')`
rows, err := database.PgPool.Query(ctx, sql)
```

Or, use `INSERT … ON CONFLICT DO UPDATE` with a unique partial index to absorb the race
at the database level rather than the application level.

---

## 5. GetOrdersInBills / SetOrderPay — JSONB expansion without index

**Files:** `pkg/sales/orders.go` — `GetOrdersInBills()`, `SetOrderPay()`  
**Severity:** Medium  

### What is happening

Order items are stored as a `JSONB` column (`order_items`). Two queries expand this column
for every matching row.

`GetOrdersInBills` returns the raw JSONB blob for every order in a bill, then parses it
in Go:

```go
// up to N orders × M items all deserialised on every bill load
err = json.Unmarshal([]byte(ordItms), &r.OrderItems)
```

`SetOrderPay` uses `jsonb_to_recordset` to expand items inline and copy them to
`sales_live`:

```sql
FROM salesorders ord, jsonb_to_recordset(ord.order_items) as
    items(trans_date timestamp, till_num bigint, item_code varchar, ...)
WHERE ord.order_num = $2
```

PostgreSQL must deserialise the full JSONB array for every matched row. There is no index
on `receipt_num` or `state` in `salesorders`, so the predicate also scans the full table.

### Suggestion

**Add a compound partial index:**

```sql
CREATE INDEX idx_salesorders_receipt_state
    ON salesorders (receipt_num, state)
    WHERE state NOT IN ('voided', 'VOIDED', 'DELETED');
```

**For SetOrderPay — avoid jsonb_to_recordset expansion:**

Maintain a normalised `salesorder_items` table alongside the JSONB column. Write to both
on `AddToOrder`; read from the normalised table for `SET_ORDER_PAY`
insert-selects. The JSONB column becomes a cache for the restaurant display only.

**For GetOrdersInBills — project only the fields the frontend actually uses:**

```sql
SELECT
    order_num, daily_count, poster, state, ac_num, receipt_num
    , order_items->>'item_name' -- if only names needed
FROM salesorders WHERE ...
```

Avoid returning the full blob when summary data is sufficient.

---

## 6. DelOrderItem / UpdateOrderItemQty — read-modify-write via Go

**File:** `pkg/sales/orders.go` — `DelOrderItem()`, `UpdateOrderItemQty()`  
**Severity:** Medium  

### What is happening

Both functions follow a three-step pattern: read the full JSONB array from Postgres into
Go, mutate one element in memory, then write the entire array back:

```go
// Step 1 — read
err = ord.FetchtemsCtx(ctx, tx)

// Step 2 — mutate in Go
for i, itm := range ord.OrderItems {
    if itm.ReceiptItem == orderItem {
        ord.OrderItems[i].State = "DELETED"
    }
}

// Step 3 — marshal and write back
jStr, _ := json.Marshal(ord.OrderItems)
db.PgPool.Query(ctx, `UPDATE salesorders SET order_items = $1 WHERE order_num = $2`, jStr, orderNum)
```

This is two network round-trips plus Go-side JSON marshal/unmarshal for what is logically
a single point update. Under concurrent requests on the same order the read-modify-write
also races: two Go routines can read the same state, each delete a different item, and the
second write overwrites the first deletion.

### Suggestion

Perform the update in a single SQL statement using Postgres `jsonb` operators:

**Delete an item:**

```sql
UPDATE salesorders
SET order_items = (
    SELECT jsonb_agg(
        CASE WHEN elem->>'receipt_item' = $1
             THEN elem || '{"state":"DELETED"}'::jsonb
             ELSE elem
        END
    )
    FROM jsonb_array_elements(order_items) AS elem
)
WHERE order_num = $2 AND state = 'pending'
RETURNING order_items::varchar
```

**Update quantity:**

```sql
UPDATE salesorders
SET order_items = (
    SELECT jsonb_agg(
        CASE WHEN elem->>'receipt_item' = $1
             THEN elem
                  || jsonb_build_object('quantity', $3::float)
                  || jsonb_build_object('total',    (elem->>'price')::float * $3::float)
             ELSE elem
        END
    )
    FROM jsonb_array_elements(order_items) AS elem
)
WHERE order_num = $2 AND state = 'pending'
RETURNING order_items::varchar
```

This eliminates one round-trip, removes the Go-side JSON work, and is atomic — no race
between concurrent updates on the same order.

---

## 7. log.Fatalln inside an HTTP handler

**File:** `internal/cash/cash.go:466`  
**Severity:** Critical  

### What is happening

```go
cart, err := item.AddCart(r.Context())
if err != nil {
    log.Fatalln("error. failed to add item to cart     err =", err)
    ...
}
```

`log.Fatal` calls `os.Exit(1)`. A single bad add-to-cart request — malformed JSON body,
a transient DB hiccup, a network blip to the inventory service — kills the entire server
process and takes down every active till simultaneously. There are also commented-out
`log.Fatalln` statements on lines 65 and 135 that indicate this is a recurring pattern.

### Suggestion

Replace with the same error-response pattern used everywhere else in the handler:

```go
cart, err := item.AddCart(r.Context())
if err != nil {
    log.Println("error. failed to add item to cart     err =", err)
    respMap["response"] = "error"
    respMap["message"] = "failed adding to cart"
    respMap["trace"] = err
    return respMap
}
```

Audit the entire `internal/` tree for any remaining `log.Fatal` / `log.Fatalln` /
`log.Fatalf` calls and apply the same fix. The only acceptable use of `log.Fatal` in a
server is at startup (before any request is served), such as during DB connection
initialisation or TLS certificate loading.

---

## 8. AddToOrder — wrong nil-check, OrderNum never validated

**File:** `pkg/sales/orders.go:530-534`  
**Severity:** Medium  

### What is happening

The guard at the top of `AddToOrder` checks `ReceiptNum` twice and never checks
`OrderNum`:

```go
if ord.ReceiptNum == 0 {
    return nil, 0, fmt.Errorf("error. Order->AddToOrder()    null order num")
}
if ord.ReceiptNum == 0 {   // ← copy-paste error: same field checked twice
    return nil, 0, fmt.Errorf("error. Order->AddToOrder()    null receipt")
}
```

When `OrderNum` is zero, `NewOrderTX` runs, generates a new order, and the code proceeds
correctly by design — but only because `NewOrderTX` handles the `OrderNum == 0` case
internally. The bug is the misleading error message and the missing explicit guard: if the
caller ever passes a deliberately zero `OrderNum` expecting to reuse an existing order,
the function silently creates a new one instead.

### Suggestion

```go
if ord.ReceiptNum == 0 {
    return nil, 0, fmt.Errorf("AddToOrder: receipt_num is required")
}
// remove the duplicate check; document the OrderNum == 0 intent explicitly
// NewOrderTX handles order creation when OrderNum is 0
```

If the zero-means-create behaviour is intentional, document it:

```go
// OrderNum == 0 signals that a new order should be created within this receipt.
// NewOrderTX will generate the order number.
```

---

## 9. 70+ fmt.Print calls in request hot paths

**Files:** `pkg/sales/receipts.go` (35 statements), `pkg/sales/orders.go` (39 statements)  
**Severity:** Low–Medium  

### What is happening

Every request through `GenReceipt`, `Fetch`, `AddCart`, `GetOrdersInBills`, and
`CompleteOrder` synchronously writes lines to stdout:

```go
// receipts.go
fmt.Println("Gen Receipt for till num =", arg.TillNum)
fmt.Printf("GenReceipt took %v", time.Since(start))
fmt.Printf("created receipt = %v", arg.ReceiptNum)

// orders.go
fmt.Println("\n\t\t ac_num =", ord.AcNum)
fmt.Println("\t\t poster =", ord.Poster)
fmt.Println("\t\t receipt =", ord.ReceiptNum)
fmt.Printf("\t kafka broker    addr = 'tcp://\t%v'\n", os.Getenv("KAFKA_BROKER"))
```

Under Go's default stdout writer, each `fmt.Print*` call acquires a mutex. Under 10+
concurrent requests these contention points accumulate. More practically, they pollute
production logs with debug noise, making it harder to spot real errors with log-monitoring
tools.

### Suggestion

Introduce a levelled logger. The standard library `log/slog` (Go 1.21+) is zero-cost at
disabled levels:

```go
// main.go / pos.go startup
import "log/slog"

var logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,   // set to Debug in development
}))
```

```go
// replace fmt.Println("Gen Receipt for till num =", arg.TillNum)
slog.Debug("GenReceipt", "till_num", arg.TillNum)

// replace defer fmt.Printf("GenReceipt took %v", time.Since(start))
defer func() { slog.Debug("GenReceipt complete", "elapsed", time.Since(start)) }()
```

With `Level: slog.LevelInfo` in production, all `slog.Debug` calls are eliminated at
zero CPU cost — no string formatting, no stdout write, no mutex.

---

## 10. Kafka.NewConn calls log.Fatalf on broker failure

**File:** `pkg/broker/kafka.go:31-41`  
**Severity:** Medium  

### What is happening

```go
func (b *Kafka) NewConn(ctx context.Context) error {
    conn, err := kafka.DialLeader(ctx, "tcp", b.Broker, b.Topic, 0)
    if err != nil {
        log.Fatalf("failed to connect: %v", err)   // ← kills the process
        return err
    }
    defer conn.Close()
    ...
}
```

If Kafka is temporarily unavailable during a reconnect attempt (rolling restart, network
partition), the entire POS server process exits. `NewConn` is also dead code — `Produce`
creates its own `kafka.Writer` directly and never calls `NewConn`.

### Suggestion

**Remove `NewConn` or fix the fatal:**

```go
func (b *Kafka) NewConn(ctx context.Context) error {
    conn, err := kafka.DialLeader(ctx, "tcp", b.Broker, b.Topic, 0)
    if err != nil {
        return fmt.Errorf("kafka connect failed: %w", err)   // caller decides
    }
    defer conn.Close()
    return nil
}
```

Since `Produce` does not use `NewConn`, the function can be deleted entirely to reduce
confusion. The `kafka.Writer` used in `Produce` already handles reconnection internally.

---

## Summary

| # | Location | Category | Severity |
|---|---|---|---|
| 1 | `AddToOrder` | Shared context across network call + DB tx | **High** |
| 2 | `Kafka.Produce` | New writer per order, synchronous in handler | **High** |
| 3 | `CashInTill` | 5-join aggregate on every cart load | **High** |
| 4 | `CreateReceipt` | `MAX` full scan + concurrent receipt race | **High** |
| 5 | `GetOrdersInBills` / `SetOrderPay` | JSONB expansion, no index on `receipt_num, state` | Medium |
| 6 | `DelOrderItem` / `UpdateOrderItemQty` | 2-round-trip read-modify-write | Medium |
| 7 | `cash.go:466` | `log.Fatalln` kills server on any cart error | **Critical** |
| 8 | `AddToOrder:530` | Duplicate nil-check, `OrderNum` never validated | Medium |
| 9 | `receipts.go`, `orders.go` | 70+ `fmt.Print` in hot paths | Low–Medium |
| 10 | `kafka.go:34` | `log.Fatalf` in dead `NewConn` function | Medium |

**Recommended priority order:**  
Fix **#7** first (crash risk), then **#4** (data correctness), then **#1 + #2 + #3**
(the three highest-throughput bottlenecks), then address the remaining items in a
subsequent pass.
