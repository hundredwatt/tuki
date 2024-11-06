package internal

import (
	"context"
	"errors"
	"io/fs"
)

// Common errors
var (
	ErrNotFound  = errors.New("not found")
	ErrConflict  = errors.New("conflict")
	ErrNoChanges = errors.New("no changes")
)

// Source represents a syncable file source
type Source interface {
	// Initialize sets up the source
	Initialize(ctx context.Context) error

	// FetchUpdates downloads changes from the remote source
	FetchUpdates(ctx context.Context) error

	// PublishChanges uploads local changes to the remote source
	PublishChanges(ctx context.Context) error

	// HasChanges checks if there are local modifications
	HasChanges() (bool, error)

	// File operations
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte) error
	ReadDir(path string) ([]fs.FileInfo, error)

	// Root returns the root path of the workspace
	Root() string

	// Close cleans up any resources
	Close() error
} 