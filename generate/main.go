package main

import (
	"log"
	"net/http"

	"generate/internal/generate"
)

func main() {
	generate.RegisterAPI()

	log.Println("[generate] api on :9383")
	log.Fatal(http.ListenAndServe(":9383", nil))
}
