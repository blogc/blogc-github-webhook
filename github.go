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

type repository struct {
	CloneURL string `json:"clone_url"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Private  bool   `json:"private"`
}

type payload struct {
	After   string     `json:"after"`
	Deleted bool       `json:"deleted"`
	Ref     string     `json:"ref"`
	Repo    repository `json:"repository"`
}

func parsePayload(r *http.Request, secret string) (*payload, error) {
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
	if event == "ping" {
		return nil, nil
	}
	if event != "push" {
		return nil, fmt.Errorf("github: Invalid event (%s). Only push and ping events supported", event)
	}

	plS, err := ioutil.ReadAll(r.Body)
	if err != nil || len(plS) == 0 {
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
	if _, err := mac.Write(plS); err != nil {
		return nil, err
	}
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sign[1]), []byte(expectedMAC)) {
		return nil, fmt.Errorf("github: Failed to validate HMAC signature")
	}

	var pl payload
	if err = json.Unmarshal([]byte(plS), &pl); err != nil {
		return nil, err
	}

	return &pl, nil
}
