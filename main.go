package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"

	webs "github.com/FidelityInternational/possum/web_server"
)

func main() {
	if os.Getenv("DEBUG") == "true" {
		log.SetLevel(log.DebugLevel)
	}
	server, err := webs.CreateServer(dbConn, webs.CreateController)
	if err != nil {
		log.WithFields(log.Fields{"package": "main", "function": "main"}).Fatalf("Error creating server [%s]", err.Error())
	}

	router := server.Start()
	http.Handle("/", router)
	port := os.Getenv("PORT")
	if port == "" {
		log.WithFields(log.Fields{"package": "main", "function": "main"}).Fatal("PORT not set. Exiting.")
	}
	log.WithFields(log.Fields{"package": "main", "function": "main"}).Infof("Listening on port: %s", port)
	err = http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	if err != nil {
		log.WithFields(log.Fields{"package": "main", "function": "main"}).Fatal(err)
	}

}

func dbConn(driverName string, connectionString string) (*sql.DB, error) {
	db, err := sql.Open(driverName, connectionString)
	return db, err
}
