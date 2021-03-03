package main

import (
	"crypto/rand"
	"encoding/json"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"

	"database/sql"
	"server/lib/relay"

	_ "github.com/lib/pq"
)

const (
	codeInternalError    int    = 0
	messageInternalError string = "Internal error."

	codeInvalidBody    int    = 1
	messageInvalidBody string = "Could not parse request body."

	codeInvalidLicense    int    = 2
	messageInvalidLicense string = "Invalid license."

	codeInvalidCode    int    = 3
	messageInvalidCode string = "Could not find requested account."

	codeInvalidCredentials    int    = 4
	messageInvalidCredentials string = "Invalid credentials."
)

var dbGlobal *sql.DB

func init() {
	// Connect.
	if db, err := sql.Open("postgres", os.Getenv("DATABASE_URL")); err != nil {
		log.Panic(err)
	} else {
		dbGlobal = db
	}

	// Ping.
	if err := dbGlobal.Ping(); err != nil {
		log.Panic(err)
	}

	// Make sure the tables are in place.
	if dat, err := ioutil.ReadFile("schemas/license_keys.sql"); err != nil {
		log.Panic(err)
	} else if _, err := dbGlobal.Exec(string(dat)); err != nil {
		log.Panic(err)
	}

	if dat, err := ioutil.ReadFile("schemas/accounts.sql"); err != nil {
		log.Panic(err)
	} else if _, err := dbGlobal.Exec(string(dat)); err != nil {
		log.Panic(err)
	}

	pool.connections = make(map[int]*relay.Conn)
}

type apiError struct {
	Code    int    `json:"errorCode"`
	Message string `json:"message"`
}

type createRequest struct {
	LicenseKey string `json:"licenseKey"`
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName,omitempty"`
}

type createResponse struct {
	ShareableLink string `json:"shareableLink"`
	ID            int    `json:"accountId"`
	Token         string `json:"token"`
}

func create(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	decoder := json.NewDecoder(r.Body)
	var req createRequest
	if err := decoder.Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInvalidBody,
			Message: messageInvalidBody,
		})
		log.Print(err)
		return
	}

	if checkLicense(req.LicenseKey, w, r) {
		createAccount(req, w, r)
	}
}

func randString(n int) (string, error) {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		ret[i] = alphabet[num.Int64()]
	}

	return string(ret), nil
}

// TODO: handle empty firstname
func createAccount(req createRequest, w http.ResponseWriter, r *http.Request) {
	code, err := randString(12)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInternalError,
			Message: messageInternalError,
		})

		log.Print(err)
		return
	}

	token, err := randString(128)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInternalError,
			Message: messageInternalError,
		})

		log.Print(err)
		return
	}

	row := dbGlobal.QueryRow(createAccountQuery, req.LicenseKey, code, token, req.FirstName, req.LastName)
	var id int
	if err := row.Scan(&id); err == sql.ErrNoRows {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInternalError,
			Message: messageInternalError,
		})

		log.Print(err)
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInternalError,
			Message: messageInternalError,
		})

		log.Print(err)
		return
	}

	if err := json.NewEncoder(w).Encode(createResponse{
		ShareableLink: "https://joinairtap.com/with/" + strings.ToLower(req.FirstName) + "/" + code,
		ID:            id,
		Token:         token,
	}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInternalError,
			Message: messageInternalError,
		})

		log.Print(err)
	}
}

func checkLicense(license string, w http.ResponseWriter, r *http.Request) bool {
	row := dbGlobal.QueryRow(findLicenseQuery, license)

	var maxActivations int
	var revoked bool
	if err := row.Scan(&maxActivations, &revoked); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInternalError,
			Message: messageInternalError,
		})

		log.Print(err)
		return false
	}

	if revoked {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInvalidLicense,
			Message: messageInvalidLicense,
		})

		return false
	}

	row = dbGlobal.QueryRow(findLicenseUsers, license)
	var currentActivations int
	if err := row.Scan(&currentActivations); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInternalError,
			Message: messageInternalError,
		})

		log.Print(err)
		return false
	}

	if currentActivations >= maxActivations {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInvalidLicense,
			Message: messageInvalidLicense,
		})

		return false
	}

	return true
}

type discoverResponse struct {
	ID        int    `json:"accountId"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName,omitempty"`
}

// TODO: handle ErrNoRows
func discover(acc account, w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.NotFound(w, r)
		return
	}

	var code string
	if codes := r.URL.Query()["code"]; len(codes) != 0 {
		code = r.URL.Query()["code"][0]
	} else {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInvalidCode,
			Message: messageInvalidCode,
		})
		return
	}

	row := dbGlobal.QueryRow(discoverAccountQuery, code)

	var id int
	var firstName, lastName string
	if err := row.Scan(&id, &firstName, &lastName); err == sql.ErrNoRows {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInvalidCode,
			Message: messageInvalidCode,
		})
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInternalError,
			Message: messageInternalError,
		})

		log.Print(err)
		return
	}

	if err := json.NewEncoder(w).Encode(discoverResponse{
		ID:        id,
		FirstName: firstName,
		LastName:  lastName,
	}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInternalError,
			Message: messageInternalError,
		})

		log.Print(err)
	}
}

type account struct {
	id int
}

func auth(handler func(account, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		id, token, ok := r.BasicAuth()
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(apiError{
				Code:    codeInvalidCredentials,
				Message: messageInvalidCredentials,
			})
			return
		}

		row := dbGlobal.QueryRow(authenticateQuery, id, token)
		var count int
		if err := row.Scan(&count); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(apiError{
				Code:    codeInternalError,
				Message: messageInternalError,
			})

			log.Print(err)
			return
		}

		if count != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(apiError{
				Code:    codeInvalidCredentials,
				Message: messageInvalidCredentials,
			})
			return
		}

		i, _ := strconv.Atoi(id)
		handler(account{i}, w, r)
	}
}

func main() {
	http.HandleFunc("/ws", auth(ws))
	http.HandleFunc("/ws/", auth(ws))
	http.HandleFunc("/account/create", create)
	http.HandleFunc("/account/create/", create)
	http.HandleFunc("/session/turn", auth(turn))
	http.HandleFunc("/session/turn/", auth(turn))
	http.HandleFunc("/account/discover/", auth(discover))
	http.HandleFunc("/account/discover", auth(discover))
	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}
