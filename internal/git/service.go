package git

import (
	"context"
	"fmt"
	"os/exec"
)

// GitService defines the interface for Git operations
type GitService interface {
	Clone(ctx context.Context, url, dest string) error
	Pull(ctx context.Context, repoPath string) error
	Checkout(ctx context.Context, repoPath, ref string) error
}

// CommandExecutor defines the interface for executing shell commands
type CommandExecutor interface {
	Execute(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ShellCommandExecutor implements CommandExecutor using os/exec
type ShellCommandExecutor struct{}

// Execute runs a shell command with the given name and arguments
func (s *ShellCommandExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

// GitCommandService implements GitService using shell commands
type GitCommandService struct {
	executor CommandExecutor
}

// NewGitCommandService creates a new GitCommandService
func NewGitCommandService(executor CommandExecutor) GitService {
	if executor == nil {
		executor = &ShellCommandExecutor{}
	}
	return &GitCommandService{executor: executor}
}

// Clone clones a git repository
func (s *GitCommandService) Clone(ctx context.Context, url, dest string) error {
	output, err := s.executor.Execute(ctx, "git", "clone", url, dest)
	if err != nil {
		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, output)
	}
	return nil
}

// Pull updates a git repository
func (s *GitCommandService) Pull(ctx context.Context, repoPath string) error {
	output, err := s.executor.Execute(ctx, "git", "-C", repoPath, "pull")
	if err != nil {
		return fmt.Errorf("git pull failed: %w\nOutput: %s", err, output)
	}
	return nil
}

// Checkout checks out a specific reference in a git repository
func (s *GitCommandService) Checkout(ctx context.Context, repoPath, ref string) error {
	output, err := s.executor.Execute(ctx, "git", "-C", repoPath, "checkout", ref)
	if err != nil {
		return fmt.Errorf("git checkout failed: %w\nOutput: %s", err, output)
	}
	return nil
}
