package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/variables"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/segmentio/kafka-go"
)

type DBPool interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type MobileMoney struct {
	table        string    `name:"mobile_money" type:"table"`
	ID           uuid.UUID `json:"id" type:"field" sql:"VARCHAR NOT NULL"`
	TxnDate      time.Time `json:"txn_date" type:"field" sql:"TIMESTAMPTZ NOT NULL DEFAULT now()"`
	MerchantTime time.Time `json:"merchant_time" type:"field" sql:"TIMESTAMPTZ NOT NULL DEFAULT now()"`
	RequestMode  string    `json:"request_mode" type:"field" sql:"VARCHAR(20) NOT NULL"`
	Code         string    `json:"code" type:"field" sql:"VARCHAR(50)"`
	Telephone    string    `json:"telephone" type:"field" sql:"VARCHAR NOT NULL"`
	Amount       float64   `json:"amount" type:"field" sql:"FLOAT NOT NULL DEFAULT '0'"`
	TraceNum     int64     `json:"trace_num" type:"field" sql:"BIGINT NOT NULL DEFAULT '0'"`
	ConfTime     time.Time `json:"conf_time" type:"field" sql:"TIMESTAMPTZ NOT NULL DEFAULT NOW()"`
	Activation   string    `json:"activation" type:"field" sql:"VARCHAR NOT NULL DEFAULT 'normal'"`
	Status       string    `json:"status" type:"field" sql:"VARCHAR NOT NULL DEFAULT 'pending'"`
	Claimed      bool      `json:"claimed" type:"field" sql:"BOOL NOT NULL DEFAULT 'false'"`
	uniqueConst  string    `name:"mm_unique" type:"constraint" sql:"UNIQUE(code)"`
	pkey         string    `name:"mobile_pk" type:"constraint" sql:"PRIMARY KEY(id)"`
}

func GenMpesaTbl() error {
	return database.CreateFromStruct(MobileMoney{})
}

type mpesaMessage struct {
	ID                 uuid.UUID `json:"id"`
	MpesaReceiptNumber string    `json:"mpesa_receipt_number"`
	Phone              string    `json:"phone"`
	Amount             float64   `json:"amount"`
	UpdatedAt          time.Time `json:"updated_at"`
	Status             string    `json:"status"`
}

// ConsumeMpesa reads from the Kafka Mpesa topic and updates the pending mobile_money record.
// Runs until ctx is cancelled.
func ConsumeMpesa(ctx context.Context, broker, topic, groupID string) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{broker},
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer reader.Close()

	log.Printf("mpesa kafka consumer started  broker=%s topic=%s", broker, topic)

	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("mpesa consumer read error: %v", err)
			continue
		}
		fmt.Printf("\n\t mpesa message = %s \n\n", msg.Value)

		var m mpesaMessage
		if err := json.Unmarshal(msg.Value, &m); err != nil {
			log.Printf("mpesa consumer unmarshal error: %v  raw=%s", err, msg.Value)
			continue
		}

		if err := m.updateCallback(ctx); err != nil {
			log.Printf("mpesa consumer update error: %v  receipt=%s", err, m.MpesaReceiptNumber)
		}

		if err := reader.CommitMessages(ctx, msg); err != nil {
			log.Printf("commit error: %v", err)
		}
	}
}

// AddPending generates a new UUID, inserts a pending mobile_money row, and assigns the
// generated ID back to arg.ID so the caller can forward it to PayGateway.
func (arg *MobileMoney) AddPending(ctxt context.Context, db DBPool) error {
	ctx, cancel := context.WithTimeout(ctxt, 15*time.Second)
	defer cancel()

	arg.ID = uuid.New()

	_, err := db.Exec(ctx,
		`INSERT INTO mobile_money(id, code, telephone, amount, request_mode) VALUES($1, $2, $3, $4, $5)`,
		arg.ID, arg.ID.String(), arg.Telephone, arg.Amount, arg.RequestMode,
	)
	if err != nil {
		log.Println("mobile_money AddPending error     err =", err)
	}
	return err
}

