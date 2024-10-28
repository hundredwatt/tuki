package internal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

type Task struct {
	Name         string     `json:"name"`
	Status       TaskStatus `json:"status"`
	LockedBy     string     `json:"locked_by,omitempty"`
	InProgressAt time.Time  `json:"in_progress_at,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
}

type TaskStore struct {
	Tasks       []Task
	IndexOnName map[string]int
}

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in_progress"
	StatusCompleted  TaskStatus = "completed"
	StatusFailed     TaskStatus = "failed"
)

// MarshalJSON implements the json.Marshaler interface.
func (s TaskStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (s *TaskStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	*s = TaskStatus(str)
	return nil
}

func NewTaskStore() *TaskStore {
	return &TaskStore{
		Tasks:       make([]Task, 0),
		IndexOnName: map[string]int{},
	}
}

func LoadTaskStore(reader io.Reader) (*TaskStore, error) {
	taskStore := NewTaskStore()
	index := 0

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		var task Task
		if err := json.Unmarshal(scanner.Bytes(), &task); err != nil {
			return nil, fmt.Errorf("error unmarshaling task: %v", err)
		}
		taskStore.Tasks = append(taskStore.Tasks, task)
		taskStore.IndexOnName[task.Name] = index
		index++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading state file: %v", err)
	}

	return taskStore, nil
}

func (ts *TaskStore) GetOrCreateTask(name string) Task {
	if task, ok := ts.GetTask(name); ok {
		return task
	}
	return Task{Name: name}
}

func (ts *TaskStore) GetTask(name string) (Task, bool) {
	if index, ok := ts.IndexOnName[name]; ok {
		return ts.Tasks[index], true
	}
	return Task{}, false
}

func (ts *TaskStore) UpsertTask(task Task) {
	if index, ok := ts.IndexOnName[task.Name]; ok {
		ts.Tasks[index] = task
	} else {
		ts.Tasks = append(ts.Tasks, task)
		ts.IndexOnName[task.Name] = len(ts.Tasks) - 1
	}
}

func (ts *TaskStore) SaveToFile(writer io.Writer) error {
	for _, task := range ts.Tasks {
		json.NewEncoder(writer).Encode(task)
	}

	return nil
}
