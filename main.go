package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

	"tuki/internal"

	"github.com/go-git/go-billy/v5/osfs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	log "github.com/sirupsen/logrus"
)

const (
	DefaultTaskRunnerCommand = "env sh"
)

var (
	config *internal.Config
	state  *State
)

type State struct {
	WorkerUUID string
	RunNumber  int

	Repository *git.Repository
	Worktree   *git.Worktree

	TaskStore *internal.TaskStore
}

func main() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	config = internal.LoadConfig()
	state = NewState()

	if config.Verbose {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manualRunSignal := make(chan struct{})

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
		for {
			sig := <-sigCh
			if sig == syscall.SIGUSR1 && config.Mode == "Manual" {
				log.Info("Received manual run signal (USR1).")
				manualRunSignal <- struct{}{}
			} else {
				log.Info("Received shutdown signal. Cancelling operations...")
				cancel()
				return
			}
		}
	}()

	if err := setup(ctx); err != nil {
		log.Fatal("Setup Error: ", err)
	}

	// Channels for time based triggering in Periodic mode
	var firstTick <-chan time.Time
	var ticker <-chan time.Time

	if config.Mode == "Manual" {
		firstTick = make(chan time.Time) // A nil channel that will never receive
		ticker = make(chan time.Time)    // A nil channel that will never receive
	} else {
		firstTick = time.After(0) // Immediate first tick
		ticker = time.Tick(time.Duration(config.TickIntervalSeconds) * time.Second)
	}

	run := func() {
		if err := run(ctx); err != nil {
			log.Fatal("Run Error: ", err)
		}

		if config.MaxTicks >= 0 && state.RunNumber >= config.MaxTicks {
			cancel()
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-firstTick:
			run()
		case <-ticker:
			run()
		case <-manualRunSignal:
			run()
		}
	}
}

func NewState() *State {
	return &State{
		WorkerUUID: uuid.NewString(),
		RunNumber:  1,
	}
}

func setup(ctx context.Context) error {
	return executeStep(ctx, "Clone repository", cloneRepository)
}

func run(ctx context.Context) error {
	log.Info("Running tick ", state.RunNumber)

	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"Fetch repository", fetchRepository},
		{"Read state file", readStateFile},
		{"Update tasks state", updateTasksState},
		{"Process tasks", processTasks},
		{"Update state file", persistStateFile},
	}

	for _, step := range steps {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := executeStep(ctx, step.name, step.fn); err != nil {
				return fmt.Errorf("failed to %s: %w", step.name, err)
			}
		}
	}

	state.RunNumber++

	return nil
}

func executeStep(ctx context.Context, name string, fn func(context.Context) error) error {
	log.Debug("Executing step: ", name)
	err := fn(ctx)
	if err != nil {
		log.WithError(err).Error("Step '", name, "' failed")
	} else {
		log.Debug("Step '", name, "' completed successfully")
	}
	return err
}

func readStateFile(ctx context.Context) error {
	file, err := state.Worktree.Filesystem.Open(path.Join(config.ScriptsDir, config.StateFile))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if os.IsNotExist(err) {
		state.TaskStore = internal.NewTaskStore()
		return nil
	}

	state.TaskStore, err = internal.LoadTaskStore(file)
	return err
}

func cloneRepository(ctx context.Context) error {
	storage := memory.NewStorage()
	tempDir, err := os.MkdirTemp("", "tuki-clone")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	filesystem := osfs.New(tempDir)

	log.Info("Cloning repository ", config.RepoURL)

	repo, err := git.CloneContext(ctx, storage, filesystem, &git.CloneOptions{
		URL: config.RepoURL,
	})
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	state.Repository = repo
	state.Worktree = worktree
	return nil
}

