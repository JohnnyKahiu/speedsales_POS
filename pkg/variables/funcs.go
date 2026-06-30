package variables

import (
	"context"
	"encoding/json"
	"fmt"

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
	_, err = database.PgPool.Exec(context.Background(),
		`UPDATE settings SET pos_defaults = $1`, string(b))
	return err
}

// FetchDefaults
func FetchDefaults() (SysSettings, error) {
	var settings SysSettings
	sql := `SELECT 
				pos_defaults, doc_heading, vat_codes::text 
			FROM settings`
	rows, err := database.PgPool.Query(context.Background(), sql)
	if err != nil {
		fmt.Println("error querying system settings    err =", err)
		return settings, err
	}

	for rows.Next() {
		vats := ""
		rows.Scan(&settings.PosDefaults, &settings.DocHead, &vats)

		err := json.Unmarshal([]byte(vats), &settings.VatCodes)
		if err != nil {
			return settings, err
		}
	}

	return settings, nil
}
