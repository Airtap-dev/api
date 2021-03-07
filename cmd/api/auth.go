package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	pTurn "github.com/pion/turn/v2"
)

func auth(f internalHandler) internalHandler {
	return func(acc account, w http.ResponseWriter, r *http.Request) (response, error) {
		id, token, ok := r.BasicAuth()
		if !ok {
			return nil, errInvalidCredentials
		}

		var firstName, lastName, code string
		row := dbGlobal.QueryRow(authenticateQuery, id, token)
		if err := row.Scan(&firstName, &lastName, &code); err == sql.ErrNoRows {
			return nil, errInvalidCredentials
		} else if err != nil {
			log.Print(err)
			return nil, errInternal
		}

		i, _ := strconv.Atoi(id)
		return f(account{id: i, firstName: firstName, lastName: lastName, code: code}, w, r)
	}
}

type turnCredentials struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

var turnKeys = []struct {
	url string
	env string
	id  int
}{
	{
		url: "turn:turn-de.airtap.dev:3478?transport=udp",
		env: "TURN_GERMANY_KEY",
	},
}

type startResponse struct {
	ID              int               `json:"accountId"`
	FirstName       string            `json:"firstName"`
	LastName        string            `json:"lastName"`
	Code            string            `json:"code"`
	TurnCredentials []turnCredentials `json:"turnCredentials"`
}

func start(acc account, w http.ResponseWriter, r *http.Request) (response, error) {
	creds, err := turn()
	if err != nil {
		return nil, err
	}

	return startResponse{
		ID:              acc.id,
		FirstName:       acc.firstName,
		LastName:        acc.lastName,
		Code:            acc.code,
		TurnCredentials: creds,
	}, nil
}

func turn() ([]turnCredentials, error) {
	creds := make([]turnCredentials, len(turnKeys))
	for i, key := range turnKeys {
		// Create credentials valid for a year.
		username, password, err := pTurn.GenerateLongTermCredentials(os.Getenv(key.env), 365*24*time.Hour)
		if err != nil {
			log.Print(err)
			return nil, errInternal
		}

		creds[i] = turnCredentials{
			URL:      key.url,
			Username: username,
			Password: password,
		}
	}

	return creds, nil
}
