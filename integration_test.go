//go:build integration
// +build integration

package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"tuki/internal"

	"github.com/stretchr/testify/assert"
)

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

func must0(err error) {
	if err != nil {
		panic(err)
	}
}

func TestMain(t *testing.T) {
	// Setup
	bareRepoDir, repoDir := setupTestRepo(t, "test/testdata/basic-repo/")
	defer os.RemoveAll(bareRepoDir)
	defer os.RemoveAll(repoDir)

	// Run one tick of tuki
	runTuki(t, bareRepoDir)

	// Ensure that the state file was committed
	cmd := exec.Command("git", "log", "--patch", "-n", "1", "master")
	cmd.Dir = bareRepoDir
	output := must(cmd.Output())
	assert.Contains(t, string(output), "state.json")

	// Check that the state file has 2 tasks, one failed (fails.sh) and one completed (hello-world.sh)
	cmd = exec.Command("git", "show", "master:.tuki/state.jsonl")
	cmd.Dir = bareRepoDir
	output = must(cmd.Output())

	tasksStore := must(internal.LoadTaskStore(strings.NewReader(string(output))))
	task, ok := tasksStore.GetTask("fails.sh")
	assert.True(t, ok)
	assert.True(t, task.Status == internal.StatusFailed)

	task, ok = tasksStore.GetTask("hello-world.sh")
	assert.True(t, ok)
	assert.True(t, task.Status == internal.StatusCompleted)
}

func TestHarnessFile(t *testing.T) {
	// Setup
	bareRepoDir, repoDir := setupTestRepo(t, "test/testdata/python-harness-repo/")
	defer os.RemoveAll(bareRepoDir)
	defer os.RemoveAll(repoDir)

	// Run one tick of tuki
	runTuki(t, bareRepoDir)

	// Ensure that the state file was committed
	cmd := exec.Command("git", "log", "--patch", "-n", "1", "master")
	cmd.Dir = bareRepoDir
	output := must(cmd.Output())
	assert.Contains(t, string(output), "state.json")

	// Check that the state file has 2 tasks, one failed (fails.sh) and one completed (hello-world.sh)
	cmd = exec.Command("git", "show", "master:.tuki/state.jsonl")
	cmd.Dir = bareRepoDir
	output = must(cmd.Output())

	tasksStore := must(internal.LoadTaskStore(strings.NewReader(string(output))))
	task, ok := tasksStore.GetTask("fails.py")
	assert.True(t, ok)
	assert.True(t, task.Status == internal.StatusFailed)

	task, ok = tasksStore.GetTask("hello-world.py")
	assert.True(t, ok)
	assert.True(t, task.Status == internal.StatusCompleted)
}

func setupTestRepo(t *testing.T, repoPath string) (string, string) {
	bareRepoDir := must(os.MkdirTemp("", "tuki-testrepo-bare"))
	repoDir := must(os.MkdirTemp("", "tuki-testrepo"))

	cmd := exec.Command("cp", "-r", repoPath, repoDir)
	cmd.Dir = "."
	cmd.Stderr = os.Stderr
	must0(cmd.Run())

	cmd = exec.Command("git", "init")
	cmd.Dir = repoDir
	must0(cmd.Run())

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	must0(cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = repoDir
	must0(cmd.Run())

	cmd = exec.Command("git", "clone", "--bare", repoDir, bareRepoDir)
	must0(cmd.Run())

	cmd = exec.Command("git", "remote", "add", "origin", bareRepoDir)
	cmd.Dir = repoDir
	must0(cmd.Run())

	return bareRepoDir, repoDir
}

func runTuki(t *testing.T, bareRepoDir string) {
	os.Setenv("REPO_URL", bareRepoDir)
	defer os.Unsetenv("REPO_URL")

	os.Setenv("MAX_TICKS", "1")
	defer os.Unsetenv("MAX_TICKS")

	main()
}