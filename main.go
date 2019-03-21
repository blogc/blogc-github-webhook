package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type HTTPError struct {
	statusCode int
	message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%d: %s", e.statusCode, e.message)
}

func build(pl *payload, allowedBranches []string, baseDir string, hasApiKey bool, apiKey string, async bool) *HTTPError {
	branch := pl.getBranch()

	log.Printf("main: %s: Processing webhook: %s (%s)", pl.Repo.FullName, pl.Ref, branch)

	found := false
	for _, v := range allowedBranches {
		if v == branch {
			found = true
			break
		}
	}

	if !found {
		log.Printf("main: %s: Invalid ref (%s). Branch is not allowed: %s", pl.Repo.FullName, pl.Ref, branch)
		return &HTTPError{
			statusCode: http.StatusAccepted,
			message:    "unsupported branch",
		}
	}

	if pl.Deleted {
		log.Printf("main: %s: Ref was deleted (%s). Branch will be cleaned up: %s", pl.Repo.FullName, pl.Ref, branch)
		if async {
			go blogcCleanup(baseDir, pl)
		} else {
			blogcCleanup(baseDir, pl)
		}
		return &HTTPError{
			statusCode: http.StatusAccepted,
			message:    "branch deleted",
		}
	}

	if pl.Repo.Private && !hasApiKey {
		log.Printf("main: %s: BGW_API_KEY must be set for private repositories", pl.Repo.FullName)
		return &HTTPError{
			statusCode: http.StatusPreconditionFailed,
			message:    "no api key",
		}
	}

	fn := func() {
		tempDir, err := pl.download(apiKey)
		if err != nil {
			log.Print(err)
			return
		}
		defer os.RemoveAll(tempDir)

		if err := blogcRun(tempDir, baseDir, pl); err != nil {
			log.Print(err)
			return
		}
	}

	if async {
		go fn()
	} else {
		fn()
	}

	return nil
}

func main() {
	secret, found := os.LookupEnv("BGW_SECRET")
	if !found {
		log.Fatalln("main: BGW_SECRET must be set")
	}

	baseDir, found := os.LookupEnv("BGW_BASEDIR")
	if !found {
		baseDir = "/var/www/blogc-github-webhook"
	}
	if !filepath.IsAbs(baseDir) {
		log.Fatalln("main: BGW_BASEDIR must be set to absolute path")
	}

	listenAddr, found := os.LookupEnv("BGW_LISTEN_ADDR")
	if !found {
		listenAddr = ":8000"
	}

	allowedBranches := []string{"master"}
	additionalBranches, found := os.LookupEnv("BGW_BRANCHES")
	if found {
		allowedBranches = []string{}
		for _, v := range strings.Split(additionalBranches, ",") {
			allowedBranches = append(allowedBranches, strings.TrimSpace(v))
		}
	}

	apiKey, hasApiKey := os.LookupEnv("BGW_API_KEY")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		pl, err := parsePayload(r, secret)
		if err != nil {
			log.Print(err)
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "INVALID JSON\n")
			return
		}

		if pl.Zen != "" {
			log.Printf("main: %s: ping: %s", pl.Repo.FullName, pl.Zen)
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "PONG\n")
			return
		}

		if err := build(pl, allowedBranches, baseDir, hasApiKey, apiKey, true); err != nil {
			w.WriteHeader(err.statusCode)
			io.WriteString(w, fmt.Sprintf("%s\n", strings.ToUpper(err.message)))
			return
		}

		w.WriteHeader(http.StatusAccepted)
		io.WriteString(w, "ACCEPTED\n")
	})

	log.Fatal(http.ListenAndServe(listenAddr, nil))
}
