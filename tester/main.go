package main

import (
	"log"
	"net/http"

	"tester/internal/tester"
)

func main() {
	tester.RegisterAPI()

	log.Println("[tester] api on :9381")
	log.Fatal(http.ListenAndServe(":9381", nil))
}
