package main

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"

	_ "github.com/jinzhu/gorm/dialects/postgres"
)

func main() {

	r := mux.NewRouter()
	r.HandleFunc("/patients", GetAllPatients2).Methods("GET")
	log.Fatal(http.ListenAndServe(":8080", r))

}

func GetAllPatients2(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello, World!"))

}
