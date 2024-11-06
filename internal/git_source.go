package internal

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	log "github.com/sirupsen/logrus"
)

type GitSource struct {
	url        string
	repository *git.Repository
	worktree   *git.Worktree
	filesystem billy.Filesystem
}

func NewGitSource(url string) *GitSource {
	return &GitSource{
		url: url,
	}
}

func (s *GitSource) Initialize(ctx context.Context) error {
	storage := memory.NewStorage()
	tempDir, err := os.MkdirTemp("", "tuki-clone")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	filesystem := osfs.New(tempDir)
	s.filesystem = filesystem

	log.Info("Cloning repository ", s.url)

	repo, err := git.CloneContext(ctx, storage, filesystem, &git.CloneOptions{
		URL: s.url,
	})
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	s.repository = repo
	s.worktree = worktree
	return nil
}

func (s *GitSource) FetchUpdates(ctx context.Context) error {
	remote, err := s.repository.Remote("origin")
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

	err = s.worktree.Reset(&git.ResetOptions{
		Mode:   git.HardReset,
		Commit: firstNonZeroRef.Hash(),
	})
	if err != nil {
		return fmt.Errorf("failed to reset worktree: %w", err)
	}

	return nil
}

func (s *GitSource) PublishChanges(ctx context.Context) error {
	status, err := s.worktree.Status()
	if err != nil {
		return fmt.Errorf("error getting worktree status: %v", err)
	}

	if status.IsClean() {
		return ErrNoChanges
	}

	_, err = s.worktree.Commit("Update state file", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Tuki Bot",
			Email: "bot@tuki",
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("error committing changes: %v", err)
	}

	if err := s.repository.PushContext(ctx, &git.PushOptions{
		RemoteName: "origin",
	}); err != nil {
		return fmt.Errorf("error pushing to remote: %v", err)
	}

	return nil
}

func (s *GitSource) HasChanges() (bool, error) {
	status, err := s.worktree.Status()
	if err != nil {
		return false, fmt.Errorf("error getting worktree status: %v", err)
	}
	return !status.IsClean(), nil
}

func (s *GitSource) ReadFile(path string) ([]byte, error) {
	file, err := s.filesystem.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

func (s *GitSource) WriteFile(path string, data []byte) error {
	file, err := s.filesystem.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return err
	}

	if _, err := s.worktree.Add(path); err != nil {
		return fmt.Errorf("error adding file to repository: %v", err)
	}

	return nil
}

func (s *GitSource) ReadDir(path string) ([]fs.FileInfo, error) {
	return s.filesystem.ReadDir(path)
}

func (s *GitSource) Root() string {
	return s.filesystem.Root()
}

func (s *GitSource) Close() error {
	return os.RemoveAll(s.filesystem.Root())
}
