package utils

import (
	"database/sql"
	"fmt"
	"reflect"

	"github.com/cloudfoundry-community/go-cfenv"
	log "github.com/sirupsen/logrus"
)

// GetDBConnectionDetails - Loads database connection details from UPS "possum-db"
func GetDBConnectionDetails() (string, error) {
	appEnv, err := cfenv.Current()
	if err != nil {
		log.WithFields(log.Fields{"package": "utils", "function": "GetDBConnectionDetails"}).Debugf("Can't get DB details from CF env: %s", err)
		return "", fmt.Errorf("Can't get DB details from CF env. Check DB binding: %s", err)
	}

	service, err := appEnv.Services.WithName("possum-db")
	if err != nil {
		return "", fmt.Errorf("Can't get DB details from CF env. Check DB binding: %s", err)
	}

	hostname := service.Credentials["host"]
	if nil == hostname {
		hostname = service.Credentials["hostname"]
	}

	database := service.Credentials["database"]
	if nil == database {
		database = service.Credentials["name"]
	}

	dbConnString := fmt.Sprintf("%s:%s@tcp(%s:%v)/%s",
		service.Credentials["username"], service.Credentials["password"], hostname,
		service.Credentials["port"], database)

	return dbConnString, nil
}

// GetMyApplicationURIs - fetched application URIs from VCAP Application
func GetMyApplicationURIs() ([]string, error) {
	appEnv, err := cfenv.Current()
	if err != nil {
		return []string{}, err
	}

	applicationURIs := appEnv.ApplicationURIs
	return applicationURIs, nil
}

