package webServer

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/FidelityInternational/possum/utils"
	log "github.com/sirupsen/logrus"
)

const (
	defaultPollingIntervalSeconds = 10
)

// Controller struct
type Controller struct {
	DB         *sql.DB
	HTTPClient *http.Client
}

// PossumStates struct
type PossumStates struct {
	PossumStates map[string]string `json:"possum_states"`
	Error        string            `json:"error"`
	Force        bool              `json:"force"`
}

// CreateController - returns a populated controller object
func CreateController(db *sql.DB) *Controller {
	return &Controller{
		DB:         db,
		HTTPClient: createHTTPClient(),
	}
}

// GetState - Get state of a single possum
func (c *Controller) GetState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", loadCORSAllowed())
	myURIs, err := utils.GetMyApplicationURIs()
	if standardError(err, w) {
		log.WithFields(log.Fields{"package": "webServer", "function": "GetState"}).Debugf("Can't get application URIs: %s", err.Error())
		return
	}
	if len(myURIs) == 0 {
		customError(w, http.StatusGone, "No uris were configured")
		log.WithFields(log.Fields{"package": "webServer", "function": "GetState"}).Debugf("No uris were configured: %v", myURIs)
		return
	}
	passel, err := utils.GetPassel()
	if standardError(err, w) {
		log.WithFields(log.Fields{"package": "webServer", "function": "GetState"}).Debugf("Can't get passel: %s", err.Error())
		return
	}
	if len(passel) == 0 {
		log.WithFields(log.Fields{"package": "webServer", "function": "GetState"}).Debugln("Passel had 0 members")
		customError(w, http.StatusGone, "Passel had 0 members")
		return
	}
	for _, uri := range myURIs {
		for _, possum := range passel {
			if uriPossumMatch(uri, possum) {
				state, err := utils.GetState(c.DB, possum)
				if standardError(err, w) {
					log.WithFields(log.Fields{"package": "webServer", "function": "GetState"}).Debugf("%v", err)
					return
				}
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, fmt.Sprintf(`{"state": "%s"}`, state))
				return
			}
		}
	}
	log.WithFields(log.Fields{"package": "webServer", "function": "GetState"}).Debugln("Could not match any possum in db")
	w.WriteHeader(http.StatusGone)
	fmt.Fprintf(w, `{"error": "Could not match any possum in db"}`)
}

// GetPasselState - Get state of the entire passel
func (c *Controller) GetPasselState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", loadCORSAllowed())

	passel, err := utils.GetPassel()
	if standardError(err, w) {
		log.WithFields(log.Fields{"package": "webServer", "function": "GetPassel"}).Debugf("Can't get passel: %s", err.Error())
		return
	}
	possumStates, err := utils.GetPasselState(c.DB, passel)
	if standardError(err, w) {
		log.WithFields(log.Fields{"package": "webServer", "function": "GetPassel"}).Debug(err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
	possumStatesBytes, _ := json.Marshal(possumStates)
	fmt.Fprintf(w, fmt.Sprintf(`{"possum_states": %s}`, string(possumStatesBytes)))
}

// GetPasselStateConsistency - Get the state conistency of the passel
func (c *Controller) GetPasselStateConsistency(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", loadCORSAllowed())

	passel, err := getPassel()
	if standardError(err, w) {
		log.WithFields(log.Fields{"package": "webServer", "function": "GetPasselStateConsistency"}).Debugf("Can't get passel: %s", err.Error())
		return
	}
	passelStates, err := gatherStates(c.HTTPClient, passel)
	if standardError(err, w) {
		log.WithFields(log.Fields{"package": "webServer", "function": "GetPasselStateConsistency"}).Debug(err.Error())
		return
	}
	consistent := arePasselStatesConsistent(passelStates)
	if stateInconsistentError(w, passelStates, consistent, "") {
		return
	}
	passelStatesBytes, _ := json.Marshal(passelStates)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, fmt.Sprintf(`{"consistent": true, "passel_states": %s}`, string(passelStatesBytes)))
}

