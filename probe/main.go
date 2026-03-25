package main

import (
	"database/sql"
	"log"
	"net/http"

	_ "github.com/mattn/go-sqlite3"

	"probe/internal/probe"
)

func main() {
	db, err := sql.Open("sqlite3", "cameras.db?mode=ro")
	if err != nil {
		log.Fatal(err)
	}

	ports := loadPorts(db)
	probe.RegisterAPI(db, ports)

	log.Println("[probe] api on :9382")
	log.Fatal(http.ListenAndServe(":9382", nil))
}

func loadPorts(db *sql.DB) []int {
	rows, err := db.Query("SELECT DISTINCT port FROM streams WHERE port > 0 UNION SELECT DISTINCT port FROM preset_streams WHERE port > 0")
	if err != nil {
		log.Printf("[probe] failed to load ports from db: %v, using defaults", err)
		return []int{554, 80, 8080, 443, 8554, 5544, 10554, 1935, 81, 88, 8090, 8001, 8081, 7070, 7447, 34567}
	}
	defer rows.Close()

	var ports []int
	for rows.Next() {
		var port int
		if err = rows.Scan(&port); err == nil {
			ports = append(ports, port)
		}
	}

	if len(ports) == 0 {
		return []int{554, 80, 8080, 443, 8554, 5544, 10554, 1935, 81, 88, 8090, 8001, 8081, 7070, 7447, 34567}
	}

	log.Printf("[probe] loaded %d ports from database", len(ports))
	return ports
}