// SetupStateDB - creates the state DB if it does not exist
func SetupStateDB(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS state
	(
		possum varchar(255),
		state varchar(255),
		PRIMARY KEY(possum)
	)`)

	if err != nil {
		log.WithFields(log.Fields{"package": "utils", "function": "SetupStateDB"}).Debugf("Can't create table: %s", err)
		return err
	}

	passel, err := GetPassel()
	if err != nil {
		return err
	}

	for _, possum := range passel {
		var possumdb, state string
		log.WithFields(log.Fields{"package": "utils", "function": "SetupStateDB", "possum": possum}).Debugf("Checking initail possum state")
		row := db.QueryRow("SELECT * FROM state WHERE possum=?", possum)
		err := row.Scan(&possumdb, &state)
		if err != nil {
			log.WithFields(log.Fields{"package": "utils", "function": "SetupStateDB"}).Debugf("Error getting row from DB %s", err.Error())
			if err.Error() == "sql: no rows in result set" {
				log.WithFields(log.Fields{"package": "utils", "function": "SetupStateDB", "possum": possum}).Debugf("Inserting alive state into DB")
				_, insertErr := db.Exec("INSERT INTO state VALUES (?, ?)", possum, "alive")
				if insertErr != nil {
					log.WithFields(log.Fields{"package": "utils", "function": "SetupStateDB"}).Debugf("Error inserting into DB %s", insertErr.Error())
					return insertErr
				}
			} else {
				log.WithFields(log.Fields{"package": "utils", "function": "SetupStateDB"}).Debug(err.Error())
				return err
			}
		} else {
			log.WithFields(log.Fields{"package": "utils", "function": "SetupStateDB", "possum": possum, "state": state}).Debugf("Retrived state")
		}
	}
	return nil
}

// GetPassel - Returns the passel of possums
func GetPassel() ([]string, error) {
	var possums []string

	appEnv, err := cfenv.Current()
	if err != nil {
		log.WithFields(log.Fields{"package": "utils", "function": "GetPassel"}).Debugf("Can't get CF env variables: %s", err)
		return []string{}, err
	}

	service, err := appEnv.Services.WithName("possum")
	if err != nil {
		log.WithFields(log.Fields{"package": "utils", "function": "GetPassel"}).Debugf("Can't get service with a name \"possum\": %s", err)
		return []string{}, err
	}

	passel := service.Credentials["passel"]

	for _, possum := range passel.([]interface{}) {
		if reflect.TypeOf(possum) != reflect.TypeOf("") {
			return []string{}, fmt.Errorf("possum was not a string")
		}
		possums = append(possums, possum.(string))
	}

	return possums, nil
}

// GetState - returns current state for the given possum
func GetState(db *sql.DB, possum string) (string, error) {
	var possumdb, state string
	row := db.QueryRow("SELECT * FROM state WHERE possum=?", possum)
	err := row.Scan(&possumdb, &state)
	if err != nil {
		log.WithFields(log.Fields{"package": "utils", "function": "GetState", "dbquery": "SELECT * FROM state WHERE possum=" + possum}).Debugf("Can't get rows from DB: %s", err)
		if err.Error() == "sql: no rows in result set" {
			return "", fmt.Errorf("Could not find possum %s in db", possum)
		}
		return "", err
	}
	return state, nil
}

// GetPasselState - returns current state for the given passel
func GetPasselState(db *sql.DB, passel []string) (map[string]string, error) {
	if len(passel) == 0 {
		log.WithFields(log.Fields{"package": "utils", "function": "GetPasselState"}).Debugf("Passel had 0 members")
		return nil, fmt.Errorf("Passel had 0 members")
	}
	passelState := make(map[string]string)
	for _, possum := range passel {
		var possumdb, state string
		row := db.QueryRow("SELECT * FROM state WHERE possum=?", possum)
		err := row.Scan(&possumdb, &state)
		if err != nil {
			log.WithFields(log.Fields{"package": "utils", "function": "GetPasselState", "dbquery": "SELECT * FROM state WHERE possum=" + possum}).Debugf("Can't get rows from DB: %s", err)
			if err.Error() == "sql: no rows in result set" {
				return nil, fmt.Errorf("Could not find possum %s in db", possum)
			}
			return nil, err
		}
		passelState[possum] = state
	}
	return passelState, nil
}

// WriteState - Updates the state of a possum
func WriteState(db *sql.DB, desiredPossum string, desiredState string) error {
	if desiredState != "alive" && desiredState != "dead" {
		return fmt.Errorf(`The state should have been "alive" or "dead" not "%s"`, desiredState)
	}
	_, err := db.Exec("UPDATE state SET state=? WHERE possum=?", desiredState, desiredPossum)
	if err != nil {
		log.WithFields(log.Fields{"package": "utils", "function": "WriteState", "dbquery": "UPDATE state SET state=" + desiredState + " WHERE possum=" + desiredPossum}).Debugf("Can't update DB: %s", err)
		return err
	}
	return nil
}

// GetUsername - Returns the basic auth username
func GetUsername() (string, error) {
	appEnv, err := cfenv.Current()
	if err != nil {
		log.WithFields(log.Fields{"package": "utils", "function": "GetUsername"}).Debugf("Can't get CF env variables: %s", err)
		return "", err
	}

	service, err := appEnv.Services.WithName("possum")
	if err != nil {
		log.WithFields(log.Fields{"package": "utils", "function": "GetUsername"}).Debugf("Can't get service with a name \"possum\": %s", err)
		return "", err
	}

	username := service.Credentials["username"]

	if username == nil {
		log.WithFields(log.Fields{"package": "utils", "function": "GetUsername"}).Debugf("Username empty")
		return "", nil
	}

	return username.(string), nil
}

// GetPassword - Returns the basic auth password
func GetPassword() (string, error) {
	appEnv, err := cfenv.Current()
	if err != nil {
		log.WithFields(log.Fields{"package": "utils", "function": "GetPassword"}).Debugf("Can't get CF env variables: %s", err)
		return "", err
	}

	service, err := appEnv.Services.WithName("possum")
	if err != nil {
		log.WithFields(log.Fields{"package": "utils", "function": "GetPassword"}).Debugf("Can't get service with a name \"possum\": %s", err)
		return "", err
	}

	password := service.Credentials["password"]

	if password == nil {
		log.WithFields(log.Fields{"package": "utils", "function": "GetPassword"}).Debugf("Password empty")
		return "", nil
	}

	return password.(string), nil
}
