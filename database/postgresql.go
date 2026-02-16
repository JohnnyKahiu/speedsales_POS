package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgCon holds database connection
// var PgCon *sql.DB

var PgPool *pgxpool.Pool

type DBConf struct {
	Server   string `json:"server"`
	Port     string `json:"port"`
	DbName   string `json:"database"`
	user     string
	password string
}

/*func (arg DBConf) NewPgCon() error {
	var err error

	arg.Server = os.Getenv("DB_HOST")
	arg.user = os.Getenv("DB_USER")
	arg.password = os.Getenv("DB_PASSWORD")
	arg.DbName = os.Getenv("DB_NAME")
	port := os.Getenv("DB_PORT")

	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s "+"password=%s dbname=%s sslmode=disable", arg.Server, port, arg.user, arg.password, arg.DbName)

	PgCon, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Println("error. DBConf->PgConnect()    err =", err)
		return err
	}
	log.Println("\t Database Connection opened Successfully")

	PgCon.SetMaxOpenConns(20)
	PgCon.SetMaxIdleConns(5)
	return nil
}*/

func (arg DBConf) NewPgPool() (*pgxpool.Pool, error) {
	arg.Server = os.Getenv("DB_HOST")
	arg.user = os.Getenv("DB_USER")
	arg.password = os.Getenv("DB_PASSWORD")
	arg.DbName = os.Getenv("DB_NAME")
	port := os.Getenv("DB_PORT")

	pgxConnString := fmt.Sprintf("postgres://%s:%s@%s:%v/%s", arg.user, arg.password, arg.Server, port, arg.DbName)
	// fmt.Println("pgxConnString =", pgxConnString)

	// open a connection to the database
	pgConf, err := pgxpool.ParseConfig(pgxConnString)
	if err != nil {
		log.Println("error.  pgxpool.ParseConfig()    err =", err)
		return nil, err
	}

	// Configure pool settings
	pgConf.MaxConns = 10                                                     // maximum number of connections
	pgConf.MinConns = 2                                                      // minimum number of connections to maintain
	pgConf.MaxConnLifetime = time.Hour                                       // maximum lifetime of a connection
	pgConf.MaxConnIdleTime = time.Minute * 15                                // maximum idle time of a connection
	pgConf.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol // disable prepared statement cache

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(context.Background(), pgConf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create connection pool: %v\n", err)
		return nil, err
	}
	return pool, nil
}
