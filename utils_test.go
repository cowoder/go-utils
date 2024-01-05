package utils

import "testing"

func TestUtils_RandomString(t *testing.T) {
	var testUtils Utils

	s := testUtils.RandomString(32)

	if len(s) != 32 {
		t.Errorf("RandomString() = %s; want length 32", s)
	}
}
