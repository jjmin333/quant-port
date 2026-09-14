package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	dsn := os.Getenv("DSN")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	var sess, glob, now, utc string
	err = db.QueryRow("SELECT @@session.time_zone, @@global.time_zone, NOW(), UTC_TIMESTAMP()").
		Scan(&sess, &glob, &now, &utc)
	if err != nil {
		fmt.Println("QUERY FAILED:", err)
		os.Exit(1)
	}
	fmt.Printf("session=%s global=%s NOW=%s UTC=%s\n", sess, glob, now, utc)
}
