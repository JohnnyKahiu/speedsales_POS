package variables

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/JohnnyKahiu/speedsales/poserver/database"
	"github.com/go-redis/redis"
)

// SysDefaults returns a json string of system defaults
func SysDefaults() (SysSettings, error) {
	if Cache {
		var settings SysSettings
		rows, err := RdbCon.Get("sys_defaults").Result()
		if err == redis.Nil {
			return settings, nil
		} else if err != nil {
			return settings, err
		}

		// unmarshal all the cart data to json array
		json.Unmarshal([]byte(rows), &settings)

		return settings, err
	}

	settings, err := FetchDefaults()
	if err != nil {
		return settings, err
	}

	return settings, nil
}

// UpdatePosSettings persists the PosSettings fields to the settings table.
func UpdatePosSettings(s PosSettings) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if _, err := database.PgPool.Exec(ctx, `UPDATE settings SET pos_defaults = $1`, string(b)); err != nil {
		log.Println("postgresql error.  failed to update settings     err =", err)
		return err
	}
	return nil
}

// UpdateDocFooter persists the DocFooter fields to the settings table.
func UpdateDocFooter(s DocFooter) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if _, err := database.PgPool.Exec(ctx, `UPDATE settings SET doc_footer = $1`, string(b)); err != nil {
		log.Println("postgresql error.  failed to update settings     err =", err)
		return err
	}
	return nil
}

// UpdateDocHeader persists the DocFooter fields to the settings table.
func UpdateDocHeader(s DocHead) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if _, err := database.PgPool.Exec(ctx, `UPDATE settings SET doc_heading = $1`, string(b)); err != nil {
		log.Println("postgresql error.  failed to update settings     err =", err)
		return err
	}
	return nil
}

// UpdateVatCodes persists the vat codes map to the settings table.
func UpdateVatCodes(codes map[string]float32) error {
	b, err := json.Marshal(codes)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if _, err := database.PgPool.Exec(ctx, `UPDATE settings SET vat_codes = $1`, string(b)); err != nil {
		log.Println("postgresql error.  failed to update vat_codes     err =", err)
		return err
	}
	return nil
}

// FetchDefaults
func FetchDefaults() (SysSettings, error) {
	var settings SysSettings
	sql := `SELECT 
				pos_defaults, doc_heading, vat_codes::text, doc_footer
			FROM settings`

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	rows, err := database.PgPool.Query(ctx, sql)
	if err != nil {
		fmt.Println("error querying system settings    err =", err)
		return settings, err
	}

	for rows.Next() {
		vats := ""
		rows.Scan(&settings.PosDefaults, &settings.DocHead, &vats, &settings.DocFooter)

		err := json.Unmarshal([]byte(vats), &settings.VatCodes)
		if err != nil {
			return settings, err
		}
	}

	return settings, nil
}
