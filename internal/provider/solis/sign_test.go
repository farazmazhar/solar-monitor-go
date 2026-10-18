package solis

import (
	"testing"
	"time"
)

func TestSignRequestGoldenVector(t *testing.T) {
	body := []byte(`{"id":"987654321"}`)
	date, err := time.Parse(time.RFC1123, "Fri, 26 Jul 2019 06:00:46 GMT")
	if err != nil {
		t.Fatal(err)
	}

	md5b64, auth := SignRequest("1234567890", "test-secret", "POST", "/v1/api/inverterDetail", body, date)

	if want := "7/EfoCzYekS+yF7SsMqbLQ=="; md5b64 != want {
		t.Errorf("Content-MD5 = %q, want %q", md5b64, want)
	}
	if want := "API 1234567890:AoWrY3UMKwwp+ZsWTo5yhElaDYg="; auth != want {
		t.Errorf("Authorization = %q, want %q", auth, want)
	}

	sts := SignStringToSign("POST", "/v1/api/inverterDetail", body, date)
	if want := "POST\n7/EfoCzYekS+yF7SsMqbLQ==\napplication/json\nFri, 26 Jul 2019 06:00:46 GMT\n/v1/api/inverterDetail"; sts != want {
		t.Errorf("stringToSign =\n%q\nwant\n%q", sts, want)
	}
}

func TestSignRequestUsesExactBodyBytes(t *testing.T) {
	date := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	compact := []byte(`{"id":"1"}`)
	spaced := []byte(`{"id": "1"}`)
	_, a := SignRequest("k", "s", "POST", "/x", compact, date)
	_, b := SignRequest("k", "s", "POST", "/x", spaced, date)
	if a == b {
		t.Fatal("signature must depend on the exact body bytes")
	}
}
