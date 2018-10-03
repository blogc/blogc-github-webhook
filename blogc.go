package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func blogcRun(tempDir string, baseDir string, p *payload) error {
	blogcfile := filepath.Join(tempDir, "blogcfile")
	if _, err := os.Stat(blogcfile); os.IsNotExist(err) {
		return fmt.Errorf("blogc: blogcfile not found")
	}

	fullName := strings.Split(p.Repo.FullName, "/")
	if len(fullName) != 2 {
		return fmt.Errorf("blogc: Invalid Full Name")
	}

	buildId := fmt.Sprintf("%s-%d", p.After, time.Now().Unix())

	outputDir := filepath.Join(baseDir, "builds", buildId)
	outputDirRelative := filepath.Join("..", "..", "builds", buildId)
	if _, err := os.Stat(outputDir); err == nil {
		outputDir += "-"
		outputDirRelative += "-"
	}

	cmd := exec.Command("blogc-make", "-f", blogcfile)
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("OUTPUT_DIR=%s", outputDir),
	)
	out, err := cmd.CombinedOutput()
	log.Printf("blogc: %s: Running command: OUTPUT_DIR='%s' blogc-make -f '%s'\n%s", p.Repo.FullName, outputDir, blogcfile, string(out))
	if err != nil {
		return err
	}

	sym := filepath.Join(baseDir, "htdocs", fullName[0], fullName[1])
	if symTarget, err := filepath.EvalSymlinks(sym); err == nil {
		os.RemoveAll(symTarget)
		os.Remove(sym)
	}

	symDir := filepath.Dir(sym)
	if _, err := os.Stat(symDir); os.IsNotExist(err) {
		os.MkdirAll(symDir, 0777)
	}

	log.Printf("blogc: %s: Creating symlink %s -> %s", p.Repo.FullName, sym, outputDirRelative)
	if err := os.Symlink(outputDirRelative, sym); err != nil {
		return err
	}

	return nil
}
