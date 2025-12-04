# GitHub PR Age Watcher

Monitors GitHub pull requests and sends email notifications when PRs exceed specified time thresholds without approval or merge.

## Installation

1. **Clone the repository**:
   ```bash
   git clone https://github.com/jimohabdol/git-pr-watcher.git
   cd git-pr-watcher
   ```

2. **Install dependencies**:
   ```bash
   go mod tidy
   ```

3. **Build the application**:
   ```bash
   go build -o pr-watcher
   ```

## Configuration

### Using Configuration File

1. Copy the example configuration:
   ```bash
   cp config.yaml.example config.yaml
   ```

2. Edit `config.yaml` with your settings:
   ```yaml
   github:
     token: "ghp_your_github_token_here"
     owner: "your-org"
     repos:
       - "repo1"
       - "repo2"
   
   email:
     smtp_host: "smtp.gmail.com"
     smtp_port: 587
     smtp_username: "your-email@gmail.com"
     smtp_password: "your-app-password"
     to:
       - "team-lead@company.com"
     from: "pr-watcher@company.com"
   
   rules:
     approval_time: "24h"
     merge_time: "72h"
   ```

### Using Environment Variables

Alternatively, you can configure the application using environment variables:

```bash
export GITHUB_TOKEN="ghp_github_token_here"
export GITHUB_OWNER="org"
export GITHUB_REPOS="repo1,repo2"
export SMTP_HOST="smtp.gmail.com"
export SMTP_PORT="587"
export SMTP_USERNAME="email@gmail.com"
export SMTP_PASSWORD="password"
export EMAIL_TO=""
export EMAIL_FROM=""
export APPROVAL_TIME="2h"
export MERGE_TIME="4h"
```

## Usage

### One-time Check

Run a single check of all configured repositories:

```bash
./pr-watcher
```

### Continuous Monitoring

Run in watch mode for continuous monitoring:

```bash
./pr-watcher -watch -interval=1h
```

### Custom Configuration File

Specify a custom configuration file:

```bash
./pr-watcher -config=my-config.yaml
```

### Command Line Options

- `-config`: Path to configuration file (default: `config.yaml`)
- `-watch`: Run in watch mode (continuous monitoring)
- `-interval`: Check interval when in watch mode (default: `1h`)

## Run as a systemd service (Linux)

### Quick install

```bash
# Build locally
make build

# Install binary and service unit (requires sudo)
sudo -v && make install-systemd

# Start and check status
sudo systemctl start pr-watcher
sudo systemctl status pr-watcher --no-pager
```

This will:
- Install the binary to `/usr/local/bin/pr-watcher`
- Install the unit file to `/etc/systemd/system/pr-watcher.service`
- Create `/etc/pr-watcher/` and copy `config.yaml.example` to `/etc/pr-watcher/config.yaml` if missing
- Create a dedicated `pr-watcher` system user
- Enable the service to start on boot

### Customize configuration

Edit `/etc/pr-watcher/config.yaml` and restart:

```bash
sudoedit /etc/pr-watcher/config.yaml
sudo systemctl restart pr-watcher
```

### Logs

Logs go to journald:

```bash
journalctl -u pr-watcher -f
```

### Uninstall

```bash
sudo make uninstall-systemd
```

## GitHub Token Setup

1. Go to [GitHub Settings > Personal Access Tokens](https://github.com/settings/tokens)
2. Click "Generate new token (classic)"
3. Select the following scopes:
   - `repo` (for private repositories) or `public_repo` (for public repositories only)
   - `read:org` (if monitoring organization repositories)
4. Copy the generated token and use it in your configuration

## Email Setup

### Gmail Setup

1. Enable 2-factor authentication on your Gmail account
2. Generate an App Password:
   - Go to Google Account settings
   - Security > 2-Step Verification > App passwords
   - Generate a password for "Mail"
3. Use the app password (not your regular password) in the configuration

### Other SMTP Providers

The application supports any SMTP provider. Update the configuration with your provider's settings:

- **Outlook/Hotmail**: `smtp-mail.outlook.com:587`
- **Yahoo**: `smtp.mail.yahoo.com:587`
- **Custom SMTP**: Use your organization's SMTP server settings

## Monitoring Rules

The application monitors PRs based on two configurable time thresholds:

1. **Approval Time**: Time after which a PR needs approval
   - Default: 24 hours
   - Sends reminder email if PR is not approved

2. **Merge Time**: Time after which a PR should be merged
   - Default: 72 hours
   - Sends escalation email to additional recipients

## Email Notifications

### Approval Reminder
- Sent when a PR exceeds the approval time threshold
- Includes PR details, age, and review status
- Sent to configured email recipients

### Escalation
- Sent when a PR exceeds the merge time threshold
- Includes escalation email recipients

### Building

```bash
# Build for current platform
go build -o pr-watcher

# Build for Linux
GOOS=linux GOARCH=amd64 go build -o pr-watcher-linux

# Build for Windows
GOOS=windows GOARCH=amd64 go build -o pr-watcher.exe
```

