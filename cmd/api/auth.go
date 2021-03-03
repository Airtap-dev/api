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

func turn(acc account, w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.NotFound(w, r)
		return
	}

	servers := make([]turnCredentials, len(turnKeys))
	for i, key := range turnKeys {
		tempID, err := randString(5)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(apiError{
				Code:    codeInternalError,
				Message: messageInternalError,
			})

			log.Print(err)
			return
		}

		username := fmt.Sprintf("%v:%v", strconv.FormatInt(time.Now().Add(365*24*time.Hour).Unix(), 10), tempID)
		hasher := hmac.New(sha1.New, []byte(os.Getenv(key.env)))
		if _, err := hasher.Write([]byte(username)); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(apiError{
				Code:    codeInternalError,
				Message: messageInternalError,
			})

			log.Print(err)
			return
		}

		pwd := base64.StdEncoding.EncodeToString(hasher.Sum(nil))
		servers[i] = turnCredentials{
			URL:      key.url,
			Username: username,
			Password: pwd,
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
