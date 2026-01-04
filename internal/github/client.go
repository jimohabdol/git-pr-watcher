package github

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub client with additional functionality
type Client struct {
	client *github.Client
	ctx    context.Context
}

// PullRequest represents a GitHub pull request with additional metadata
type PullRequest struct {
	Number       int       `json:"number"`
	Title        string    `json:"title"`
	State        string    `json:"state"`
	Draft        bool      `json:"draft"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	User         *User     `json:"user"`
	Head         *Branch   `json:"head"`
	Base         *Branch   `json:"base"`
	URL          string    `json:"html_url"`
	Approved     bool      `json:"approved"`
	ReviewCount  int       `json:"review_count"`
	Repo         string    `json:"repo"`
	Additions    int       `json:"additions"`
	Deletions    int       `json:"deletions"`
	TotalChanges int       `json:"total_changes"`
	ChangedFiles int       `json:"changed_files"`
	SizeCategory string    `json:"size_category"` // XS, S, M, L, XL
}

// User represents a GitHub user
type User struct {
	Login string `json:"login"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// Branch represents a Git branch
type Branch struct {
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

func categorizePRSize(totalChanges int) string {
	switch {
	case totalChanges <= 50:
		return "XS"
	case totalChanges <= 200:
		return "S"
	case totalChanges <= 500:
		return "M"
	case totalChanges <= 1000:
		return "L"
	default:
		return "XL"
	}
}

// NewClient creates a new GitHub client
func NewClient(token string) (*Client, error) {
	if token == "" {
		return nil, fmt.Errorf("GitHub token is required")
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	return &Client{
		client: client,
		ctx:    ctx,
	}, nil
}

// GetPullRequests fetches all open pull requests for the given repositories
func (c *Client) GetPullRequests(owner string, repos []string) ([]*PullRequest, error) {
	var allPRs []*PullRequest

	for _, repo := range repos {
		prs, err := c.getPullRequestsForRepo(owner, repo)
		if err != nil {
			return nil, fmt.Errorf("failed to get PRs for repo %s: %w", repo, err)
		}
		allPRs = append(allPRs, prs...)
	}

	return allPRs, nil
}

// getPullRequestsForRepo fetches pull requests for a specific repository
func (c *Client) getPullRequestsForRepo(owner, repo string) ([]*PullRequest, error) {
	var prs []*PullRequest

	opts := &github.PullRequestListOptions{
		State: "open",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		githubPRs, resp, err := c.client.PullRequests.List(c.ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}

		for _, pr := range githubPRs {

			// Check if PR has approvals
			approved, reviewCount, err := c.checkPRApprovals(owner, repo, pr.GetNumber())
			if err != nil {
				// Log error but continue processing
				fmt.Printf("Warning: failed to check approvals for PR #%d: %v\n", pr.GetNumber(), err)
			}

			additions := pr.GetAdditions()
			deletions := pr.GetDeletions()
			totalChanges := additions + deletions

			prData := &PullRequest{
				Number:    pr.GetNumber(),
				Title:     pr.GetTitle(),
				State:     pr.GetState(),
				Draft:     pr.GetDraft(),
				CreatedAt: pr.GetCreatedAt().Time,
				UpdatedAt: pr.GetUpdatedAt().Time,
				User: &User{
					Login: pr.User.GetLogin(),
					Email: pr.User.GetEmail(),
					Name:  pr.User.GetName(),
				},
				Head: &Branch{
					Ref: pr.Head.GetRef(),
					SHA: pr.Head.GetSHA(),
				},
				Base: &Branch{
					Ref: pr.Base.GetRef(),
					SHA: pr.Base.GetSHA(),
				},
				URL:          pr.GetHTMLURL(),
				Approved:     approved,
				ReviewCount:  reviewCount,
				Repo:         repo,
				Additions:    additions,
				Deletions:    deletions,
				TotalChanges: totalChanges,
				ChangedFiles: pr.GetChangedFiles(),
				SizeCategory: categorizePRSize(totalChanges),
			}

			prs = append(prs, prData)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return prs, nil
}

// checkPRApprovals checks if a PR has been approved
func (c *Client) checkPRApprovals(owner, repo string, prNumber int) (bool, int, error) {
	opts := &github.ListOptions{
		PerPage: 100,
	}

	reviews, _, err := c.client.PullRequests.ListReviews(c.ctx, owner, repo, prNumber, opts)
	if err != nil {
		return false, 0, err
	}

	approved := false
	reviewCount := 0

	for _, review := range reviews {
		if review.GetState() == "APPROVED" {
			approved = true
		}
		if review.GetState() != "COMMENTED" {
			reviewCount++
		}
	}

	return approved, reviewCount, nil
}

// GetPRDetails fetches detailed information about a specific PR
func (c *Client) GetPRDetails(owner, repo string, prNumber int) (*PullRequest, error) {
	pr, _, err := c.client.PullRequests.Get(c.ctx, owner, repo, prNumber)
	if err != nil {
		return nil, err
	}

	approved, reviewCount, err := c.checkPRApprovals(owner, repo, prNumber)
	if err != nil {
		return nil, err
	}

	additions := pr.GetAdditions()
	deletions := pr.GetDeletions()
	totalChanges := additions + deletions

	return &PullRequest{
		Number:    pr.GetNumber(),
		Title:     pr.GetTitle(),
		State:     pr.GetState(),
		Draft:     pr.GetDraft(),
		CreatedAt: pr.GetCreatedAt().Time,
		UpdatedAt: pr.GetUpdatedAt().Time,
		User: &User{
			Login: pr.User.GetLogin(),
			Email: pr.User.GetEmail(),
			Name:  pr.User.GetName(),
		},
		Head: &Branch{
			Ref: pr.Head.GetRef(),
			SHA: pr.Head.GetSHA(),
		},
		Base: &Branch{
			Ref: pr.Base.GetRef(),
			SHA: pr.Base.GetSHA(),
		},
		URL:          pr.GetHTMLURL(),
		Approved:     approved,
		ReviewCount:  reviewCount,
		Repo:         repo,
		Additions:    additions,
		Deletions:    deletions,
		TotalChanges: totalChanges,
		ChangedFiles: pr.GetChangedFiles(),
		SizeCategory: categorizePRSize(totalChanges),
	}, nil
}
