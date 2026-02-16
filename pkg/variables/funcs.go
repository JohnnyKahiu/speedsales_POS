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

// FetchDefaults
func FetchDefaults() (SysSettings, error) {
	var settings SysSettings
	sql := `SELECT label, params FROM sys_conf WHERE params IS NOT NULL`
	rows, err := database.PgPool.Query(context.Background(), sql)
	if err != nil {
		return settings, err
	}

	var settingS []string

	for rows.Next() {
		var label string
		var setting []byte

		rows.Scan(&label, &setting)

		settingStr := fmt.Sprintf("\"%v\": %v", label, string(setting))

		settingS = append(settingS, settingStr)
	}

	settingStr := "{"
	for i, row := range settingS {
		if i < (len(settingS) - 1) {
			settingStr += row + ", "
		} else {
			settingStr += row
		}
	}
	settingStr += "}"

	err = json.Unmarshal([]byte(settingStr), &settings)
	if err != nil {
		fmt.Println("error ", err)
	}

	return settings, nil
}
