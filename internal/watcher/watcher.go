package watcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jimohabdol/git-pr-watcher/internal/config"
	"github.com/jimohabdol/git-pr-watcher/internal/github"
	"github.com/jimohabdol/git-pr-watcher/internal/logger"
	"github.com/jimohabdol/git-pr-watcher/internal/notifier"
)

type PRWatcher struct {
	githubClient *github.Client
	notifier     *notifier.EmailNotifier
	config       *config.Config
	ctx          context.Context
	cancel       context.CancelFunc
}

type NotificationResult struct {
	ApprovalReminders int
	MergeReminders    int
	Escalations       int
	DraftOverdue      int
	Errors            []error
}

func NewPRWatcher(githubClient *github.Client, notifier *notifier.EmailNotifier, cfg *config.Config) *PRWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &PRWatcher{
		githubClient: githubClient,
		notifier:     notifier,
		config:       cfg,
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (w *PRWatcher) Close() {
	if w.cancel != nil {
		w.cancel()
	}
	if w.notifier != nil {
		w.notifier.Close()
	}
}

func (w *PRWatcher) processPR(pr *github.PullRequest) *NotificationResult {
	result := &NotificationResult{}
	age := time.Since(pr.CreatedAt)

	if pr.Draft {
		if age >= w.config.Rules.DraftTime {
			logger.Debug("Draft PR #%d is overdue (age: %v, threshold: %v)",
				pr.Number, age, w.config.Rules.DraftTime)

			if err := w.notifier.SendDraftOverdue(pr, age, w.config.Rules.DraftTime); err != nil {
				logger.Error("Failed to send draft overdue notification for PR #%d: %v", pr.Number, err)
				result.Errors = append(result.Errors, fmt.Errorf("draft overdue for PR #%d: %w", pr.Number, err))
			} else {
				result.DraftOverdue++
				logger.Info("Sent draft overdue notification for PR #%d", pr.Number)
			}
		}
		return result
	}

	if age >= w.config.Rules.MergeTime {
		logger.Debug("PR #%d needs escalation (age: %v, threshold: %v)",
			pr.Number, age, w.config.Rules.MergeTime)

		if err := w.notifier.SendEscalation(pr, age, w.config.Rules.MergeTime, w.config.Rules.EscalationEmail); err != nil {
			logger.Error("Failed to send escalation for PR #%d: %v", pr.Number, err)
			result.Errors = append(result.Errors, fmt.Errorf("escalation for PR #%d: %w", pr.Number, err))
		} else {
			result.Escalations++
			logger.Info("Sent escalation for PR #%d", pr.Number)
		}
		return result
	}

	// PRs without sufficient approvals need approval reminder
	if pr.ReviewCount < 2 && age >= w.config.Rules.ApprovalTime {
		logger.Debug("PR #%d needs approval reminder (age: %v, threshold: %v, reviews: %d)",
			pr.Number, age, w.config.Rules.ApprovalTime, pr.ReviewCount)

		if err := w.notifier.SendApprovalReminder(pr, age, w.config.Rules.ApprovalTime); err != nil {
			logger.Error("Failed to send approval reminder for PR #%d: %v", pr.Number, err)
			result.Errors = append(result.Errors, fmt.Errorf("approval reminder for PR #%d: %w", pr.Number, err))
		} else {
			result.ApprovalReminders++
			logger.Info("Sent approval reminder for PR #%d", pr.Number)
		}
		return result
	}

	// PRs with sufficient approvals need merge reminder
	if pr.ReviewCount >= 2 && age >= w.config.Rules.MergeReminderTime {
		logger.Debug("PR #%d needs merge reminder (age: %v, threshold: %v, reviews: %d)",
			pr.Number, age, w.config.Rules.MergeReminderTime, pr.ReviewCount)

		if err := w.notifier.SendMergeReminder(pr, age, w.config.Rules.MergeReminderTime); err != nil {
			logger.Error("Failed to send merge reminder for PR #%d: %v", pr.Number, err)
			result.Errors = append(result.Errors, fmt.Errorf("merge reminder for PR #%d: %w", pr.Number, err))
		} else {
			result.MergeReminders++
			logger.Info("Sent merge reminder for PR #%d", pr.Number)
		}
	}

	return result
}

func (w *PRWatcher) CheckPRs() error {
	logger.Info("Checking PRs for repositories: %v", w.config.GitHub.Repos)

	prs, err := w.githubClient.GetPullRequests(w.config.GitHub.Owner, w.config.GitHub.Repos)
	if err != nil {
		return fmt.Errorf("failed to fetch pull requests: %w", err)
	}

	logger.Info("Found %d open pull requests", len(prs))
	logger.Info("Processing %d open PRs (including drafts)", len(prs))

	concurrency := w.config.Debug.Concurrency
	if concurrency <= 0 {
		concurrency = 5
	}

	results := w.processPRsConcurrently(prs, concurrency)

	logger.Info("Completed processing: %d approval reminders, %d merge reminders, %d escalations, and %d draft overdue notifications sent",
		results.ApprovalReminders, results.MergeReminders, results.Escalations, results.DraftOverdue)

	if len(results.Errors) > 0 {
		logger.Error("Encountered %d errors during processing", len(results.Errors))
		for _, err := range results.Errors {
			logger.Error("Error: %v", err)
		}
	}

	return nil
}

func (w *PRWatcher) processPRsConcurrently(prs []*github.PullRequest, concurrency int) *NotificationResult {
	prChan := make(chan *github.PullRequest, len(prs))
	resultChan := make(chan *NotificationResult, len(prs))

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pr := range prChan {
				select {
				case <-w.ctx.Done():
					return
				default:
					resultChan <- w.processPR(pr)
				}
			}
		}()
	}

	go func() {
		defer close(prChan)
		for i, pr := range prs {
			logger.Progress("Processing PR %d/%d: #%d", i+1, len(prs), pr.Number)
			select {
			case prChan <- pr:
			case <-w.ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(resultChan)
		logger.ProgressEnd()
	}()

	totalResult := &NotificationResult{}
	for result := range resultChan {
		totalResult.ApprovalReminders += result.ApprovalReminders
		totalResult.MergeReminders += result.MergeReminders
		totalResult.Escalations += result.Escalations
		totalResult.DraftOverdue += result.DraftOverdue
		totalResult.Errors = append(totalResult.Errors, result.Errors...)
	}

	return totalResult
}

func (w *PRWatcher) CheckSpecificPR(repo string, prNumber int) error {
	logger.Info("Checking specific PR #%d in repository %s", prNumber, repo)

	pr, err := w.githubClient.GetPRDetails(w.config.GitHub.Owner, repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed to fetch PR details: %w", err)
	}

	result := w.processPR(pr)

	logger.Info("Completed processing PR #%d: %d approval reminders, %d merge reminders, %d escalations, and %d draft overdue notifications sent",
		prNumber, result.ApprovalReminders, result.MergeReminders, result.Escalations, result.DraftOverdue)

	if len(result.Errors) > 0 {
		return result.Errors[0]
	}

	return nil
}

func (w *PRWatcher) GetPRSummary() (*PRSummary, error) {
	prs, err := w.githubClient.GetPullRequests(w.config.GitHub.Owner, w.config.GitHub.Repos)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pull requests: %w", err)
	}

	summary := &PRSummary{
		TotalPRs:        len(prs),
		NeedsApproval:   0,
		NeedsEscalation: 0,
		Approved:        0,
		Draft:           0,
		PRs:             []PRStatus{},
	}

	now := time.Now()

	for _, pr := range prs {
		age := now.Sub(pr.CreatedAt)

		status := PRStatus{
			Number:      pr.Number,
			Title:       pr.Title,
			Repo:        pr.Repo,
			Author:      pr.User.Login,
			Age:         age,
			Approved:    pr.Approved,
			ReviewCount: pr.ReviewCount,
			URL:         pr.URL,
		}

		if pr.Draft {
			summary.Draft++
			if age >= w.config.Rules.DraftTime {
				status.Status = "Draft Overdue"
			} else {
				status.Status = "Draft"
			}
		} else if pr.ReviewCount >= 2 {
			summary.Approved++
			status.Status = "Approved"
		} else if age >= w.config.Rules.MergeTime {
			summary.NeedsEscalation++
			status.Status = "Needs Escalation"
		} else if age >= w.config.Rules.ApprovalTime {
			summary.NeedsApproval++
			status.Status = "Needs Approval"
		} else {
			status.Status = "OK"
		}

		summary.PRs = append(summary.PRs, status)
	}

	return summary, nil
}

type PRSummary struct {
	TotalPRs        int        `json:"total_prs"`
	NeedsApproval   int        `json:"needs_approval"`
	NeedsEscalation int        `json:"needs_escalation"`
	Approved        int        `json:"approved"`
	Draft           int        `json:"draft"`
	PRs             []PRStatus `json:"prs"`
}

type PRStatus struct {
	Number      int           `json:"number"`
	Title       string        `json:"title"`
	Repo        string        `json:"repo"`
	Author      string        `json:"author"`
	Age         time.Duration `json:"age"`
	Approved    bool          `json:"approved"`
	ReviewCount int           `json:"review_count"`
	Status      string        `json:"status"`
	URL         string        `json:"url"`
}
