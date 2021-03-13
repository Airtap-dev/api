package main

import "testing"

func TestRandString(t *testing.T) {
	if s, err := randString(12); err != nil {
		t.Error(err)
	} else if len(s) != 12 {
		t.Fail()
	}

	if s, err := randString(0); err != nil {
		t.Error(err)
	} else if len(s) != 0 {
		t.Fail()
	}
}
