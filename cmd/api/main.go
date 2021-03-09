package main

import (
	"crypto/rand"
	"encoding/json"
	"log"
	"math/big"
	"net/http"
	"os"

	"database/sql"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

type internalHandler func(acc account, w http.ResponseWriter, r *http.Request) (response, error)

type response interface{}

type apiError struct {
	Code       int    `json:"errorCode"`
	Message    string `json:"message"`
	httpStatus int
}

func (e apiError) Error() string {
	return e.Message
}

var dbGlobal *sql.DB

var (
	errInternal = apiError{
		Code:       0,
		Message:    "internal error",
		httpStatus: http.StatusInternalServerError,
	}

	errInvalidBody = apiError{
		Code:       1,
		Message:    "invalid body",
		httpStatus: http.StatusBadRequest,
	}

	errInvalidLicense = apiError{
		Code:       2,
		Message:    "invalid license",
		httpStatus: http.StatusBadRequest,
	}

	errInvalidCode = apiError{
		Code:       3,
		Message:    "invalid account code",
		httpStatus: http.StatusBadRequest,
	}

	errInvalidCredentials = apiError{
		Code:       4,
		Message:    "invalid credentials",
		httpStatus: http.StatusUnauthorized,
	}
)

func init() {
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

	var migrationsPath string
	if os.Getenv("TEST") != "" {
		migrationsPath = "file://../../migrations"
	} else {
		migrationsPath = "file://migrations"
	}

	// Make sure the tables are in place.
	if driver, err := postgres.WithInstance(dbGlobal, &postgres.Config{}); err != nil {
		log.Panic(err)
	} else if m, err := migrate.NewWithDatabaseInstance(migrationsPath, "postgres", driver); err != nil {
		log.Panic(err)
	} else if err := m.Up(); err != migrate.ErrNoChange && err != nil {
		log.Panic(err)
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

func router(method string, f internalHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != method {
			http.NotFound(w, r)
			return
		}

		if res, err := f(account{}, w, r); err != nil {
			if e, ok := err.(apiError); ok {
				w.WriteHeader(e.httpStatus)
				json.NewEncoder(w).Encode(e)
				return
			}

			log.Print("unknown error type")
			w.WriteHeader(errInternal.httpStatus)
			json.NewEncoder(w).Encode(errInternal)
		} else if res != nil {
			switch r := res.(type) {
			case createResponse, discoverResponse, startResponse:
				json.NewEncoder(w).Encode(r)
				return
			default:
				log.Printf("unknown response type: %v", res)
				w.WriteHeader(errInternal.httpStatus)
				json.NewEncoder(w).Encode(errInternal)
				return
			}
		}
	}
}

func main() {
	http.HandleFunc("/ws", router("GET", auth(ws)))
	http.HandleFunc("/ws/", router("GET", auth(ws)))
	http.HandleFunc("/account/create", router("POST", create))
	http.HandleFunc("/account/create/", router("POST", create))
	http.HandleFunc("/account/start", router("GET", auth(start)))
	http.HandleFunc("/account/discover/", router("GET", auth(discover)))
	http.HandleFunc("/account/discover", router("GET", auth(discover)))
	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}
