package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"

	webs "github.com/FidelityInternational/possum/web_server"
)

func main() {
	server, err := webs.CreateServer(dbConn, webs.CreateController)
	if err != nil {
		panic(fmt.Sprintf("Error creating server [%s]...", err.Error()))
	}

	router := server.Start()

	http.Handle("/", router)

	err = http.ListenAndServe(":"+os.Getenv("PORT"), nil)
	if err != nil {
		fmt.Println("ListenAndServe:", err)
	}
}

func dbConn(driverName string, connectionString string) (*sql.DB, error) {
	db, err := sql.Open(driverName, connectionString)
	return db, err
}
