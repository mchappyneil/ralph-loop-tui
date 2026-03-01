package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// resolveRepoName returns a human-readable repo identifier using the chain:
// git remote repo name > cwd basename > "unknown".
func resolveRepoName() string {
	if name := detectRepoName(); name != "" {
		return name
	}
	if wd, err := os.Getwd(); err == nil {
		return filepath.Base(wd)
	}
	return "unknown"
}

// deriveInstanceID builds an instance identity from a repo name and optional epic.
// If epic is non-empty, it is appended as /epic (guarding against double-append).
func deriveInstanceID(repo, epic string) string {
	base := repo
	if base == "" {
		base = "unknown"
	}
	if epic != "" && !strings.HasSuffix(base, "/"+epic) {
		base += "/" + epic
	}
	return base
}

// detectRepoName runs git remote get-url origin and parses the result.
func detectRepoName() string {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return repoNameFromRemoteURL(strings.TrimSpace(string(out)))
}

// repoNameFromRemoteURL extracts "user/repo" from SSH or HTTPS git remote URLs.
func repoNameFromRemoteURL(remote string) string {
	if remote == "" {
		return ""
	}
	remote = strings.TrimSuffix(remote, ".git")

	// SSH format: git@github.com:user/repo
	if !strings.Contains(remote, "://") {
		if idx := strings.Index(remote, ":"); idx != -1 {
			return remote[idx+1:]
		}
		return ""
	}

	// HTTPS format: https://github.com/user/repo
	parts := strings.SplitN(remote, "://", 2)
	if len(parts) < 2 {
		return ""
	}
	// Strip host: "github.com/user/repo" → "/user/repo"
	path := parts[1]
	if idx := strings.Index(path, "/"); idx != -1 {
		return path[idx+1:]
	}
	return ""
}