func fetchRepository(ctx context.Context) error {
	remote, err := state.Repository.Remote("origin")
	if err != nil {
		return fmt.Errorf("failed to get remote: %w", err)
	}
	err = remote.FetchContext(ctx, &git.FetchOptions{
		Force: true,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to fetch repository: %w", err)
	}
	if err == git.NoErrAlreadyUpToDate {
		log.Info("Repository is already up to date")
		return nil
	}

	refs, err := remote.List(&git.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list references: %w", err)
	}
	var firstNonZeroRef *plumbing.Reference
	for _, ref := range refs {
		if !ref.Hash().IsZero() {
			firstNonZeroRef = ref
			break
		}
	}
	if firstNonZeroRef == nil {
		return fmt.Errorf("no non-zero references found")
	}

	err = state.Worktree.Reset(&git.ResetOptions{
		Mode:   git.HardReset,
		Commit: firstNonZeroRef.Hash(),
	})
	if err != nil {
		return fmt.Errorf("failed to reset worktree: %w", err)
	}

	return nil
}

func updateTasksState(ctx context.Context) error {
	// Read tasks from repository
	dir, err := state.Worktree.Filesystem.ReadDir(config.ScriptsDir)
	if err != nil {
		return err
	}

	for _, file := range dir {
		if file.IsDir() {
			continue
		}

		task := state.TaskStore.GetOrCreateTask(file.Name())

		if task.Name == "" {
			task.Name = file.Name()
		}

		if task.Status == "" {
			task.Status = internal.StatusPending
		}

		state.TaskStore.UpsertTask(task)
	}

	return nil
}

func handleTaskError(task internal.Task, state *State, err error, errMsg string) {
	log.WithError(err).Error("Task ", task.Name, " failed")
	task.Status = internal.StatusFailed
	task.ErrorMessage = fmt.Sprintf("%s: %v", errMsg, err)
	state.TaskStore.UpsertTask(task)
}

func processTasks(ctx context.Context) error {
	taskRunnerCommand, err := determineTaskRunnerCommand()
	if err != nil {
		return err
	}

	for _, task := range state.TaskStore.Tasks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			switch task.Status {
			case internal.StatusPending:
				// Run
				task := task
				task.Status = internal.StatusInProgress
				task.LockedBy = state.WorkerUUID
				task.InProgressAt = time.Now()
				state.TaskStore.UpsertTask(task)

				file, err := state.Worktree.Filesystem.Open(path.Join(config.ScriptsDir, task.Name))
				if err != nil {
					handleTaskError(task, state, err, "Failed to open file")
					continue
				}
				defer file.Close()

				contents, err := io.ReadAll(file)
				if err != nil {
					handleTaskError(task, state, err, "Failed to read file")
					continue
				}

				log.Info("Running task ", task.Name)
				cmd := exec.Command(taskRunnerCommand[0], taskRunnerCommand[1:]...)
				cmd.Dir = state.Worktree.Filesystem.Root()
				cmd.Stdin = strings.NewReader(string(contents))
				cmd.Stderr = &prefixWriter{prefix: task.Name, level: "ERROR"}
				cmd.Stdout = &prefixWriter{prefix: task.Name, level: "INFO"}
				if err := cmd.Run(); err != nil {
					handleTaskError(task, state, err, "Failed to run command")
					continue
				}

				task.Status = internal.StatusCompleted
				state.TaskStore.UpsertTask(task)
			case internal.StatusInProgress:
				if time.Since(task.InProgressAt) > time.Duration(config.InProgressTimeoutMinutes)*time.Minute {
					log.Warn("Task ", task.Name, " in progress for more than ", config.InProgressTimeoutMinutes, " minutes. Marking as failed.")

					task.Status = internal.StatusFailed
					task.ErrorMessage = "Task in progress for more than an hour"
					state.TaskStore.UpsertTask(task)
				}
			case internal.StatusCompleted:
			case internal.StatusFailed:
				continue
			}
		}
	}

	return nil
}

func persistStateFile(ctx context.Context) error {
	file, err := state.Worktree.Filesystem.OpenFile("/"+config.StateFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("error opening state file: %v", err)
	}
	defer file.Close()

	err = state.TaskStore.SaveToFile(file)
	if err != nil {
		return fmt.Errorf("error saving state file: %v", err)
	}

	status, err := state.Worktree.Status()
	if err != nil {
		return fmt.Errorf("error getting worktree status: %v", err)
	}

	if status.IsClean() {
		return nil // No changes to commit
	}

	if _, err := state.Worktree.Add(config.StateFile); err != nil {
		return fmt.Errorf("error adding state file to repository: %v", err)
	}

	_, err = state.Worktree.Commit("Update state file", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Tuki Bot",
			Email: "bot@tuki",
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("error committing state file: %v", err)
	}

	if err := state.Repository.PushContext(ctx, &git.PushOptions{
		RemoteName: "origin",
	}); err != nil {
		return fmt.Errorf("error pushing to remote: %v", err)
	}

	return nil
}

func determineTaskRunnerCommand() ([]string, error) {
	harnessFilePath := path.Join(config.ScriptsDir, config.HarnessFile)
	if _, err := state.Worktree.Filesystem.Stat(harnessFilePath); os.IsNotExist(err) {
		return strings.Split(DefaultTaskRunnerCommand, " "), nil
	}

	return []string{"./" + config.HarnessFile}, nil
}

type prefixWriter struct {
	prefix string
	level  string
}

func (w *prefixWriter) Write(p []byte) (n int, err error) {
	if w.level == "ERROR" {
		log.WithField("task", w.prefix).Error(string(p))
	} else {
		log.WithField("task", w.prefix).Info(string(p))
	}
	return len(p), nil
}
