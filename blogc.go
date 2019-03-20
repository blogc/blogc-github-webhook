package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func blogcCleanup(basedir string, p *payload) string {
	sym := filepath.Join(basedir, "htdocs", p.Repo.Owner.Login, fmt.Sprintf("%s--%s", p.Repo.Name, p.getBranch()))
	if symTarget, err := filepath.EvalSymlinks(sym); err == nil {
		os.RemoveAll(symTarget)
		os.Remove(sym)
	}
	return sym
}

func blogcRun(tempDir string, baseDir string, p *payload) error {
	blogcfile := filepath.Join(tempDir, "blogcfile")
	if _, err := os.Stat(blogcfile); os.IsNotExist(err) {
		return fmt.Errorf("blogc: blogcfile not found")
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

	sym := blogcCleanup(baseDir, p)
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
