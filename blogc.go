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
	blogcZeroconf := ""
	if _, err := os.Stat(blogcfile); os.IsNotExist(err) {
		bz, err := exec.LookPath("blogc-zeroconf")
		if err != nil {
			return fmt.Errorf("blogc: %s: blogcfile not found and blogc-zeroconf not installed", p.Repo.FullName)
		}
		log.Printf("blogc: %s: blogcfile not found, will try to use blogc-zeroconf", p.Repo.FullName)
		blogcZeroconf = bz
	}

	buildId := fmt.Sprintf("%s-%d", p.After, time.Now().Unix())

	outputDir := filepath.Join(baseDir, "builds", buildId)
	outputDirRelative := filepath.Join("..", "..", "builds", buildId)
	if _, err := os.Stat(outputDir); err == nil {
		outputDir += "-"
		outputDirRelative += "-"
	}

	var cmd *exec.Cmd
	cmdStr := ""
	if blogcZeroconf != "" {
		cmd = exec.Command(blogcZeroconf)
		cmdStr = fmt.Sprintf("OUTPUT_DIR=%q %s", outputDir, blogcZeroconf)
	} else {
		cmd = exec.Command("blogc-make", "-f", blogcfile)
		cmdStr = fmt.Sprintf("OUTPUT_DIR=%q blogc-make -f %q", outputDir, blogcfile)
	}
	cmd.Dir = tempDir
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("OUTPUT_DIR=%s", outputDir),
	)
	out, err := cmd.CombinedOutput()
	log.Printf("blogc: %s: Running command: %s\n%s", p.Repo.FullName, cmdStr, string(out))
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
