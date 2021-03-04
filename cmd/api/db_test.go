package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestDB(t *testing.T) {
	var (
		license           string
		err               error
		expectedFirstName = "Alexander"
		expectedLastName  = "Macedon"
	)

	license = testLicenseCreationQuery(t)
	if _, _, err = testLicenseCheck(t, license); err != nil {
		t.Error(err)
	}

	id, token, link := testCreateAccount(t, license, expectedFirstName, expectedLastName)
	req := makeAuthenticatedRequest(id, token)
	startFunc := auth(start)
	testAccountStart(t, startFunc, req, expectedFirstName, expectedLastName, id)

	discoverFunc := auth(discover)
	code := strings.Split(link, "/")[len(strings.Split(link, "/"))-1]
	url, _ := url.Parse("api.airtap.dev/account/discover?code=" + code)
	req.URL = url
	testAccountDiscover(t, discoverFunc, req, expectedFirstName, expectedLastName, id)
}

func testAccountDiscover(t *testing.T, f internalHandler, req *http.Request, firstName, lastName string, id int) {
	if res, err := f(account{}, nil, req); err != nil {
		t.Error(err)
	} else if r, ok := res.(discoverResponse); !ok {
		t.Errorf("got unexpected start account return type: %v", r)
	} else {
		if r.ID != id || r.FirstName != firstName || r.LastName != lastName {
			t.Errorf("bad account info returned: %v %v %v", r.ID, r.FirstName, r.LastName)
		}
	}
}

func makeAuthenticatedRequest(id int, token string) *http.Request {
	header := "BASIC " + base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(id)+":"+token))

	return &http.Request{
		Header: map[string][]string{
			"Authorization": []string{header},
		},
	}
}

func testAccountStart(t *testing.T, f internalHandler, req *http.Request, firstName, lastName string, id int) {
	if res, err := f(account{}, nil, req); err != nil {
		t.Error(err)
	} else if r, ok := res.(startResponse); !ok {
		t.Errorf("got unexpected start account return type: %v", r)
	} else {
		if r.ID != id || r.FirstName != firstName || r.LastName != lastName || r.Code == "" {
			t.Errorf("bad account info returned: %v %v %v %v", r.ID, r.FirstName, r.LastName, r.Code)
		}

		if len(r.TurnCredentials) == 0 {
			t.Error("empty turn credentials returned")
		} else if r.TurnCredentials[0].URL == "" /*|| r.TurnCredentials[0].Username == "" || r.TurnCredentials[0].Password == "" TODO: fix*/ {
			t.Errorf("bad turn credentials returned: %v %v %v", r.TurnCredentials[0].URL, r.TurnCredentials[0].Username, r.TurnCredentials[0].Password)
		}
	}
}

func testCreateAccount(t *testing.T, license, firstName, lastName string) (int, string, string) {
	res, err := create(account{}, nil, &http.Request{
		Body: ioutil.NopCloser(bytes.NewReader([]byte(fmt.Sprintf(`{"licenseKey":"%v","firstName":"%v","lastName":"%v"}`, license, firstName, lastName)))),
	})

	if err != nil {
		t.Error(err)
	}

	if r, ok := res.(createResponse); !ok {
		t.Errorf("got unexpected create account return type: %v", r)
	} else {
		return r.ID, r.Token, r.ShareableLink
	}

	return 0, "", ""
}

func testLicenseCheck(t *testing.T, license string) (bool, int, error) {
	return checkLicense(license)
}

func testLicenseCreationQuery(t *testing.T) string {
	row := dbGlobal.QueryRow(issueQuery, 10)

	var (
		license        string
		maxActivations int
		revoked        bool
	)

	if err := row.Scan(&license, &maxActivations, &revoked); err != nil {
		t.Error(err)
	}

	if license == "" || maxActivations != 10 || revoked {
		t.Errorf("wrong license created: %v %v %v", license, maxActivations, revoked)
	}

	return license
}
