package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	GitHub GitHubConfig `yaml:"github"`
	Email  EmailConfig  `yaml:"email"`
	Rules  RulesConfig  `yaml:"rules"`
	Debug  DebugConfig  `yaml:"debug"`
}

type GitHubConfig struct {
	Token     string   `yaml:"token"`
	Owner     string   `yaml:"owner"`
	Repos     []string `yaml:"repos"`
	BaseURL   string   `yaml:"base_url,omitempty"`
	UploadURL string   `yaml:"upload_url,omitempty"`
}

type EmailConfig struct {
	SMTPHost     string        `yaml:"smtp_host"`
	SMTPPort     int           `yaml:"smtp_port"`
	SMTPUsername string        `yaml:"smtp_username"`
	SMTPPassword string        `yaml:"smtp_password"`
	From         string        `yaml:"from"`
	To           []string      `yaml:"to"`
	Subject      string        `yaml:"subject"`
	RateLimit    time.Duration `yaml:"rate_limit"`   // Rate limit between emails
	RateTimeout  time.Duration `yaml:"rate_timeout"` // Timeout for rate limiting
}

type RulesConfig struct {
	ApprovalTime      time.Duration `yaml:"approval_time"`
	MergeReminderTime time.Duration `yaml:"merge_reminder_time"`
	MergeTime         time.Duration `yaml:"merge_time"`
	DraftTime         time.Duration `yaml:"draft_time"`
	CheckInterval     time.Duration `yaml:"check_interval"`
	EscalationEmail   string        `yaml:"escalation_email"`
}

type DebugConfig struct {
	Enabled     bool `yaml:"enabled"`
	Verbose     bool `yaml:"verbose"`
	SkipEmails  bool `yaml:"skip_emails"`
	Concurrency int  `yaml:"concurrency"`
}

func Load(filename string) (*Config, error) {
	if _, err := os.Stat(filename); err == nil {
		data, err := os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		var config Config
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}

		setDefaults(&config)
		return &config, nil
	}

	config := &Config{
		GitHub: GitHubConfig{
			Token: os.Getenv("GITHUB_TOKEN"),
			Owner: os.Getenv("GITHUB_OWNER"),
		},
		Email: EmailConfig{
			SMTPHost:     os.Getenv("SMTP_HOST"),
			SMTPUsername: os.Getenv("SMTP_USERNAME"),
			SMTPPassword: os.Getenv("SMTP_PASSWORD"),
			From:         os.Getenv("EMAIL_FROM"),
		},
		Rules: RulesConfig{
			ApprovalTime:  parseDuration(os.Getenv("APPROVAL_TIME"), 2*time.Hour),
			MergeTime:     parseDuration(os.Getenv("MERGE_TIME"), 4*time.Hour),
			CheckInterval: parseDuration(os.Getenv("CHECK_INTERVAL"), 30*time.Minute),
		},
		Debug: DebugConfig{
			Enabled:     os.Getenv("DEBUG") == "true",
			Verbose:     os.Getenv("VERBOSE") == "true",
			SkipEmails:  os.Getenv("SKIP_EMAILS") == "true",
			Concurrency: 5,
		},
	}

	if repos := os.Getenv("GITHUB_REPOS"); repos != "" {
		config.GitHub.Repos = []string{repos}
	}

	if to := os.Getenv("EMAIL_TO"); to != "" {
		config.Email.To = []string{to}
	}

	setDefaults(config)
	return config, nil
}

func setDefaults(config *Config) {
	if config.Rules.ApprovalTime == 0 {
		config.Rules.ApprovalTime = 2 * time.Hour
	}
	if config.Rules.MergeReminderTime == 0 {
		config.Rules.MergeReminderTime = 4 * time.Hour
	}
	if config.Rules.MergeTime == 0 {
		config.Rules.MergeTime = 6 * time.Hour
	}
	if config.Rules.DraftTime == 0 {
		config.Rules.DraftTime = 96 * time.Hour
	}
	if config.Rules.CheckInterval == 0 {
		config.Rules.CheckInterval = 1 * time.Hour
	}
	if config.Email.Subject == "" {
		config.Email.Subject = "PR Age Alert"
	}
	if config.Email.RateLimit == 0 {
		config.Email.RateLimit = 500 * time.Millisecond
	}
	if config.Email.RateTimeout == 0 {
		config.Email.RateTimeout = 30 * time.Second
	}
	if config.Debug.Concurrency == 0 {
		config.Debug.Concurrency = 5
	}
}

// parseDuration parses duration from string with fallback
func parseDuration(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return fallback
	}
	return d
}
