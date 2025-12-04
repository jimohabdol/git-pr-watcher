package notifier

import (
	"crypto/tls"
	"fmt"
	"html/template"
	"strings"
	"sync"
	"time"

	"github.com/jimohabdol/git-pr-watcher/internal/config"
	"github.com/jimohabdol/git-pr-watcher/internal/github"
	"gopkg.in/gomail.v2"
)

var (
	emailTemplate *template.Template
	templateOnce  sync.Once
)

// getEmailTemplate returns a singleton email template
func getEmailTemplate() *template.Template {
	templateOnce.Do(func() {
		tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>{{.Title}}</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: {{.HeaderColor}}; color: white; padding: 15px; border-radius: 5px; margin-bottom: 20px; }
        .pr-info { background-color: #f8f9fa; padding: 15px; border-radius: 5px; margin: 15px 0; }
        .pr-title { font-size: 18px; font-weight: bold; margin-bottom: 10px; }
        .pr-details { margin: 10px 0; }
        .pr-details strong { color: #555; }
        .age-info { background-color: {{.AgeColor}}; padding: 10px; border-radius: 3px; margin: 10px 0; }
        .action-required { background-color: #fff3cd; border: 1px solid #ffeaa7; padding: 15px; border-radius: 5px; margin: 15px 0; }
        .footer { margin-top: 30px; padding-top: 20px; border-top: 1px solid #eee; font-size: 12px; color: #666; }
        .button { display: inline-block; background-color: #007bff; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px; margin: 10px 0; }
        .button:hover { background-color: #0056b3; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2>{{.Title}}</h2>
        </div>
        
        <div class="pr-info">
            <div class="pr-title">{{.PullRequest.Title}}</div>
            <div class="pr-details">
                <strong>Repository:</strong> {{.PullRequest.Repo}}<br>
                <strong>Author:</strong> {{.PullRequest.User.Login}}<br>
                <strong>Branch:</strong> {{.PullRequest.Head.Ref}} → {{.PullRequest.Base.Ref}}<br>
                <strong>Created:</strong> {{.PullRequest.CreatedAt.Format "2006-01-02 15:04:05"}}<br>
                <strong>Last Updated:</strong> {{.PullRequest.UpdatedAt.Format "2006-01-02 15:04:05"}}<br>
                <strong>Reviews:</strong> {{.PullRequest.ReviewCount}} ({{if .PullRequest.Approved}}Approved{{else}}Pending{{end}})
            </div>
        </div>

        <div class="age-info">
            <strong>Age:</strong> {{.AgeText}}<br>
            <strong>Threshold:</strong> {{.ThresholdText}}
        </div>

        {{if .ActionRequired}}
        <div class="action-required">
            <h3>Action Required:</h3>
            <p>{{.ActionText}}</p>
        </div>
        {{end}}

        <div style="text-align: center;">
            <a href="{{.PullRequest.URL}}" class="button">View Pull Request</a>
        </div>

        <div class="footer">
            <p>This is an automated message from the PR Age Watcher.</p>
            <p>Generated at: {{.GeneratedAt}}</p>
        </div>
    </div>
</body>
</html>
`
		emailTemplate = template.Must(template.New("email").Parse(tmpl))
	})
	return emailTemplate
}

type EmailNotifier struct {
	config     config.EmailConfig
	skipEmails bool
	rateLimit  time.Duration
	mu         sync.Mutex
	closed     bool
	lastSent   time.Time
}

func NewEmailNotifier(cfg config.EmailConfig, skipEmails bool) (*EmailNotifier, error) {
	if !skipEmails && (cfg.SMTPHost == "" || cfg.SMTPPort == 0) {
		return nil, fmt.Errorf("SMTP configuration is required")
	}

	return &EmailNotifier{
		config:     cfg,
		skipEmails: skipEmails,
		rateLimit:  cfg.RateLimit,
		lastSent:   time.Now().Add(-cfg.RateLimit),
	}, nil
}

type NotificationType int

const (
	ApprovalReminder NotificationType = iota
	MergeReminder
	Escalation
	DraftOverdue
)

type NotificationData struct {
	Type        NotificationType
	PullRequest *github.PullRequest
	Age         time.Duration
	Threshold   time.Duration
	Recipients  []string
}

func (e *EmailNotifier) SendApprovalReminder(pr *github.PullRequest, age time.Duration, threshold time.Duration) error {
	data := &NotificationData{
		Type:        ApprovalReminder,
		PullRequest: pr,
		Age:         age,
		Threshold:   threshold,
		Recipients:  e.config.To,
	}

	return e.sendNotification(data)
}

func (e *EmailNotifier) SendMergeReminder(pr *github.PullRequest, age time.Duration, threshold time.Duration) error {
	data := &NotificationData{
		Type:        MergeReminder,
		PullRequest: pr,
		Age:         age,
		Threshold:   threshold,
		Recipients:  e.config.To,
	}

	return e.sendNotification(data)
}

func (e *EmailNotifier) SendEscalation(pr *github.PullRequest, age time.Duration, threshold time.Duration, escalationEmail string) error {
	recipients := e.config.To
	if escalationEmail != "" {
		recipients = append(recipients, escalationEmail)
	}

	data := &NotificationData{
		Type:        Escalation,
		PullRequest: pr,
		Age:         age,
		Threshold:   threshold,
		Recipients:  recipients,
	}

	return e.sendNotification(data)
}

func (e *EmailNotifier) SendDraftOverdue(pr *github.PullRequest, age time.Duration, threshold time.Duration) error {
	data := &NotificationData{
		Type:        DraftOverdue,
		PullRequest: pr,
		Age:         age,
		Threshold:   threshold,
		Recipients:  e.config.To,
	}

	return e.sendNotification(data)
}

func (e *EmailNotifier) sendNotification(data *NotificationData) error {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return fmt.Errorf("email notifier is closed")
	}
	e.mu.Unlock()

	if e.skipEmails {
		fmt.Printf("[SKIPPED] Would send %s email for PR #%d to %v\n",
			getNotificationTypeName(data.Type), data.PullRequest.Number, data.Recipients)
		return nil
	}

	// Sequential rate limiting - wait for proper interval
	e.mu.Lock()
	timeSinceLastSent := time.Since(e.lastSent)
	if timeSinceLastSent < e.rateLimit {
		waitTime := e.rateLimit - timeSinceLastSent
		fmt.Printf("[DEBUG] Rate limiting: waiting %v before sending next email\n", waitTime)
		time.Sleep(waitTime)
	}
	e.lastSent = time.Now()
	e.mu.Unlock()

	subject := e.getSubject(data)
	body, err := e.generateEmailBody(data)
	if err != nil {
		return fmt.Errorf("failed to generate email body: %w", err)
	}

	m := gomail.NewMessage()
	m.SetHeader("From", e.config.From)
	m.SetHeader("To", data.Recipients...)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	maxRetries := 3
	baseDelay := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		dialer := gomail.NewDialer(e.config.SMTPHost, e.config.SMTPPort, e.config.SMTPUsername, e.config.SMTPPassword)

		dialer.SSL = false
		if e.config.SMTPPort == 587 {
			// AWS SES requires STARTTLS on port 587
			dialer.TLSConfig = &tls.Config{
				ServerName: e.config.SMTPHost,
			}
		}

		fmt.Printf("[DEBUG] Attempting SMTP connection to %s:%d (attempt %d/%d)\n",
			e.config.SMTPHost, e.config.SMTPPort, i+1, maxRetries)

		if err := dialer.DialAndSend(m); err != nil {
			fmt.Printf("[DEBUG] SMTP connection failed: %v\n", err)
			if i == maxRetries-1 {
				return fmt.Errorf("failed to send email after %d retries: %w", maxRetries, err)
			}

			delay := baseDelay * time.Duration(1<<uint(i))
			fmt.Printf("Email send attempt %d failed (%v), retrying in %v...\n", i+1, err, delay)
			time.Sleep(delay)
			continue
		}
		fmt.Printf("[DEBUG] SMTP connection successful\n")
		break
	}

	return nil
}

func (e *EmailNotifier) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return
	}

	e.closed = true
}

func getNotificationTypeName(nt NotificationType) string {
	switch nt {
	case ApprovalReminder:
		return "approval reminder"
	case MergeReminder:
		return "merge reminder"
	case Escalation:
		return "escalation"
	case DraftOverdue:
		return "draft overdue"
	default:
		return "unknown"
	}
}

func (e *EmailNotifier) getSubject(data *NotificationData) string {
	baseSubject := e.config.Subject
	if baseSubject == "" {
		baseSubject = "PR Age Alert"
	}

	switch data.Type {
	case ApprovalReminder:
		return fmt.Sprintf("%s - PR #%d needs approval (%s)", baseSubject, data.PullRequest.Number, data.PullRequest.Repo)
	case MergeReminder:
		return fmt.Sprintf("MERGE REMINDER: %s - PR #%d ready to merge (%s)", baseSubject, data.PullRequest.Number, data.PullRequest.Repo)
	case Escalation:
		return fmt.Sprintf("ESCALATION: %s - PR #%d exceeds merge time (%s)", baseSubject, data.PullRequest.Number, data.PullRequest.Repo)
	case DraftOverdue:
		return fmt.Sprintf("DRAFT OVERDUE: %s - Draft PR #%d needs attention (%s)", baseSubject, data.PullRequest.Number, data.PullRequest.Repo)
	default:
		return baseSubject
	}
}

// generateEmailBody generates the HTML email body
func (e *EmailNotifier) generateEmailBody(data *NotificationData) (string, error) {
	type TemplateData struct {
		Title          string
		HeaderColor    string
		AgeColor       string
		PullRequest    *github.PullRequest
		AgeText        string
		ThresholdText  string
		ActionRequired bool
		ActionText     string
		GeneratedAt    string
	}

	templateData := TemplateData{
		PullRequest:   data.PullRequest,
		AgeText:       formatDuration(data.Age),
		ThresholdText: formatDuration(data.Threshold),
		GeneratedAt:   time.Now().Format("2006-01-02 15:04:05"),
	}

	switch data.Type {
	case ApprovalReminder:
		templateData.Title = "PR Needs Approval"
		templateData.HeaderColor = "#ffc107"
		templateData.AgeColor = "#fff3cd"
		templateData.ActionRequired = true
		templateData.ActionText = "This pull request has been open for " + formatDuration(data.Age) + " without approval. Please review and approve if ready."
	case MergeReminder:
		templateData.Title = "PR Ready to Merge"
		templateData.HeaderColor = "#28a745"
		templateData.AgeColor = "#d4edda"
		templateData.ActionRequired = true
		templateData.ActionText = "This pull request has been approved and ready for " + formatDuration(data.Age) + ". Please merge it to complete the review process."
	case Escalation:
		templateData.Title = "PR Escalation Required"
		templateData.HeaderColor = "#dc3545"
		templateData.AgeColor = "#f8d7da"
		templateData.ActionRequired = true
		templateData.ActionText = "This pull request has exceeded the merge time threshold of " + formatDuration(data.Threshold) + ". Immediate action is required to review and merge or close this PR."
	case DraftOverdue:
		templateData.Title = "Draft PR Overdue"
		templateData.HeaderColor = "#6c757d"
		templateData.AgeColor = "#e9ecef"
		templateData.ActionRequired = true
		templateData.ActionText = "This draft pull request has been open for " + formatDuration(data.Age) + " and exceeds the draft time threshold of " + formatDuration(data.Threshold) + ". Please either mark as ready for review or close if no longer needed."
	}

	// Use singleton template
	t := getEmailTemplate()
	var buf strings.Builder
	if err := t.Execute(&buf, templateData); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%.0f minutes", d.Minutes())
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.1f hours", d.Hours())
	} else {
		days := int(d.Hours() / 24)
		hours := int(d.Hours()) % 24
		if hours == 0 {
			return fmt.Sprintf("%d days", days)
		}
		return fmt.Sprintf("%d days, %d hours", days, hours)
	}
}
