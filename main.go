package main

import (
	"io"
	"log"
	"net/http"
	"os"
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

		log.Printf("main: Processing webhook for %s: %s", pl.Repo.FullName, pl.Ref)

		if pl.Ref != "refs/heads/master" {
			log.Printf("main: Invalid ref (%s). Branch is not master", pl.Ref)
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, "UNSUPPORTED BRANCH\n")
			return
		}

		if pl.Deleted {
			log.Printf("main: Ref was deleted (%s)", pl.Ref)
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, "BRANCH DELETED\n")
			return
		}

		if pl.Repo.Private && !hasApiKey {
			log.Printf("main: BGW_API_KEY must be set for private repositories")
			w.WriteHeader(http.StatusPreconditionFailed)
			io.WriteString(w, "NO API KEY\n")
			return
		}

		go func() {
			tempDir, err := gitClone(apiKey, &pl.Repo)
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
