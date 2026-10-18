package solis

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"
)

const contentTypeHeader = "application/json;charset=UTF-8"
const contentTypeSign = "application/json"

// SignRequest returns the Content-MD5 and Authorization header values for one
// SolisCloud request. The body must be the exact bytes sent on the wire.
func SignRequest(apiKey, apiSecret, method, uri string, body []byte, date time.Time) (contentMD5, authorization string) {
	sum := md5.Sum(body)
	md5b64 := base64.StdEncoding.EncodeToString(sum[:])

	dateStr := date.UTC().Format(http.TimeFormat)
	toSign := method + "\n" + md5b64 + "\n" + contentTypeSign + "\n" + dateStr + "\n" + uri

	mac := hmac.New(sha1.New, []byte(apiSecret))
	mac.Write([]byte(toSign))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return md5b64, fmt.Sprintf("API %s:%s", apiKey, sig)
}

// SignStringToSign exposes the canonical string for tests and diagnostics.
func SignStringToSign(method, uri string, body []byte, date time.Time) string {
	sum := md5.Sum(body)
	md5b64 := base64.StdEncoding.EncodeToString(sum[:])
	return method + "\n" + md5b64 + "\n" + contentTypeSign + "\n" + date.UTC().Format(http.TimeFormat) + "\n" + uri
}

// ContentType returns the Content-Type header value used by every request.
func ContentType() string { return contentTypeHeader }
