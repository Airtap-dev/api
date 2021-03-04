package main

import (
	"os"
	"strings"
	"testing"
)

func TestKeys(t *testing.T) {
	if len(turnKeys) == 0 {
		t.Fail()
	}
}

func TestCreateUsername(t *testing.T) {
	if username, err := createUsername(); err != nil {
		t.Fail()
	} else if !strings.Contains(username, ":") {
		t.Fail()
	}
}

func TestCreatePassword(t *testing.T) {
	if username, err := createUsername(); err != nil || len(username) < 10 || !strings.Contains(username, ":") {
		t.Fail()
	} else if password, err := createPassword(username, os.Getenv(turnKeys[0].env)); err != nil || len(password) < 10 {
		t.Fail()
	}
}
