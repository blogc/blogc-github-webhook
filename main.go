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
		log.Fatalln("Error: BGW_SECRET must be set")
	}

	baseDir, found := os.LookupEnv("BGW_BASEDIR")
	if !found {
		baseDir = "/var/blogc-github-webhook"
	}

	listenAddr, found := os.LookupEnv("BGW_LISTEN_ADDR")
	if !found {
		listenAddr = ":8000"
	}

	apiKey := os.Getenv("BGW_API_KEY")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		payload, err := parsePayload(r, secret)
		if err != nil {
			log.Print(err)
			io.WriteString(w, "FAIL\n")
			return
		}

		log.Printf("main: Processing webhook for %s: %s", payload.Repo.FullName, payload.Ref)

		if payload.Ref != "refs/heads/master" {
			log.Printf("main: Invalid ref (%s). Branch is not master", payload.Ref)
			io.WriteString(w, "OK\n")
			return
		}

		if payload.Deleted {
			log.Printf("main: Ref was deleted (%s)", payload.Ref)
			io.WriteString(w, "OK\n")
			return
		}

		go func() {
			tempDir, err := gitClone(apiKey, &payload.Repo)
			if err != nil {
				log.Print(err)
				return
			}
			defer os.RemoveAll(tempDir)

			if err := blogcRun(tempDir, baseDir, payload); err != nil {
				log.Print(err)
				return
			}
		}()

		io.WriteString(w, "OK\n")
	})

	log.Fatal(http.ListenAndServe(listenAddr, nil))
}
