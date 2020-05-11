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

func build(pl *payload, allowedBranches []string, baseDir string, apiKey string, async bool) *HTTPError {
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
			go builderCleanup(baseDir, pl)
		} else {
			builderCleanup(baseDir, pl)
		}
		return &HTTPError{
			statusCode: http.StatusAccepted,
			message:    "branch deleted",
		}
	}

	fn := func() {
		tempDir, err := pl.download(apiKey)
		if err != nil {
			log.Print(err)
			return
		}
		defer os.RemoveAll(tempDir)

		if err := builderRun(tempDir, baseDir, pl); err != nil {
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

	apiKey, found := os.LookupEnv("BGW_API_KEY")
	if !found {
		log.Fatalln("main: BGW_API_KEY must be set")
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

	if len(os.Args) > 2 {
		fullName := os.Args[1]
		branch := os.Args[2]

		pieces := strings.Split(fullName, "/")
		if len(pieces) != 2 {
			log.Fatalln("main: invalid full name:", fullName)
		}

		sha, err := getRef(fullName, branch, apiKey)
		if err != nil {
			log.Fatalln(err)
		}

		pl := &payload{
			After:   sha,
			Deleted: false,
			Ref:     fmt.Sprintf("refs/heads/%s", branch),
			Repo: &repository{
				Name:     pieces[1],
				FullName: fullName,
				Owner: &owner{
					Login: pieces[0],
				},
			},
		}

		if err := build(pl, allowedBranches, baseDir, apiKey, false); err != nil {
			log.Fatalln(err)
		}

		return
	}

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

		if err := build(pl, allowedBranches, baseDir, apiKey, true); err != nil {
			w.WriteHeader(err.statusCode)
			io.WriteString(w, fmt.Sprintf("%s\n", strings.ToUpper(err.message)))
			return
		}

		w.WriteHeader(http.StatusAccepted)
		io.WriteString(w, "ACCEPTED\n")
	})

	log.Fatal(http.ListenAndServe(listenAddr, nil))
}
