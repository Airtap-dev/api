package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

type account struct {
	id        int
	code      string
	firstName string
	lastName  string
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

func create(acc account, w http.ResponseWriter, r *http.Request) (response, error) {
	decoder := json.NewDecoder(r.Body)
	var req createRequest
	if err := decoder.Decode(&req); err != nil {
		log.Print(err)
		return nil, errInternal
	}

	if ok, id, err := checkLicense(req.LicenseKey); err != nil {
		return nil, err
	} else if ok {
		return createAccount(id, req.FirstName, req.LastName)
	} else {
		return nil, errInvalidLicense
	}
}

func createAccount(licenseID int, firstName, lastName string) (response, error) {
	if firstName == "" || strings.Count(firstName, " ") == len(firstName) {
		return nil, errInvalidBody
	}

	code, err := randString(12)
	if err != nil {
		log.Println(err)
		return nil, errInternal
	}

	token, err := randString(128)
	if err != nil {
		log.Print(err)
		return nil, errInternal
	}

	row := dbGlobal.QueryRow(createAccountQuery, licenseID, code, token, firstName, lastName)
	var id int
	if err := row.Scan(&id); err != nil {
		log.Print(err)
		return nil, errInternal
	}

	return createResponse{
		ShareableLink: "https://joinairtap.com/with/" + code,
		ID:            id,
		Token:         token,
	}, nil
}

type discoverResponse struct {
	ID        int    `json:"accountId"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName,omitempty"`
}

func discover(acc account, w http.ResponseWriter, r *http.Request) (response, error) {
	var code string
	log.Println(r.URL)
	if codes := r.URL.Query()["code"]; len(codes) != 0 {
		code = r.URL.Query()["code"][0]
	} else {
		return nil, errInvalidCode
	}

	log.Println("here")
	row := dbGlobal.QueryRow(discoverAccountQuery, code)

	var id int
	var firstName, lastName string
	if err := row.Scan(&id, &firstName, &lastName); err == sql.ErrNoRows {
		return nil, errInvalidCode
	} else if err != nil {
		log.Println(err)
		return nil, errInternal
	}

	return discoverResponse{
		ID:        id,
		FirstName: firstName,
		LastName:  lastName,
	}, nil
}
