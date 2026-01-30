# Repomon

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

## Quick Start

1. **Create Configuration:**

```bash
mkdir -p ~/.config/repomon
cat > ~/.config/repomon/config.toml << 'EOF'
[defaults]
days = 7

[[repos]]
name = "my-project"
path = "/home/user/projects/my-project"

[[repos]]
name = "kubernetes"
url = "https://github.com/kubernetes/kubernetes"
EOF
```

2. **Run Report:**

```bash
# Using default config
repomon

# Custom config file
repomon -c /path/to/config.toml

# Last 3 days instead of default
repomon -d 3
```

## Configuration

Configuration is done via TOML file. Use `~/.config/repomon/config.toml` or specify with `-c` flag.

### Repository Types

**Local Repositories:**
```toml
[[repos]]
name = "local-project"
path = "/home/user/projects/local-project"
```

**Remote Repositories (HTTPS):**
```toml
[[repos]]
name = "public-repo"
url = "https://github.com/user/repo"
```

**Remote Repositories (SSH):**
```toml
[[repos]]
name = "private-repo"
url = "git@github.com:user/private-repo.git"
```

### Full Example

```toml
[defaults]
days = 7  # Number of days to look back

[[repos]]
name = "local-work"
path = "/home/user/work/projects/webapp"

[[repos]]
name = "go-git"
url = "https://github.com/go-git/go-git"

[[repos]]
name = "company-private"
url = "git@github.com:company/internal-repo.git"

[[repos]]
name = "gitlab-project"
url = "https://gitlab.com/company/project.git"
```

## Usage

```bash
# Basic usage
repomon

# Specify config file
repomon -c ~/.config/repomon/config.toml

# Custom time range
repomon -d 14

# Combine options
repomon -c /custom/config.toml -d 30
```

### CLI Options

- `-c, --config`: Path to configuration file (default: `~/.config/repomon/config.toml`)
- `-d, --days`: Number of days to look back (default: 1)
- `--debug`: Enable debug logging

## Output Example

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

## How It Works

### Local Repositories
- Directly opens existing git repository
- Reads commit history from local `.git` directory
- Very fast, no network access needed

### Remote Repositories
- Performs **shallow clone** with depth 1 (latest commit only)
- Uses **memory storage** to avoid disk writes
- Applies **date filtering** during iteration
- **No file checkout** - we only need commit metadata


## License

MIT License - see LICENSE file for details.
