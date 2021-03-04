package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

type turnCredentials struct {
	ID       int    `json:"serverId"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type turnResponse struct {
	Servers []turnCredentials `json:"servers"`
}

var turnKeys = []struct {
	url string
	env string
	id  int
}{
	{
		url: "turns:turn.airtap.dev:5349",
		env: "TURN_GERMANY_KEY",
		id:  1,
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

func turn(acc account, w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.NotFound(w, r)
		return
	}

	servers := make([]turnCredentials, len(turnKeys))
	for i, key := range turnKeys {
		var username, password string
		var err error

		if username, err = createUsername(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(apiError{
				Code:    codeInternalError,
				Message: messageInternalError,
			})

			log.Print(err)
			return
		}

		if password, err = createPassword(username, os.Getenv(key.env)); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(apiError{
				Code:    codeInternalError,
				Message: messageInternalError,
			})

			log.Print(err)
			return
		}

		servers[i] = turnCredentials{
			URL:      key.url,
			Username: username,
			Password: password,
			ID:       key.id,
		}
	}

	if err := json.NewEncoder(w).Encode(turnResponse{
		Servers: servers,
	}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(apiError{
			Code:    codeInternalError,
			Message: messageInternalError,
		})

		log.Print(err)
	}
}