// SetState - set the possum state
func (c *Controller) SetState(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(w, r) {
		w.WriteHeader(401)
		fmt.Fprintf(w, "401 Unauthorized")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	myURIs, err := utils.GetMyApplicationURIs()
	if standardError(err, w) {
		log.WithFields(log.Fields{"package": "webServer", "function": "SetState"}).Debugf("Can't get application URIs: %s, Request: ", err.Error())
		return
	}
	if len(myURIs) == 0 {
		customError(w, http.StatusGone, "No uris were configured")
		return
	}
	passel, err := utils.GetPassel()
	if standardError(err, w) {
		log.WithFields(log.Fields{"package": "webServer", "function": "SetState"}).Debugf("Can't get passel: %s", err.Error())
		return
	}
	if len(passel) == 0 {
		customError(w, http.StatusGone, "Passel had 0 members")
		return
	}
	for _, uri := range myURIs {
		for _, possum := range passel {
			if uriPossumMatch(uri, possum) {
				desiredPasselState, err := getDesiredPasselState(r)
				if standardError(err, w) {
					log.WithFields(log.Fields{"package": "webServer", "function": "SetState"}).Debug(err.Error())
					return
				}
				desiredPossumFound, desiredPossum := desiredPossumInPassel(desiredPasselState, passel)
				if !desiredPossumFound {
					customError(w, http.StatusInternalServerError, fmt.Sprintf("Possum %s is not part of my passel", desiredPossum))
					return
				}
				passelState, err := getPasselState(c.HTTPClient, possum)
				if standardError(err, w) {
					log.WithFields(log.Fields{"package": "webServer", "function": "SetState"}).Debugf("Can't get passel: %s", err.Error())
					return
				}
				if !isAtLeastOnePossumAlive(desiredPasselState, passelState) {
					customError(w, http.StatusInternalServerError, "Would have killed all possums")
					return
				}
				for desiredPossum, desiredState := range desiredPasselState {
					err = utils.WriteState(c.DB, desiredPossum, desiredState)
					if standardError(err, w) {
						return
					}
				}
				afterWritePasselState, err := getPasselState(c.HTTPClient, possum)
				if standardError(err, w) {
					log.WithFields(log.Fields{"package": "webServer", "function": "SetState"}).Debug(err.Error())
					return
				}
				completeDesiredState := updateStateToDesired(desiredPasselState, afterWritePasselState)
				configuredCorrectly := reflect.DeepEqual(completeDesiredState, afterWritePasselState)
				afterWritePasselStateBytes, _ := json.Marshal(afterWritePasselState)
				if !configuredCorrectly {
					completeDesiredStateBytes, _ := json.Marshal(completeDesiredState)
					customError(w, http.StatusInternalServerError, fmt.Sprintf("State should have been: %s but was %s", string(completeDesiredStateBytes), string(afterWritePasselStateBytes)))
					log.WithFields(log.Fields{"package": "webServer", "function": "SetState"}).Debug("State should have been: %s but was %s", string(completeDesiredStateBytes), string(afterWritePasselStateBytes))
					return
				}
				w.WriteHeader(http.StatusAccepted)
				fmt.Fprintf(w, fmt.Sprintf(`{"possum_states": %s}`, string(afterWritePasselStateBytes)))
				return
			}
		}
	}
	w.WriteHeader(http.StatusGone)
	fmt.Fprintf(w, `{"error": "Could not match any possum in db"}`)
}

// SetPasselState - Set the state of the entire passel
func (c *Controller) SetPasselState(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(w, r) {
		w.WriteHeader(401)
		fmt.Fprintf(w, "401 Unauthorized")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	passel, err := getPassel()
	if standardError(err, w) {
		log.WithFields(log.Fields{"package": "webServer", "function": "SetPasselState"}).Debugf("Can't get passel: %s", err.Error())
		return
	}
	desiredPossumStates, err := getDesiredPossumStates(r)
	if standardError(err, w) {
		log.WithFields(log.Fields{"package": "webServer", "function": "SetPasselState"}).Debug(err.Error())
		return
	}
	desiredPasselState := desiredPossumStates.PossumStates
	passelStates, err := gatherStates(c.HTTPClient, passel)
	if standardError(err, w) {
		log.WithFields(log.Fields{"package": "webServer", "function": "SetPasselState"}).Debug(err.Error())
		return
	}
	if !desiredPossumStates.Force {
		consistent := arePasselStatesConsistent(passelStates)
		if stateInconsistentError(w, passelStates, consistent, "State was inconsistent before update") {
			return
		}
	}
	if !isAtLeastOnePossumAlive(desiredPasselState, passelStates[0]) {
		customError(w, http.StatusInternalServerError, "Would have killed all possums")
		return
	}
	desiredPasselStateBytes, _ := json.Marshal(desiredPasselState)
	afterWritePasselStates, err := setStates(c.HTTPClient, passel, desiredPasselStateBytes)
	if standardError(err, w) {
		log.WithFields(log.Fields{"package": "webServer", "function": "SetPasselState"}).Debug(err.Error())
		return
	}
	afterWriteConsistent := arePasselStatesConsistent(afterWritePasselStates)
	if stateInconsistentError(w, afterWritePasselStates, afterWriteConsistent, "State was inconsistent after update") {
		return
	}
	afterWritePasselStatesBytes, _ := json.Marshal(afterWritePasselStates)
	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(w, fmt.Sprintf(`{"consistent": true, "passel_states": %s}`, string(afterWritePasselStatesBytes)))
}

func standardError(err error, w http.ResponseWriter) bool {
	if err != nil {
		fmt.Println("An error occurred:")
		fmt.Println(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()))
		return true
	}
	return false
}

func customError(w http.ResponseWriter, statusCode int, err string) {
	fmt.Println("An error occurred:")
	fmt.Println(err)
	w.WriteHeader(statusCode)
	fmt.Fprintf(w, fmt.Sprintf(`{"error": "%s"}`, err))
}

func stateInconsistentError(w http.ResponseWriter, passelStates []map[string]string, consistent bool, customError string) bool {
	if !consistent {
		passelStatesBytes, _ := json.Marshal(passelStates)
		passelStatesString := string(passelStatesBytes)
		fmt.Println("An error occurred:")
		w.WriteHeader(http.StatusInternalServerError)
		if customError == "" {
			fmt.Printf("State was inconsistent: \n%s\n", passelStatesString)
			fmt.Fprintf(w, fmt.Sprintf(`{"consistent": false, "error": "State was inconsistent", "passel_states": %s}`, passelStatesString))
			return true
		}
		fmt.Printf("%s: \n%s\n", customError, passelStatesString)
		fmt.Fprintf(w, fmt.Sprintf(`{"consistent": false, "error": "%s", "passel_states": %s}`, customError, passelStatesString))
		return true
	}
	return false
}

func getDesiredPasselState(r *http.Request) (map[string]string, error) {
	var desiredPasselState map[string]string
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.WithFields(log.Fields{"package": "webServer", "function": "getDesiredPasselState"}).Debugf("Couldn't read body of request :%s", err)
		return nil, err
	}
	err = json.Unmarshal(data, &desiredPasselState)
	if err != nil {
		log.WithFields(log.Fields{"package": "webServer", "function": "getDesiredPasselState"}).Debugf("Couldn't unmarshal JSON :%s", err)
		return nil, err
	}
	return desiredPasselState, nil
}

