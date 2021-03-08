package geobalance

import "testing"

func TestBalance(t *testing.T) {
	if url, key := Balance("US"); len(url) == 0 || len(key) == 0 {
		t.Fail()
	}
}
