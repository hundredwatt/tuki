package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

	"tuki/internal"

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

	Source    internal.Source
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
			if sig == syscall.SIGUSR1 {
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

	if config.TickIntervalSeconds >= 0 {
		firstTick = time.After(0) // Immediate first tick
		ticker = time.Tick(time.Duration(config.TickIntervalSeconds) * time.Second)
	} else {
		firstTick = make(chan time.Time) // A nil channel that will never receive
		ticker = make(chan time.Time)    // A nil channel that will never receive
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
	source := internal.NewGitSource(config.RepoURL)
	state.Source = source

	return executeStep(ctx, "Initialize source", source.Initialize)
}

func run(ctx context.Context) error {
	log.Info("Running tick ", state.RunNumber)

	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"Fetch updates", fetchUpdates},
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
	file, err := state.Source.ReadFile(config.StateFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if os.IsNotExist(err) {
		state.TaskStore = internal.NewTaskStore()
		return nil
	}

	state.TaskStore, err = internal.LoadTaskStore(bytes.NewReader(file))
	return err
}

func fetchUpdates(ctx context.Context) error {
	return state.Source.FetchUpdates(ctx)
}

func updateTasksState(ctx context.Context) error {
	// Read tasks from repository
	dir, err := state.Source.ReadDir(config.ScriptsDir)
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

				file, err := state.Source.ReadFile(path.Join(config.ScriptsDir, task.Name))
				if err != nil {
					handleTaskError(task, state, err, "Failed to open file")
					continue
				}

				log.Info("Running task ", task.Name)
				cmd := exec.Command(taskRunnerCommand[0], taskRunnerCommand[1:]...)
				cmd.Dir = state.Source.Root()
				cmd.Stdin = strings.NewReader(string(file))
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
	var buf bytes.Buffer
	if err := state.TaskStore.SaveToFile(&buf); err != nil {
		return fmt.Errorf("error serializing state: %v", err)
	}

	if err := state.Source.WriteFile(config.StateFile, buf.Bytes()); err != nil {
		return fmt.Errorf("error writing state file: %v", err)
	}

	if err := state.Source.PublishChanges(ctx); err != nil {
		if err == internal.ErrNoChanges {
			return nil
		}
		return fmt.Errorf("error publishing changes: %v", err)
	}

	return nil
}

func determineTaskRunnerCommand() ([]string, error) {
	harnessFilePath := path.Join(config.ScriptsDir, config.HarnessFile)
	if _, err := state.Source.ReadFile(harnessFilePath); os.IsNotExist(err) {
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