func getDesiredPossumStates(r *http.Request) (PossumStates, error) {
	var desiredPossumStates PossumStates
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.WithFields(log.Fields{"package": "webServer", "function": "getDesiredPossumStates"}).Debugf("Couldn't read body of request :%s", err)
		return PossumStates{}, err
	}
	err = json.Unmarshal(data, &desiredPossumStates)
	if err != nil {
		log.WithFields(log.Fields{"package": "webServer", "function": "getDesiredPossumStates"}).Debugf("Couldn't unmarshal JSON :%s", err)
		return PossumStates{}, err
	}
	return desiredPossumStates, nil
}

func uriPossumMatch(uri string, possum string) bool {
	match, _ := regexp.MatchString(fmt.Sprintf("^http.?://%s$", uri), possum)
	return match
}

func isAtLeastOnePossumAlive(desiredPasselStates map[string]string, passelStates map[string]string) bool {
	for _, state := range updateStateToDesired(desiredPasselStates, passelStates) {
		if state == "alive" {
			return true
		}
	}
	return false
}

func updateStateToDesired(desiredPasselStates map[string]string, passelStates map[string]string) map[string]string {
	newPasselStates := make(map[string]string)
	for possum, state := range passelStates {
		newPasselStates[possum] = state
	}
	for possum, state := range desiredPasselStates {
		newPasselStates[possum] = state
	}
	return newPasselStates
}

func gatherStates(httpClient *http.Client, passel []string) ([]map[string]string, error) {
	var passelStates []map[string]string
	for _, possum := range passel {
		possumStates, err := getPasselState(httpClient, possum)
		if err != nil {
			return nil, err
		}
		passelStates = append(passelStates, possumStates)
	}
	return passelStates, nil
}

func arePasselStatesConsistent(passelStates []map[string]string) bool {
	var count int
	consistent := make(map[bool]struct{})
	for i, passelState := range passelStates {
		count++
		if count < len(passelStates) {
			consistent[reflect.DeepEqual(passelState, passelStates[i+1])] = struct{}{}
		}
	}
	if _, ok := consistent[true]; ok {
		if _, ok := consistent[false]; !ok {
			return true
		}
	}
	return false
}