// updateCallback stamps the real mpesa_receipt_number, merchant_time, and status on the pending row.
func (m *mpesaMessage) updateCallback(ctxt context.Context) error {
	ctx, cancel := context.WithTimeout(ctxt, 15*time.Second)
	defer cancel()

	_, err := database.PgPool.Exec(ctx,
		`UPDATE mobile_money SET code=$1, merchant_time=$2, status=$3 WHERE id=$4`,
		m.MpesaReceiptNumber, m.UpdatedAt, m.Status, m.ID,
	)
	if err != nil {
		log.Println("mobile_money updateCallback error     err =", err)
	}
	return err
}

// Add creates a new mpeasa transaction
// Inserts into mobile_money
// returns an error if it fails
func (arg *MobileMoney) Add(ctxt context.Context) error {
	ctx, cancel := context.WithTimeout(ctxt, 15*time.Second)
	defer cancel()

	sql := `INSERT INTO mobile_money(id, merchant_time, code, telephone, amount)
			VALUES($1, $2, $3, $4, $5)
			ON CONFLICT DO NOTHING`
	if _, err := database.PgPool.Exec(ctx, sql, &arg.ID, &arg.MerchantTime, &arg.Code, &arg.Telephone, &arg.Amount); err != nil {
		log.Println("mobileMoney postgresql error.  failed inserting  mobile money txn     err =", err)
		return err
	}

	return nil
}

// Fetch - gets details of a mobile money txn
// queries mobile money and populates struct
// returns an error if it fails
func (arg *MobileMoney) Fetch(ctxt context.Context) error {
	sql := `SELECT 
				txn_date, merchant_time, request_mode, code, telephone, amount, trace_num, conf_time, activation, status, claimed 
			FROM mobile_money WHERE ID = $1`

	ctx, cancel := context.WithTimeout(ctxt, 15*time.Second)
	defer cancel()

	if err := database.PgPool.QueryRow(ctx, sql, arg.ID).Scan(&arg.TxnDate, &arg.MerchantTime, &arg.RequestMode, &arg.Code, &arg.Telephone, &arg.Amount, &arg.TraceNum, &arg.ConfTime, &arg.Activation, &arg.Status, &arg.Claimed); err != nil {
		log.Println("postgresql error.  mobile_money query      err =", err)
		return err
	}

	return nil
}

// FetchActiveMpesa - queries all active mpesa txns
// returns a slice of MobileMoney and an error if it fails
func FetchActiveMpesa(ctxt context.Context) ([]MobileMoney, error) {
	sysSetts, _ := variables.FetchDefaults()

	sql := fmt.Sprintf(`
		SELECT 
			txn_date, merchant_time, request_mode, coalesce(code, ''), telephone, amount, trace_num, conf_time, activation, status, claimed 
		FROM mobile_money WHERE status = 'success' AND claimed = false AND (txn_date >=  (SELECT now()- INTERVAL '%v MINUTES') OR  activation = 'reactivated')
		`, sysSetts.PosDefaults.MpesaExpiry)

	fmt.Println(sql)

	ctx, cancel := context.WithTimeout(ctxt, 15*time.Second)
	defer cancel()

	rows, err := database.PgPool.Query(ctx, sql)
	if err != nil {
		return []MobileMoney{}, err
	}
	defer rows.Close()

	vals := []MobileMoney{}
	for rows.Next() {
		r := MobileMoney{}
		err = rows.Scan(&r.TxnDate, &r.MerchantTime, &r.RequestMode, &r.Code, &r.Telephone, &r.Amount, &r.TraceNum, &r.ConfTime, &r.Activation, &r.Status, &r.Claimed)
		if err != nil {
			return vals, err
		}

		vals = append(vals, r)
	}

	return vals, nil
}

// FetchByCode looks up a single mobile_money record by M-Pesa receipt code.
func FetchByCode(ctxt context.Context, code string) (*MobileMoney, error) {
	ctx, cancel := context.WithTimeout(ctxt, 15*time.Second)
	defer cancel()

	var m MobileMoney
	err := database.PgPool.QueryRow(ctx,
		`SELECT id, code, telephone, amount, request_mode, status
		 FROM mobile_money WHERE code = $1 LIMIT 1`,
		code,
	).Scan(&m.ID, &m.Code, &m.Telephone, &m.Amount, &m.RequestMode, &m.Status)
	if err != nil {
		return nil, err
	}
	return &m, nil
}
