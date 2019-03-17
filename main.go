package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
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

		log.Printf("main: %s: Processing webhook: %s", pl.Repo.FullName, pl.Ref)

		if pl.Ref != "refs/heads/master" {
			log.Printf("main: %s: Invalid ref (%s). Branch is not master", pl.Repo.FullName, pl.Ref)
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, "UNSUPPORTED BRANCH\n")
			return
		}

		if pl.Deleted {
			log.Printf("main: %s: Ref was deleted (%s)", pl.Repo.FullName, pl.Ref)
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, "BRANCH DELETED\n")
			return
		}

		if pl.Repo.Private && !hasApiKey {
			log.Printf("main: %s: BGW_API_KEY must be set for private repositories", pl.Repo.FullName)
			w.WriteHeader(http.StatusPreconditionFailed)
			io.WriteString(w, "NO API KEY\n")
			return
		}

		go func() {
			tempDir, err := downloadCommit(apiKey, pl)
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
