package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
)

func gitClone(apiKey string, r *Repository) (string, error) {
	dir, err := ioutil.TempDir("", "bgw_")
	if err != nil {
		return "", err
	}

	var repo string
	if apiKey != "" {
		repo = fmt.Sprintf("https://%s@github.com/%s.git", apiKey, r.FullName)
	} else {
		repo = fmt.Sprintf("https://github.com/%s.git", r.FullName)
	}

	cmd := exec.Command("git", "clone", "--depth=1", repo, dir)
	out, err := cmd.CombinedOutput()
	log.Printf("git: Cloning repository: %s\n%s", r.FullName, string(out))
	if err != nil {
		return "", err
	}

	return dir, nil
}
