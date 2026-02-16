package database

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"
)

func CreateFromStruct(tblStruct any) error {
	sqlBody := ``
	sqlHead := ``

	// var tblStruct Property

	tblName := ""

	val := reflect.ValueOf(tblStruct)
	for i := 0; i < val.Type().NumField(); i++ {
		if val.Type().Field(i).Tag.Get("type") == "field" && i < (val.Type().NumField()-1) {
			fieldName := val.Type().Field(i).Tag.Get("json") + ""
			sqlDef := val.Type().Field(i).Tag.Get("sql") + ""
			if sqlBody == "" {
				sqlBody += fieldName + " " + sqlDef + "\n\t"
			} else {
				sqlBody += ", " + fieldName + " " + sqlDef + "\n\t"
			}
		} else if val.Type().Field(i).Tag.Get("type") == "table" {
			tblName = val.Type().Field(i).Tag.Get("name")
			fmt.Printf("\n\t %v \n", tblName)
		} else if val.Type().Field(i).Tag.Get("type") == "constraint" {
			fieldName := val.Type().Field(i).Tag.Get("name") + ""
			sqlDef := val.Type().Field(i).Tag.Get("sql") + ""

			sqlBody += ", CONSTRAINT " + fieldName + " " + sqlDef + "\n\t"
		}
	}

	sqlHead = fmt.Sprintf("CREATE TABLE IF NOT EXISTS %v ( ", tblName)
	sql := sqlHead + sqlBody + ");"

	// run sql transaction
	_, err := PgPool.Exec(context.Background(), sql)
	if err != nil {
		fmt.Println("sql =", sql)
		log.Printf("\n error creating '%v' table\n \t%v", tblName, err.Error())
		return err
	}

	// Add non existing columns
	for i := 0; i < val.Type().NumField(); i++ {
		if val.Type().Field(i).Tag.Get("type") == "field" {
			fieldName := val.Type().Field(i).Tag.Get("json") + ""
			sqlDef := val.Type().Field(i).Tag.Get("sql") + " ;"

			if !strings.Contains(sqlDef, "PRIMARY KEY") && fieldName != "" {
				sqlAlter := fmt.Sprintf("ALTER TABLE IF EXISTS %v ADD IF NOT EXISTS %v %v", tblName, fieldName, sqlDef)
				// fmt.Println("\t", sqlAlter)
				_, err := PgPool.Exec(context.Background(), sqlAlter)
				if err != nil {
					fmt.Printf("\nerror Altering %v table\n \t%v", tblName, err.Error())
					// return err
				}
			}
		}

		/*if val.Type().Field(i).Tag.Get("type") == "constraint" {
			fieldName := val.Type().Field(i).Tag.Get("name") + ""
			sqlDef := val.Type().Field(i).Tag.Get("sql") + " ;"
			sqlConst := fmt.Sprintf("ALTER TABLE IF EXISTS %v ADD IF NOT EXISTS CONSTRAINT %v %v", tblName, fieldName, sqlDef)

			_, err := PgCon.Exec(sqlConst)
			if err != nil {
				fmt.Printf("\nerror Altering %v table\n \t%v", tblName, err.Error())
				// return err
			}
		}*/

	}

	return nil
}
