package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type repository struct {
	CloneURL string `json:"clone_url"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Private  bool   `json:"private"`
	Owner    *struct {
		Login string `json:"login"`
	} `json:"owner"`
}

type payload struct {
	Zen     string      `json:"zen"`
	After   string      `json:"after"`
	Deleted bool        `json:"deleted"`
	Ref     string      `json:"ref"`
	Repo    *repository `json:"repository"`
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
	if event != "push" && event != "ping" {
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

func (pl *payload) download(apiKey string) (string, error) {
	log.Printf("github: %s: Downloading commit: %s", pl.Repo.FullName, pl.After)

	dir, err := ioutil.TempDir("", "bgw_")
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/tarball/%s", pl.Repo.FullName, pl.After)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	if apiKey != "" {
		req.Header.Add("Authorization", fmt.Sprintf("token %s", apiKey))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return "", err
	}

	reader := tar.NewReader(gr)
	for {
		hdr, err := reader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return "", err
		}

		dirIndex := strings.Index(filepath.ToSlash(hdr.Name), "/")
		if dirIndex == -1 && !hdr.FileInfo().IsDir() {
			continue
		}

		fn := dir
		if dirIndex > 0 {
			fn = filepath.Join(dir, hdr.Name[dirIndex:])
		}

		if hdr.FileInfo().IsDir() {
			if err := os.MkdirAll(fn, os.FileMode(hdr.Mode)); err != nil {
				return "", err
			}
			continue
		}

		if hdr.Linkname != "" {
			if err := os.Symlink(hdr.Linkname, fn); err != nil {
				return "", err
			}
			continue
		}

		f, err := os.Create(fn)
		if err != nil {
			return "", err
		}

		if _, err := io.Copy(f, reader); err != nil {
			f.Close()
			return "", err
		}

		if err := f.Close(); err != nil {
			return "", err
		}

		if err := os.Chmod(fn, os.FileMode(hdr.Mode)); err != nil {
			return "", err
		}
	}

	return dir, nil
}

func (pl *payload) getBranch() string {
	if !strings.HasPrefix(pl.Ref, "refs/heads/") {
		return ""
	}

	return strings.TrimPrefix(pl.Ref, "refs/heads/")
}
