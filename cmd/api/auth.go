package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
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
	// TODO: fix
	// {
	// url: "turns:turn.airtap.dev:5349",
	// env: "TURN_GERMANY_KEY",
	// },
	{
		url: "stun:stun.l.google.com:19302",
		env: "",
	},
}

// The username is a colon-delimited random string and an expiration timestamp.
// For more information, see the TURN REST API memo:
// https://tools.ietf.org/html/draft-uberti-behave-turn-rest-00.
func createUsername() (string, error) {
	tempID, err := randString(5)
	if err != nil {
		return "", err
	}

	username := fmt.Sprintf("%v:%v", strconv.FormatInt(time.Now().Add(365*24*time.Hour).Unix(), 10), tempID)
	return username, nil
}

// The password is base64(hmac(secret key, username)) where secret key is a key
// stored between the backend and the TURN server. HMAC uses SHA-1 as required
// by COTURN. For more information, see
// https://tools.ietf.org/html/draft-uberti-behave-turn-rest-00 and
// https://github.com/coturn/coturn/blob/060bf187879fd1a6386012f4c5a7494824ebe5c8/README.turnserver.
func createPassword(username, key string) (string, error) {
	hasher := hmac.New(sha1.New, []byte(os.Getenv(key)))
	if _, err := hasher.Write([]byte(username)); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(hasher.Sum(nil)), nil
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
		var username, password string
		var err error

		if username, err = createUsername(); err != nil {
			log.Print(err)
			return nil, errInternal
		}

		if password, err = createPassword(username, os.Getenv(key.env)); err != nil {
			log.Print(err)
			return nil, errInternal
		}

		username = "" // TODO: fix
		password = "" // TODO: fix
		creds[i] = turnCredentials{
			URL:      key.url,
			Username: username,
			Password: password,
		}
	}

	return creds, nil
}
