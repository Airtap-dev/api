package main

import (
	"crypto/rand"
	"encoding/json"
	"log"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"

	"database/sql"
	"server/lib/relay"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

const (
	codeInternalError    int    = 0
	messageInternalError string = "Internal error."

	codeInvalidBody    int    = 1
	messageInvalidBody string = "Invalid body."

	codeInvalidLicense    int    = 2
	messageInvalidLicense string = "Invalid license."

	codeInvalidCode    int    = 3
	messageInvalidCode string = "Could not find requested account."

	codeInvalidCredentials    int    = 4
	messageInvalidCredentials string = "Invalid credentials."
)

var dbGlobal *sql.DB

func init() {
	if os.Getenv("TEST") != "" {
		return
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)

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
	if driver, err := postgres.WithInstance(dbGlobal, &postgres.Config{}); err != nil {
		log.Panic(err)
	} else if m, err := migrate.NewWithDatabaseInstance("file://migrations", "postgres", driver); err != nil {
		log.Panic(err)
	} else {
		m.Up()
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

	if ok, id := checkLicense(req.LicenseKey, w, r); ok {
		createAccount(id, req.FirstName, req.LastName, w, r)
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

func createAccount(licenseID int, firstName, lastName string, w http.ResponseWriter, r *http.Request) {
	if firstName == "" || strings.Count(firstName, " ") == len(firstName) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInvalidBody,
			Message: messageInvalidBody,
		})

		return
	}

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

	row := dbGlobal.QueryRow(createAccountQuery, licenseID, code, token, firstName, lastName)
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
		ShareableLink: "https://joinairtap.com/with/" + code,
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

func checkLicense(license string, w http.ResponseWriter, r *http.Request) (bool, int) {
	row := dbGlobal.QueryRow(findLicenseQuery, license)

	var id, maxActivations int
	var revoked bool
	if err := row.Scan(&id, &maxActivations, &revoked); err == sql.ErrNoRows {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInvalidLicense,
			Message: messageInvalidLicense,
		})

		log.Print(err)
		return false, 0
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInternalError,
			Message: messageInternalError,
		})

		log.Print(err)
		return false, 0
	}

	if revoked {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInvalidLicense,
			Message: messageInvalidLicense,
		})

		return false, 0
	}

	row = dbGlobal.QueryRow(findLicenseUsers, id)
	var currentActivations int
	if err := row.Scan(&currentActivations); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInternalError,
			Message: messageInternalError,
		})

		log.Print(err)
		return false, 0
	}

	if currentActivations >= maxActivations {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInvalidLicense,
			Message: messageInvalidLicense,
		})

		return false, 0
	}

	return true, id
}

type discoverResponse struct {
	ID        int    `json:"accountId"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName,omitempty"`
}

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
		if err := row.Scan(&count); err == sql.ErrNoRows {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(apiError{
				Code:    codeInvalidCredentials,
				Message: messageInvalidCredentials,
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

func preHandle(handler func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	}
}

func main() {
	http.HandleFunc("/ws", preHandle(auth(ws)))
	http.HandleFunc("/ws/", preHandle(auth(ws)))
	http.HandleFunc("/account/create", preHandle(create))
	http.HandleFunc("/account/create/", preHandle(create))
	http.HandleFunc("/rtc/servers", preHandle(auth(turn)))
	http.HandleFunc("/rtc/servers/", preHandle(auth(turn)))
	http.HandleFunc("/account/discover/", preHandle(auth(discover)))
	http.HandleFunc("/account/discover", preHandle(auth(discover)))
	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}
