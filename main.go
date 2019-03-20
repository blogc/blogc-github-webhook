package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

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
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, "UNSUPPORTED BRANCH\n")
			return
		}

		if pl.Deleted {
			log.Printf("main: %s: Ref was deleted (%s). Branch will be cleaned up: %s", pl.Repo.FullName, pl.Ref, branch)
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, "BRANCH DELETED\n")
			go blogcCleanup(baseDir, pl)
			return
		}

		if pl.Repo.Private && !hasApiKey {
			log.Printf("main: %s: BGW_API_KEY must be set for private repositories", pl.Repo.FullName)
			w.WriteHeader(http.StatusPreconditionFailed)
			io.WriteString(w, "NO API KEY\n")
			return
		}

		go func() {
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
		}()

		w.WriteHeader(http.StatusAccepted)
		io.WriteString(w, "ACCEPTED\n")
	})

	log.Fatal(http.ListenAndServe(listenAddr, nil))
}
