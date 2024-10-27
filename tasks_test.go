package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddAndUpdateTaskToTaskStore(t *testing.T) {
	taskStore := NewTaskStore()
	taskStore.UpsertTask(Task{Name: "test", Status: StatusPending})
	taskStore.UpsertTask(Task{Name: "test2", Status: StatusPending})
	taskStore.UpsertTask(Task{Name: "test", Status: StatusCompleted})

	assert.Len(t, taskStore.Tasks, 2)
	assert.Len(t, taskStore.IndexOnName, 2)

	assert.Equal(t, taskStore.Tasks[0].Name, "test")
	assert.Equal(t, taskStore.Tasks[0].Status, StatusCompleted)
	assert.Equal(t, taskStore.Tasks[1].Name, "test2")
	assert.Equal(t, taskStore.Tasks[1].Status, StatusPending)
}

func TestSaveAndLoadStateFile(t *testing.T) {
	taskStore := NewTaskStore()
	taskStore.UpsertTask(Task{Name: "test", Status: StatusPending})
	taskStore.UpsertTask(Task{Name: "test2", Status: StatusCompleted})

	tmpFile, err := os.CreateTemp("", "teststatefile")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	taskStore.SaveToFile(tmpFile)
	tmpFile.Close()

	// Re-open the file
	tmpFile, err = os.Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open temp file: %v", err)
	}

	loadedTaskStore, err := LoadTaskStore(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load task store: %v", err)
	}

	assert.Equal(t, taskStore.Tasks, loadedTaskStore.Tasks)
}
