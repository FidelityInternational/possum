package webServer

import (
	"database/sql"

	"github.com/FidelityInternational/possum/utils"
	"github.com/gorilla/mux"
	// sql driver
	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
)

var (
	db                 *sql.DB
	err                error
	dbConnectionString string
)

// Server struct
type Server struct {
	Controller *Controller
}

// DBConn - database opening interface
type DBConn func(driverName string, connectionString string) (*sql.DB, error)

// ControllerCreator - controller creation function
type ControllerCreator func(db *sql.DB) *Controller

// CreateServer - creates a server
func CreateServer(dbConnFunc DBConn, controllerCreator ControllerCreator) (*Server, error) {
	dbConnectionString, err = utils.GetDBConnectionDetails()
	if err != nil {
		return nil, err
	}

	db, err = dbConnFunc("mysql", dbConnectionString)
	if err != nil {
		log.WithFields(log.Fields{"package": "webServer", "function": "CreateServer"}).Debugf("Can't open DB connection: %s", err)
		return nil, err
	}

	err = utils.SetupStateDB(db)
	if err != nil {
		log.WithFields(log.Fields{"package": "webServer", "function": "CreateServer"}).Debugf("Can't set up state DB: %s", err)
		return nil, err
	}

	controller := controllerCreator(db)

	return &Server{
		Controller: controller,
	}, nil
}

// Start - starts the web server
func (s *Server) Start() *mux.Router {
	router := mux.NewRouter()

	router.HandleFunc("/v1/state", s.Controller.GetState).Methods("GET")
	router.HandleFunc("/v1/passel_state", s.Controller.GetPasselState).Methods("GET")
	router.HandleFunc("/v1/passel_state_consistency", s.Controller.GetPasselStateConsistency).Methods("GET")
	router.HandleFunc("/v1/state", s.Controller.SetState).Methods("POST")
	router.HandleFunc("/v1/passel_state", s.Controller.SetPasselState).Methods("POST")

	return router
}
