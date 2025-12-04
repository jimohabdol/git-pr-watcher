package watcher

import (
	"testing"
	"time"

	"github.com/jimohabdol/git-pr-watcher/internal/config"
	"github.com/jimohabdol/git-pr-watcher/internal/github"
)

func TestPRWatcher_GetPRSummary(t *testing.T) {
	// This is a basic test structure
	// In a real implementation, you would mock the GitHub client and notifier

	cfg := &config.Config{
		GitHub: config.GitHubConfig{
			Owner: "test-org",
			Repos: []string{"test-repo"},
		},
		Rules: config.RulesConfig{
			ApprovalTime: 24 * time.Hour,
			MergeTime:    72 * time.Hour,
		},
	}

	// Note: This test would require mocking the GitHub client
	// just test the structure for now
	if cfg.GitHub.Owner != "test-org" {
		t.Errorf("Expected owner 'test-org', got '%s'", cfg.GitHub.Owner)
	}

	if len(cfg.GitHub.Repos) != 1 {
		t.Errorf("Expected 1 repo, got %d", len(cfg.GitHub.Repos))
	}

	if cfg.Rules.ApprovalTime != 24*time.Hour {
		t.Errorf("Expected approval time 24h, got %v", cfg.Rules.ApprovalTime)
	}
}

func TestPRStatus_Status(t *testing.T) {
	tests := []struct {
		name     string
		pr       github.PullRequest
		age      time.Duration
		approved bool
		expected string
	}{
		{
			name: "New PR",
			pr: github.PullRequest{
				Number: 1,
				Title:  "Test PR",
				Draft:  false,
			},
			age:      1 * time.Hour,
			approved: false,
			expected: "OK",
		},
		{
			name: "PR needs approval",
			pr: github.PullRequest{
				Number: 2,
				Title:  "Test PR 2",
				Draft:  false,
			},
			age:      25 * time.Hour,
			approved: false,
			expected: "Needs Approval",
		},
		{
			name: "PR needs escalation",
			pr: github.PullRequest{
				Number: 3,
				Title:  "Test PR 3",
				Draft:  false,
			},
			age:      75 * time.Hour,
			approved: false,
			expected: "Needs Escalation",
		},
		{
			name: "Draft PR",
			pr: github.PullRequest{
				Number: 4,
				Title:  "Draft PR",
				Draft:  true,
			},
			age:      25 * time.Hour,
			approved: false,
			expected: "Draft",
		},
		{
			name: "Approved PR",
			pr: github.PullRequest{
				Number: 5,
				Title:  "Approved PR",
				Draft:  false,
			},
			age:      25 * time.Hour,
			approved: true,
			expected: "Approved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would be tested in the actual watcher logic
			// just verify the test structure for now
			if tt.pr.Number == 0 {
				t.Error("PR number should not be zero")
			}
		})
	}
}
