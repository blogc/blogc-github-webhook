package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

type Repository struct {
	CloneURL string `json:"clone_url"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
}

type Payload struct {
	After   string     `json:"after"`
	Deleted bool       `json:"deleted"`
	Ref     string     `json:"ref"`
	Repo    Repository `json:"repository"`
}

func parsePayload(r *http.Request, secret string) (*Payload, error) {
	defer func() {
		_, _ = io.Copy(ioutil.Discard, r.Body)
		_ = r.Body.Close()
	}()

	if r.Method != http.MethodPost {
		return nil, fmt.Errorf("github: Invalid HTTP method (%s)", r.Method)
	}

	event := r.Header.Get("X-GitHub-Event")
	if event == "" {
		return nil, fmt.Errorf("github: Missing GitHub event")
	}
	if event != "push" {
		return nil, fmt.Errorf("github: Invalid event (%s). Only push event supported", event)
	}

	payload, err := ioutil.ReadAll(r.Body)
	if err != nil || len(payload) == 0 {
		return nil, fmt.Errorf("github: Failed to parse payload")
	}

	signature := r.Header.Get("X-Hub-Signature")
	if len(signature) == 0 {
		return nil, fmt.Errorf("github: Missing GitHub signature")
	}

	sign := strings.Split(signature, "=")
	if len(sign) != 2 {
		return nil, fmt.Errorf("github: Malformed GitHub signature")
	}

	if sign[0] != "sha1" {
		return nil, fmt.Errorf("github: Invalid signature algorithm (%s). Only sha1 supported", sign[0])
	}

	mac := hmac.New(sha1.New, []byte(secret))
	if _, err := mac.Write(payload); err != nil {
		return nil, err
	}
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature[5:]), []byte(expectedMAC)) {
		return nil, fmt.Errorf("github: Failed to validate HMAC signature")
	}

	var pl Payload
	if err = json.Unmarshal([]byte(payload), &pl); err != nil {
		return nil, err
	}

	return &pl, nil
}
