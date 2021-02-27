package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"

	"database/sql"

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

	// Make sure the table are in place.
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
}

type apiError struct {
	Code    int    `json:"error_code"`
	Message string `json:"message"`
}

type createRequest struct {
	LicenseKey string `json:"license_key"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name,omitempty"`
}

type createResponse struct {
	ShareableLink string `json:"shareable_link"`
	ID            int    `json:"id"`
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

func randString(n int) string {
	const charPool = "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = charPool[rand.Intn(len(charPool))]
	}
	return string(b)
}

func createAccount(req createRequest, w http.ResponseWriter, r *http.Request) {
	code, token := randString(12), randString(128)
	row := dbGlobal.QueryRow(createAccountQuery, req.LicenseKey, code, token, req.FirstName, req.LastName)

	var id int
	if err := row.Scan(&id); err == sql.ErrNoRows {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInvalidCode,
			Message: messageInvalidCode,
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
		ShareableLink: "https://joinairtap.com/" + strings.ToLower(req.FirstName) + "/" + code,
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
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
}

func discover(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.NotFound(w, r)
		return
	}

	code := r.URL.Query()["code"][0]
	row := dbGlobal.QueryRow(discoverAccountQuery, code)

	var id int
	var firstName, lastName string
	if err := row.Scan(&id, &firstName, &lastName); err != nil {
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

func auth(handler http.HandlerFunc) http.HandlerFunc {
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

		handler(w, r)
	}
}

func main() {
	http.HandleFunc("/account/create", create)
	http.HandleFunc("/account/create/", create)
	http.HandleFunc("/account/discover/", auth(discover))
	http.HandleFunc("/account/discover", auth(discover))
	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}
