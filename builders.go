package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func command(name string, arg ...string) *exec.Cmd {
	cmd := func() *exec.Cmd {
		if p, err := exec.LookPath("telegram-notify"); err == nil {
			_, f1 := os.LookupEnv("TELEGRAM_NOTIFY_TOKEN")
			_, f2 := os.LookupEnv("TELEGRAM_NOFITY_CHAT_ID")
			if f1 && f2 {
				// todo: make the telegram-notify arguments configurable
				args := []string{
					"-id=blogc-github-webhook",
					"-success",
					"--",
					name,
				}
				return exec.Command(p, append(args, arg...)...)
			}
		}
		return exec.Command(name, arg...)
	}()

	// make sure that we are inheriting the default environment
	cmd.Env = os.Environ()
	return cmd
}

type builder interface {
	getBinary() string
	getCommand(inputDir string, outputDir string) string
	lookup(inputDir string) bool
	build(inputDir string, outputDir string) ([]byte, error)
}

type builderBlogcMake struct {
	blogcfile string
}

func (b *builderBlogcMake) getBinary() string {
	return "blogc-make"
}

func (b *builderBlogcMake) getCommand(inputDir string, outputDir string) string {
	return fmt.Sprintf("OUTPUT_DIR=%q %s --file %q", outputDir, b.getBinary(), b.blogcfile)
}

func (b *builderBlogcMake) lookup(inputDir string) bool {
	b.blogcfile = filepath.Join(inputDir, "blogcfile")
	_, err := os.Stat(b.blogcfile)
	return err == nil
}

func (b *builderBlogcMake) build(inputDir string, outputDir string) ([]byte, error) {
	cmd := command(b.getBinary(), "--file", b.blogcfile)
	cmd.Dir = inputDir
	cmd.Env = append(
		cmd.Env,
		fmt.Sprintf("OUTPUT_DIR=%s", outputDir),
	)
	return cmd.CombinedOutput()
}

type builderMake struct {
	makefile string
}

func (b *builderMake) getBinary() string {
	return "make"
}

func (b *builderMake) getCommand(inputDir string, outputDir string) string {
	return fmt.Sprintf("OUTPUT_DIR=%q %s -f %q blogc-github-webhook", outputDir, b.getBinary(), b.makefile)
}

func (b *builderMake) lookup(inputDir string) bool {
	b.makefile = filepath.Join(inputDir, "Makefile")
	if _, err := os.Stat(b.makefile); err != nil {
		return false
	}
	cmd := exec.Command(b.getBinary(), "--dry-run", "--file", b.makefile, "blogc-github-webhook")
	cmd.Dir = inputDir
	return cmd.Run() == nil
}

func (b *builderMake) build(inputDir string, outputDir string) ([]byte, error) {
	cmd := command(b.getBinary(), "-f", b.makefile, "blogc-github-webhook")
	cmd.Dir = inputDir
	cmd.Env = append(
		cmd.Env,
		fmt.Sprintf("OUTPUT_DIR=%s", outputDir),
	)
	return cmd.CombinedOutput()
}

type builderBlogcZeroconf struct{}

func (b *builderBlogcZeroconf) getBinary() string {
	return "blogc-zeroconf"
}

func (b *builderBlogcZeroconf) getCommand(inputDir string, outputDir string) string {
	return fmt.Sprintf("OUTPUT_DIR=%q %s", outputDir, b.getBinary())
}

func (b *builderBlogcZeroconf) lookup(inputDir string) bool {
	// blogc-zeroconf will (at least try to) build anything
	return true
}

func (b *builderBlogcZeroconf) build(inputDir string, outputDir string) ([]byte, error) {
	cmd := command(b.getBinary())
	cmd.Dir = inputDir
	cmd.Env = append(
		cmd.Env,
		fmt.Sprintf("OUTPUT_DIR=%s", outputDir),
	)
	return cmd.CombinedOutput()
}

var (
	builders = []builder{
		&builderBlogcMake{},
		&builderMake{},
		&builderBlogcZeroconf{},
	}
)

func builderRun(inputDir string, baseDir string, p *payload) error {
	var supported builder
	for _, b := range builders {
		if _, err := exec.LookPath(b.getBinary()); err != nil {
			continue
		}

		if !b.lookup(inputDir) {
			continue
		}

		supported = b
		break
	}

	if supported == nil {
		return fmt.Errorf("builders: no builder supported")
	}

	buildId := fmt.Sprintf("%s-%d", p.After, time.Now().Unix())
	outputDir := filepath.Join(baseDir, "builds", buildId)
	outputDirRelative := filepath.Join("..", "..", "builds", buildId)
	if _, err := os.Stat(outputDir); err == nil {
		outputDir += "-"
		outputDirRelative += "-"
	}

	out, err := supported.build(inputDir, outputDir)
	log.Printf(
		"%s: %s: Running command: %s\n%s",
		supported.getBinary(),
		p.Repo.FullName,
		supported.getCommand(inputDir, outputDir),
		string(out),
	)
	if err != nil {
		return err
	}

	sym := builderCleanup(baseDir, p)
	symDir := filepath.Dir(sym)
	if _, err := os.Stat(symDir); os.IsNotExist(err) {
		os.MkdirAll(symDir, 0777)
	}

	log.Printf(
		"%s: %s: Creating symlink %s -> %s",
		supported.getBinary(),
		p.Repo.FullName,
		sym,
		outputDirRelative,
	)
	if err := os.Symlink(outputDirRelative, sym); err != nil {
		return err
	}

	return nil
}

func builderCleanup(baseDir string, p *payload) string {
	sym := filepath.Join(
		baseDir,
		"htdocs",
		p.Repo.Owner.Login,
		fmt.Sprintf("%s--%s", p.Repo.Name, p.getBranch()),
	)
	if symTarget, err := filepath.EvalSymlinks(sym); err == nil {
		os.RemoveAll(symTarget)
		os.Remove(sym)
	}
	return sym
}
