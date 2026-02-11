# 🛻 Repomon

A fast tool to monitor multiple git repositories and report recent commits. Supports both local repositories and remote repositories via HTTPS or SSH, using shallow cloning and concurrency.


## Installation

```bash
# Build from source
git clone https://github.com/plars/repomon.git
cd repomon
go build -o repomon ./cmd/repomon

# Or install directly
go install github.com/plars/repomon/cmd/repomon@latest
```

## ⚡ Quick Start

1. **Create Configuration:**

```bash
mkdir -p ~/.config/repomon
cat > ~/.config/repomon/config.toml << 'EOF'
[defaults]
days = 7

[groups.default]
repos = [
    "/home/user/projects/my-project",
    "https://github.com/kubernetes/kubernetes",
]

[groups.work]
repos = [
    "https://github.com/go-git/go-git",
]

[groups.personal]
repos = [
    "~/projects/dotfiles",
]
EOF
```

2. **Run Report:**

```bash
# Using default group
repomon

# Specify a group
repomon -g work

# Custom config file
repomon -c /path/to/config.toml

# Last 3 days instead of default
repomon -d 3
```

## ⚙ Configuration

Configuration is done via TOML file. Use `~/.config/repomon/config.toml` or specify with `-c` flag.

Repository names are automatically extracted from paths/URLs. No manual naming required.

### Format

```toml
[defaults]
days = 7  # Number of days to look back

[groups.default]
repos = [
    "/home/user/projects/my-project",           # Local - auto-named "my-project"
    "https://github.com/go-git/go-git",        # Remote - auto-named "go-git"
    "git@github.com:plars/repomon.git",      # Remote SSH - auto-named "repomon"
    "~/projects/work-app",                     # Local with ~ - auto-named "work-app"
    "https://gitlab.com/company/project.git",   # Remote GitLab - auto-named "project"
]

[groups.work]
repos = [
    "https://github.com/company/backend",
    "https://github.com/company/frontend",
]

[groups.personal]
repos = [
    "~/projects/dotfiles",
    "https://github.com/user/hobby-project",
]
```

### Auto-Naming Rules

- **Local paths**: Uses the final directory name (e.g., `/home/user/projects/my-app` → "my-app")
- **HTTPS URLs**: Uses the repo name (e.g., `https://github.com/user/repo` → "repo")
- **SSH URLs**: Uses the repo name after colon (e.g., `git@github.com:user/repo` → "repo")
- **Trailing .git**: Automatically removed (e.g., `repo.git` → "repo")
- **Tilde expansion**: `~` expands to your home directory

## 📖 Usage

```bash
# Basic usage (uses 'default' group)
repomon

# Specify a group
repomon -g work
repomon --group personal

# Specify config file
repomon -c ~/.config/repomon/config.toml

# Custom time range
repomon -d 14

# Combine options
repomon -c /custom/config.toml -g work -d 30
```

### Adding Repositories

Add repositories to your configuration without editing the config file manually:

```bash
# Add to default group
repomon add https://github.com/kubernetes/kubernetes

# Add to a specific group
repomon add -g work https://github.com/go-git/go-git

# Add a local repository
repomon add ~/projects/my-project
```

The `add` command supports the same flags as other commands:
- `-c, --config`: Path to configuration file
- `-g, --group`: Repository group to add to (default: 'default')

### Listing Repositories

List configured repositories:

```bash
# List default group
repomon list

# List specific group
repomon list -g work

# List from custom config
repomon list -c /path/to/config.toml
```

### CLI Options

- `-c, --config`: Path to configuration file (default: `~/.config/repomon/config.toml`)
- `-d, --days`: Number of days to look back (default: 1)
- `-g, --group`: Repository group to use (default: 'default')
- `--debug`: Enable debug logging

## 🖵 Output Example

```
Repository Monitor Report
========================

📁 go-git
   Recent commits:
   • chore: update go.mod dependencies (2 hours ago)
   • feat: add support for partial clones (5 hours ago)
   • docs: update README with new features (1 day ago)

📁 kubernetes
   Recent commits:
   • fix: update deployment manifests (30 minutes ago)
   • feat: add new autoscaling features (3 hours ago)

📁 local-work
   ✅ No recent commits

📁 company-private
   ❌ Error: authentication required
```

## 🛠️ How It Works

### Local Repositories
- Directly opens existing git repository
- Reads commit history from local `.git` directory
- Very fast, no network access needed

### Remote Repositories
- Performs **shallow clone** with depth 1 (latest commit only)
- Uses **memory storage** to avoid disk writes
- Applies **date filtering** during iteration
- **No file checkout** - we only need commit metadata


## 📄 License

MIT License - see LICENSE file for details.