func getPasselState(httpClient *http.Client, possum string) (map[string]string, error) {
	var possumStates PossumStates
	resp, err := httpClient.Get(fmt.Sprintf("%s/v1/passel_state", possum))
	if err != nil {
		log.WithFields(log.Fields{"package": "webServer", "function": "getPasselState", "URI": fmt.Sprintf("%s/v1/passel_state", possum)}).Debugf("Couldn't complete API request :%s", err)
		return nil, err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(log.Fields{"package": "webServer", "function": "getPasselState"}).Debugf("Couldn't read body of request :%s", err)
		return nil, err
	}
	err = json.Unmarshal(data, &possumStates)
	if err != nil {
		log.WithFields(log.Fields{"package": "webServer", "function": "getPasselState"}).Debugf("Couldn't unmarshal JSON :%s", err)
		return nil, err
	}
	if possumStates.Error != "" {
		log.WithFields(log.Fields{"package": "webServer", "function": "getPasselState"}).Debug(possumStates.Error)
		return nil, fmt.Errorf(possumStates.Error)
	}
	return possumStates.PossumStates, nil
}

func getPassel() ([]string, error) {
	passel, err := utils.GetPassel()
	if err != nil {
		return nil, err
	}
	if len(passel) == 0 {
		return nil, fmt.Errorf("Passel had 0 members")
	}
	return passel, nil
}

func setStates(httpClient *http.Client, passel []string, passelState []byte) ([]map[string]string, error) {
	var passelStates []map[string]string
	for _, possum := range passel {
		possumStates, err := setPasselState(httpClient, possum, passelState)
		if err != nil {
			return nil, err
		}
		passelStates = append(passelStates, possumStates)
	}
	return passelStates, nil
}

func setPasselState(httpClient *http.Client, possum string, passelState []byte) (map[string]string, error) {
	var possumStates PossumStates
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/state", possum), bytes.NewReader(passelState))
	if err != nil {
		log.WithFields(log.Fields{"package": "webServer", "function": "setPasselState", "URI": fmt.Sprintf("%s/v1/passel_state", possum)}).Debugf("Couldn't set possum state :%s", err)
		return nil, err
	}
	username, err := utils.GetUsername()
	if err != nil {
		return nil, err
	}

	password, err := utils.GetPassword()
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(username, password)
	resp, err := httpClient.Do(req)
	if err != nil {
		log.WithFields(log.Fields{"package": "webServer", "function": "setPasselState"}).Debugf("Couldn't authenticate :%s", err)
		return nil, err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(log.Fields{"package": "webServer", "function": "setPasselState"}).Debugf("Couldn't read body of request :%s", err)
		return nil, err
	}
	err = json.Unmarshal(data, &possumStates)
	if err != nil {
		log.WithFields(log.Fields{"package": "webServer", "function": "setPasselState"}).Debugf("Couldn't unmarshal JSON :%s", err)
		return nil, err
	}
	if possumStates.Error != "" {
		log.WithFields(log.Fields{"package": "webServer", "function": "setPasselState"}).Debug(possumStates.Error)
		return nil, fmt.Errorf(possumStates.Error)
	}
	return possumStates.PossumStates, nil
}

func desiredPossumInPassel(desiredPasselState map[string]string, passel []string) (bool, string) {
	for desiredPossum := range desiredPasselState {
		var found bool
		for _, possum := range passel {
			if desiredPossum == possum {
				found = true
			}
		}
		if !found {
			return false, desiredPossum
		}
	}
	return true, ""
}

func createHTTPClient() *http.Client {
	tlsConfig := new(tls.Config)
	if ca, err := ioutil.ReadFile("cacert.pem"); err == nil {
		tlsConfig.RootCAs = x509.NewCertPool()
		tlsConfig.RootCAs.AppendCertsFromPEM(ca)
	}
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	return &http.Client{Transport: transport}
}

func checkAuth(w http.ResponseWriter, r *http.Request) bool {
	s := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	if len(s) != 2 {
		log.WithFields(log.Fields{"package": "webServer", "function": "checkAuth"}).Debugf("No authorisation header found ")
		return false
	}

	b, err := base64.StdEncoding.DecodeString(s[1])
	if err != nil {
		log.WithFields(log.Fields{"package": "webServer", "function": "checkAuth"}).Debugf("Cannot decode string :%s ", err)
		return false
	}

	pair := strings.SplitN(string(b), ":", 2)
	if len(pair) != 2 {
		fmt.Println(err)
		return false
	}

	username, err := utils.GetUsername()
	if err != nil {
		fmt.Println(err)
		return false
	}

	password, err := utils.GetPassword()
	if err != nil {
		fmt.Println(err)
		return false
	}

	return pair[0] == username && pair[1] == password
}

func loadCORSAllowed() string {
	corsAllowed := os.Getenv("CORS_ALLOWED")
	if corsAllowed == "" {
		corsAllowed = "*"
	}
	return corsAllowed
}
