package main

import "testing"

func TestDeriveInstanceID_ExplicitOverride(t *testing.T) {
	got := deriveInstanceID("my-custom-id", "")
	if got != "my-custom-id" {
		t.Errorf("expected %q, got %q", "my-custom-id", got)
	}
}

func TestDeriveInstanceID_FallbackToDirectory(t *testing.T) {
	got := deriveInstanceID("", "")
	if got == "" {
		t.Error("expected non-empty instance ID from directory/repo fallback")
	}
}

func TestDeriveInstanceID_WithEpic(t *testing.T) {
	got := deriveInstanceID("my-app", "BD-42")
	if got != "my-app/BD-42" {
		t.Errorf("expected %q, got %q", "my-app/BD-42", got)
	}
}

func TestRepoNameFromGitRemote(t *testing.T) {
	tests := []struct {
		name   string
		remote string
		want   string
	}{
		{"SSH URL", "git@github.com:user/repo.git", "user/repo"},
		{"HTTPS with .git", "https://github.com/user/repo.git", "user/repo"},
		{"HTTPS without .git", "https://github.com/user/repo", "user/repo"},
		{"empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repoNameFromRemoteURL(tt.remote)
			if got != tt.want {
				t.Errorf("repoNameFromRemoteURL(%q) = %q, want %q", tt.remote, got, tt.want)
			}
		})
	}
}
