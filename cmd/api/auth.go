package main

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"

	"server/lib/geobalance"

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

type startResponse struct {
	ID              int               `json:"accountId"`
	FirstName       string            `json:"firstName"`
	LastName        string            `json:"lastName"`
	ShareableLink   string            `json:"shareableLink"`
	TurnCredentials []turnCredentials `json:"turnCredentials"`
}

func start(acc account, w http.ResponseWriter, r *http.Request) (response, error) {
	var countryCode = r.Header.Get("CF-IPCountry")
	if countryCode == "" {
		countryCode = "unknown"
	}

	creds, err := turn(countryCode)
	if err != nil {
		return nil, err
	}

	return startResponse{
		ID:              acc.id,
		FirstName:       acc.firstName,
		LastName:        acc.lastName,
		ShareableLink:   createShareableLink(acc.code),
		TurnCredentials: creds,
	}, nil
}

func turn(countryCode string) ([]turnCredentials, error) {
	url, key := geobalance.Balance(countryCode)
	// Create credentials valid for a year.
	username, password, err := pTurn.GenerateLongTermCredentials(key, 365*24*time.Hour)
	if err != nil {
		log.Print(err)
		return nil, errInternal
	}

	creds := turnCredentials{
		URL:      url,
		Username: username,
		Password: password,
	}

	return []turnCredentials{creds}, nil
}
