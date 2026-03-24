package main

import (
	"database/sql"
	"log"
	"net/http"

	_ "github.com/mattn/go-sqlite3"

	"search/pkg/camdb"
)

func main() {
	db, err := sql.Open("sqlite3", "cameras.db?mode=ro")
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/api/search", camdb.SearchHandler(db))
	http.HandleFunc("/api/streams", camdb.StreamsHandler(db))

	log.Println("[search] api on :9380")
	log.Fatal(http.ListenAndServe(":9380", nil))
}
