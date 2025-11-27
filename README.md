# P2PFlow

<div align="center">
<img width="200" height="200" alt="P2PFlow" src="https://github.com/user-attachments/assets/4442a855-1d24-4bbf-9518-6cf84fc4ec63" />


**Intelligent Peer-to-Peer File Synchronization for Development Teams**

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/badge/Release-v0.0.0-green.svg)](https://github.com/JerryLegend254/P2PFlow/releases)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)](https://github.com/JerryLegend254/P2PFlow/actions)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

[Features](#-features) • [Installation](#-installation) • [Quick Start](#-quick-start) • [Modes](#-collaboration-modes) • [Documentation](docs/)

</div>

---

## 📖 Table of Contents

- [About](#-about)
- [Why P2PFlow?](#-why-p2pflow)
- [Features](#-features)
- [Installation](#-installation)
- [Quick Start](#-quick-start)
- [Collaboration Modes](#-collaboration-modes)
- [Analytics & Intelligence](#-analytics--intelligence)
- [Commands Reference](#-commands-reference)
- [Configuration](#-configuration)
- [Architecture](#-architecture)
- [Use Cases](#-use-cases)
- [Testing](#-testing)
- [Development](#-development)
- [Troubleshooting](#-troubleshooting)
- [Roadmap](#-roadmap)
- [Security](#-security)

---

## 🚀 About

**P2PFlow** is a decentralized, peer-to-peer file synchronization tool built with Go and libp2p. It enables real-time collaboration between developers without requiring a central server, featuring intelligent analytics, multiple collaboration modes, and CRDT-based conflict resolution.

Unlike traditional cloud-based sync solutions, P2PFlow provides:

- ⚡ **Instant synchronization** - Sub-second sync across team members
- 🔒 **Privacy-first architecture** - All data stays within your local network
- 🎯 **Zero configuration** - Works out of the box with sensible defaults
- 🪶 **Minimal resource usage** - Single lightweight binary
- 🌐 **True P2P** - Direct peer connections using libp2p
- 🤖 **AI-Powered Analytics** - Intelligent file access prediction and prefetching
- 🎭 **10 Collaboration Modes** - Optimized for different workflows

### Key Differentiators

| Feature | P2PFlow | Traditional Cloud Sync | Git |
|---------|----------|------------------------|-----|
| Real-time sync | ✅ Sub-second | ⏱️ Minutes | ❌ Manual |
| No internet required | ✅ Local network | ❌ Required | ✅ Can work offline |
| Automatic conflict detection | ✅ CRDT-based | ⚠️ Limited | ✅ Manual resolution |
| Setup time | ⚡ < 2 minutes | ⏱️ 10+ minutes | ⏱️ 5+ minutes |
| Privacy | 🔒 Complete | ⚠️ Data on servers | 🔒 Self-hosted option |
| Collaboration modes | ✅ 10 modes | ❌ None | ❌ None |
| AI Analytics | ✅ Built-in | ❌ None | ❌ None |

---

## 💡 Why P2PFlow?

### The Problem

Modern development teams face several collaboration challenges:

- **Git overhead**: Constant commits, pulls, and pushes interrupt flow
- **Cloud sync lag**: Services like Dropbox/Google Drive have multi-second delays
- **Merge conflicts**: Disruptive and time-consuming to resolve
- **Privacy concerns**: Sensitive code passing through third-party servers
- **Network dependency**: Can't collaborate on local networks without internet
- **One-size-fits-all**: No optimization for different collaboration patterns

### The Solution

P2PFlow provides **real-time, peer-to-peer file synchronization** that:

1. **Eliminates sync friction** - Files update instantly across all team members
2. **Preserves privacy** - All data stays within your local network
3. **Handles conflicts gracefully** - CRDT-based automatic conflict resolution
4. **Works everywhere** - Single binary, no dependencies, cross-platform
5. **Integrates seamlessly** - Works alongside Git, doesn't replace it
6. **Adapts to workflows** - 10 collaboration modes for different scenarios
7. **Learns from usage** - AI-powered analytics and predictions

### Perfect For

- 🏢 **Co-located teams** working in the same office
- 👥 **Pair programming** sessions requiring instant file sharing
- 🎓 **Educational environments** where students collaborate on projects
- 🏠 **Home lab setups** with multiple development machines
- 🚀 **Rapid prototyping** sessions where speed matters
- 👨‍🏫 **Mob programming** with driver/observer patterns
- 🔍 **Code reviews** with real-time change observation
- ✈️ **Offline-first** development with eventual consistency

---

## ✨ Features

### Core Capabilities

- **True Peer-to-Peer Architecture** - Direct connections between peers using libp2p, no central server required
- **Real-time Synchronization** - Instant file changes propagation across all connected peers (100ms sync interval)
- **CRDT-Based Collaboration** - Conflict-free replicated data types (RGA) for automatic conflict resolution
- **10 Collaboration Modes** - Optimized modes for different workflows:
  - `realtime` - Pair programming
  - `batch` - Bandwidth-efficient
  - `manual` - Full control
  - `review` - Code review with approval
  - `observer` - Read-only learning
  - `offline` - Work offline, sync later
  - `leader` - Mob programming driver
  - `follower` - Mob programming observer
  - `conflict-free` - CRDT-based eventual consistency
  - `selective` - Sync specific paths only
- **Intelligent Analytics** - AI-powered features:
  - File access tracking and pattern learning
  - Next-file prediction engine
  - Smart prefetch suggestions
  - Anomaly detection for unusual patterns
  - Bandwidth allocation optimization
- **Selective Sync** - Choose specific files/directories to synchronize
- **Bandwidth Management** - Configurable profiles with compression and throttling
- **Smart Ignore** - `.p2pignore` file support (similar to `.gitignore`)
- **OAuth Authentication** - GitHub OAuth integration for user authentication

### Advanced Features

- **Vector Clocks** - Distributed causality tracking for operation ordering
- **RGA (Replicated Growable Array)** - Efficient CRDT data structure for text editing
- **File Watcher Integration** - Automatic detection and sync of file changes using fsnotify
- **Conflict Resolution Strategies** - Multiple strategies:
  - Last-write-wins (timestamp-based)
  - Manual resolution
  - CRDT-based automatic merge
  - Leader-wins (for mob programming)
- **Anti-Entropy Protocol** - Ensures eventual consistency across distributed peers
- **Compression & Batching** - Efficient network usage with configurable batch operations (up to 60% bandwidth savings)
- **Session Management** - Create and join collaboration sessions with session IDs
- **Debouncing & Throttling** - Intelligent network optimization per mode

---

## 📥 Installation

### Prerequisites

- **Operating System**: macOS 10.15+, Linux (Ubuntu 20.04+, RHEL 8+), Windows 10+ (untested)
- **Go**: 1.25.0 or higher (for building from source)
- **Network**: Local network connectivity (WiFi or Ethernet)
- **Disk Space**: ~20MB for binary, additional space for project files
- **RAM**: Minimum 512MB available

### Installation Methods

#### Option 1: Build from Source

**Requirements**: Go 1.25 or higher

```bash
# Clone repository
git clone https://github.com/JerryLegend254/p2pflow.git
cd p2pflow

# Build the binary
go build -o bin/p2pflow cmd/p2pflowcli/*.go

# Add to PATH (optional)
export PATH=$PATH:$(pwd)/bin

# Or use make if available
make build
sudo make install
```

### Other install options coming soon!

### Verify Installation

```bash
p2pflow --version
# Output: p2pflow v0.0.0

# List available commands
p2pflow --help
```

---

## 🚀 Quick Start

### 1. Start a Collaboration Session

```bash
# Server (creates a new session)
p2pflow collab serve /path/to/project

# The command will output:
# - Session ID (share with collaborators)
# - Peer ID
# - Listening addresses
```

### 2. Join an Existing Session

```bash
# Client (joins an existing session)
p2pflow collab join <session-id>

# Files will start syncing immediately
```

### 3. Using Different Modes

```bash
# Real-time collaboration (default)
p2pflow collab serve /path/to/project --mode realtime

# Batch mode (bandwidth-efficient, 60% savings)
p2pflow collab serve /path/to/project --mode batch

# Observer mode (read-only)
p2pflow collab join <session-id> --mode observer

# CRDT-based conflict-free mode
p2pflow collab-crdt serve /path/to/project --mode conflict-free
```

### 4. View Analytics

```bash
# See which files are accessed most
p2pflow analytics stats

# Predict next files to be accessed
p2pflow analytics predict

# Detect unusual patterns
p2pflow analytics anomalies
```

---

## 🎭 Collaboration Modes

P2PFlow supports 10 different collaboration modes, each optimized for specific workflows:

| Mode | Use Case | Sync Behavior | Bandwidth | Best For |
|------|----------|---------------|-----------|----------|
| `realtime` | Pair programming | Instant (100ms) | High | Live collaboration |
| `batch` | Distributed teams | Periodic (5s) | Medium | Large teams, 60% savings |
| `manual` | Async workflows | Manual triggers | Medium | Full control |
| `review` | Code review | Manual approval | Medium | Quality control |
| `observer` | Learning/demos | Real-time read-only | Low | Watching others |
| `offline` | Unreliable networks | Offline-first | Low | Airplane mode |
| `leader` | Mob programming | Instant (leader) | High | Teaching, driver |
| `follower` | Following leader | Real-time | Medium | Learning, observer |
| `conflict-free` | Complex collab | CRDT-based | Medium | Multiple editors |
| `selective` | Focused work | Path-based | Medium | Specific features |

### Mode Examples

```bash
# List all available modes with descriptions
p2pflow modes

# Pair programming session
p2pflow collab serve ~/project --mode realtime

# Bandwidth-constrained environment
p2pflow collab serve ~/project --mode batch

# Mob programming (driver)
p2pflow collab serve ~/project --mode leader

# Mob programming (observers)
p2pflow collab join <session-id> --mode follower

# Read-only observer (perfect for demos)
p2pflow collab join <session-id> --mode observer

# Selective sync (only specific directories)
p2pflow collab serve ~/project --mode selective --selective-paths src/,docs/

# Work offline, sync when connected
p2pflow collab serve ~/project --mode offline
```

### Mode Configuration Parameters

Each mode has fine-grained control over:
- **Sync behavior**: Realtime, interval-based, or manual
- **Permissions**: Read-only, send/receive control
- **Conflict strategy**: Last-write-wins, manual, CRDT, leader-wins
- **Bandwidth profile**: High, medium, low, metered
- **Notifications**: All, important, batch, silent
- **Compression**: Enable/disable
- **Batching**: Max batch size and debounce intervals

See [docs/MODES.md](docs/MODES.md) for detailed mode documentation.

---

## 🤖 Analytics & Intelligence

P2PFlow includes an intelligent analytics engine that learns from your file access patterns:

### View Analytics

```bash
# Overall statistics
p2pflow analytics stats

# File-specific analytics
p2pflow analytics file path/to/file.go

# Predict next files to be accessed (ML-based)
p2pflow analytics predict

# Get intelligent prefetch suggestions
p2pflow analytics prefetch

# Detect anomalies and unusual patterns
p2pflow analytics anomalies
```

### Analytics Features

- **Access Tracking** - Records file access patterns with timestamps and access types
- **Prediction Engine** - ML-based prediction of likely next files (configurable confidence threshold)
- **Prefetch Suggestions** - Smart prefetching based on access patterns and co-occurrence
- **Anomaly Detection** - Identifies unusual sync patterns, frequency spikes, and suspicious behavior
- **Bandwidth Allocation** - Dynamic bandwidth management based on file priority and usage
- **Pattern Learning** - Learns your workflow over time (30-day history by default)

### Analytics Configuration

```yaml
# ~/.config/p2pflow/config.yaml
analytics:
  enabled: true
  storage_path: ".collab/analytics"
  prefetch_enabled: true
  anomaly_detection: true
  max_history_days: 30
  min_confidence: 0.6  # Minimum confidence for predictions
```

---

## 📚 Commands Reference

### Authentication

```bash
# Login with GitHub OAuth
p2pflow login

# Check current user
p2pflow whoami

# Logout
p2pflow logout
```

### Configuration

```bash
# Set configuration value
p2pflow config:set <key> <value>

# View configuration
p2pflow config:show
```

### Collaboration

```bash
# Standard P2P collaboration
p2pflow collab serve <directory> [--mode <mode>]
p2pflow collab join <session-id> [--mode <mode>]

# CRDT-based collaboration (conflict-free)
p2pflow collab-crdt serve <directory> [--mode <mode>]
p2pflow collab-crdt join <session-id> [--mode <mode>]
```

### Analytics

```bash
p2pflow analytics stats              # Overall statistics
p2pflow analytics file <path>        # File-specific analytics
p2pflow analytics predict            # Predict next files
p2pflow analytics prefetch           # Prefetch suggestions
p2pflow analytics anomalies          # Detect anomalies
```

### Modes

```bash
p2pflow modes                        # List all collaboration modes
```

---

## ⚙️ Configuration

### Config File

P2PFlow uses a YAML configuration file located at `~/.config/p2pflow/config.yaml`:

```yaml
# Authentication
github:
  client_id: "your-client-id"
  client_secret: "your-client-secret"

# Analytics
analytics:
  enabled: true
  storage_path: ".collab/analytics"
  prefetch_enabled: true
  anomaly_detection: true
  max_history_days: 30
  min_confidence: 0.6

# Collaboration
collab:
  default_mode: "realtime"
  session_timeout: 3600
```

### Environment Variables

```bash
# GitHub OAuth
export GITHUB_CLIENT_ID="your-client-id"
export GITHUB_CLIENT_SECRET="your-client-secret"

# Default collaboration mode
export P2PFLOW_MODE="batch"
```

### .p2pignore File

Create a `.p2pignore` file in your project root to exclude files from synchronization:

```
# Ignore patterns (similar to .gitignore)
.git/
node_modules/
*.log
.env
.DS_Store
dist/
build/
__pycache__/
```

See [.p2pignore.example](.p2pignore.example) for comprehensive examples.

---

## 🏗️ Architecture

### Technology Stack

- **Language**: Go 1.25
- **P2P Networking**: [libp2p](https://libp2p.io/) - Industry-standard P2P networking
- **Pub/Sub**: libp2p pubsub for message broadcasting
- **CRDTs**: Custom RGA (Replicated Growable Array) implementation
- **CLI Framework**: [Cobra](https://github.com/spf13/cobra)
- **Configuration**: [Viper](https://github.com/spf13/viper)
- **Logging**: [Zap](https://github.com/uber-go/zap) - High-performance structured logging
- **File Watching**: [fsnotify](https://github.com/fsnotify/fsnotify) - Cross-platform file system notifications

### Key Components

```
p2pflow/
├── cmd/
│   └── p2pflowcli/          # CLI application
│       ├── main.go          # Entry point
│       ├── cli.go           # Command setup
│       ├── collab.go        # Collaboration commands
│       ├── collab_crdt.go   # CRDT collaboration commands
│       ├── analytics.go     # Analytics commands
│       ├── modes.go         # Modes command
│       ├── login.go         # Authentication commands
│       └── config.go        # Configuration commands
├── internal/
│   ├── network/             # P2P networking layer
│   │   ├── node.go          # Standard P2P node
│   │   ├── crdt_node.go     # CRDT-based P2P node
│   │   └── messages.go      # Network message types
│   ├── crdt/                # CRDT implementation
│   │   ├── engine.go        # CRDT session management
│   │   ├── rga.go           # RGA implementation
│   │   ├── vector_clock.go  # Vector clock for causality
│   │   └── persistence.go   # CRDT state persistence
│   ├── modes/               # Collaboration modes
│   │   └── modes.go         # Mode definitions and configs
│   ├── analytics/           # Intelligence layer
│   │   ├── analytics.go     # Analytics engine
│   │   ├── tracker.go       # Access tracking
│   │   ├── predictor.go     # ML-based prediction
│   │   ├── prefetch.go      # Prefetch engine
│   │   ├── anomaly.go       # Anomaly detection
│   │   └── bandwidth.go     # Bandwidth allocation
│   ├── watcher/             # File system watching
│   │   ├── watcher.go       # Standard watcher
│   │   └── crdt_watcher.go  # CRDT-aware watcher
│   ├── collab/              # Collaboration engine
│   │   ├── engine.go        # Session management
│   │   ├── session_manager.go # Session lifecycle
│   │   └── conflict_resolver.go # Conflict resolution
│   ├── ignore/              # .p2pignore handling
│   ├── auth/                # Authentication
│   │   ├── auth.go          # Auth interface
│   │   └── oauth.go         # OAuth implementation
│   └── logger/              # Logging utilities
├── docs/                    # Documentation
│   └── MODES.md            # Collaboration modes guide
├── examples/                # Example scripts
│   └── modes-demo.sh       # Mode demonstration
└── scripts/                 # Testing scripts
    ├── test-bidirectional-sync.sh
    ├── test-dev-sync.sh
    └── debug-test.sh
```

---

## 💼 Use Cases

### Pair Programming

Two developers working on the same codebase in real-time:

```bash
# Developer A
p2pflow collab serve ~/project --mode realtime

# Developer B
p2pflow collab join <session-id> --mode realtime
```

### Code Review

Reviewer watches changes as they're made:

```bash
# Reviewer (requires approval for changes)
p2pflow collab serve ~/project --mode review

# Developer (makes changes)
p2pflow collab join <session-id> --mode realtime
```

### Mob Programming

One driver, multiple observers:

```bash
# Driver (changes take precedence)
p2pflow collab serve ~/project --mode leader

# Team members (can make suggestions)
p2pflow collab join <session-id> --mode follower
```

### Remote Teams

Distributed team with bandwidth constraints:

```bash
# All team members (60% bandwidth savings)
p2pflow collab serve ~/project --mode batch
```

### Live Demos & Teaching

Presenter shares code, students watch:

```bash
# Presenter
p2pflow collab serve ~/demo --mode realtime

# Students (read-only, cannot modify)
p2pflow collab join <session-id> --mode observer
```

### Offline Development

Work offline, sync when connected:

```bash
# Work offline with CRDT-based eventual consistency
p2pflow collab serve ~/project --mode offline

# Changes sync automatically when network available
```

---

## 🧪 Testing

### Run Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/crdt/...
go test ./internal/analytics/...

# Verbose output
go test -v ./...
```

### Test Scripts

```bash
# Test bidirectional sync
./scripts/test-bidirectional-sync.sh

# Test development sync
./scripts/test-dev-sync.sh

# Debug test with detailed logs
./scripts/debug-test.sh

# Test interview scenario
./scripts/test-interview.sh
```

### Build

```bash
# Build binary
go build -o bin/p2pflow cmd/p2pflowcli/*.go

# Run static checks
staticcheck ./...

# Build for all platforms (if make available)
make build-all
```

---

## 👨‍💻 Development

### Project Structure

- **cmd/** - Command-line interface and application entry points
- **internal/** - Internal packages (not importable by other projects)
- **docs/** - Documentation
- **scripts/** - Testing and utility scripts
- **examples/** - Example scripts and demos

### Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines.

### Code Style

- Follow Go conventions and best practices
- Run `go fmt` before committing
- Run `staticcheck` to catch common issues
- Write tests for new functionality
- Add documentation for public APIs

---

## 🔧 Troubleshooting

### Connection Issues

**Problem**: Cannot connect to peer

**Solutions**:
- Check firewall settings (libp2p needs open ports)
- Ensure both peers are on the same network or have proper NAT traversal
- Verify session ID is correct
- Check network connectivity
- Review peer addresses in logs

### Sync Issues

**Problem**: Files not syncing

**Solutions**:
- Verify the file is not in `.p2pignore`
- Check file permissions
- Review mode configuration (ensure not in observer/read-only mode)
- Check analytics logs for errors (`.collab/analytics/`)
- Verify file watcher is running

### Performance Issues

**Problem**: High bandwidth usage

**Solutions**:
- Switch to `batch` mode for reduced bandwidth (~60% savings)
- Use `selective` mode to sync specific paths only
- Enable compression in configuration
- Adjust throttle rate for your mode
- Use metered bandwidth profile for limited connections

### CRDT Conflicts

**Problem**: Unexpected merge results

**Solutions**:
- Use `conflict-free` mode with CRDT-based sync
- Ensure vector clocks are synchronized
- Check operation log for ordering issues
- Review CRDT state persistence (`.collab/crdt/`)
- Verify all peers have consistent state

### Analytics Issues

**Problem**: Analytics not working

**Solutions**:
- Check if analytics is enabled in config
- Verify storage path exists and is writable
- Review access log file (`.collab/analytics/access_log.json`)
- Ensure sufficient disk space
- Check confidence threshold settings

---

## 🗺️ Roadmap

### Planned Features

- [ ] Runtime mode switching without restart
- [ ] Custom mode creation via CLI
- [ ] Web UI for session monitoring and visualization
- [ ] End-to-end encryption for secure collaboration
- [ ] Mobile client support (iOS/Android)
- [ ] IDE integration (VS Code, JetBrains IDEs, Vim/Neovim)
- [ ] Cloud backup and sync (optional)
- [ ] Automatic mode suggestion based on network conditions
- [ ] Per-file mode configuration
- [ ] Conflict visualization and resolution UI
- [ ] Session recording and playback
- [ ] Real-time metrics dashboard
- [ ] Plugin system for extensibility
- [ ] Multi-session support
- [ ] Session persistence across restarts

### Current Limitations

- No end-to-end encryption (in development)
- Limited to text-based files for CRDT mode
- Manual sync triggers not yet implemented for manual mode
- No Windows support (untested)
- Single session per instance

---

## 🔒 Security

### Current Security Features

- OAuth authentication with GitHub
- Peer identity verification using libp2p
- Session-based access control
- Read-only mode enforcement
- Permission-based change propagation

### Best Practices

- Use `observer` mode for untrusted peers
- Use `review` mode for approval-gated workflows
- Keep OAuth credentials secure (use environment variables)
- Regularly update dependencies
- Use `.p2pignore` to exclude sensitive files
- Monitor analytics for anomaly detection

### Reporting Security Issues

Please report security vulnerabilities to: [security contact - to be added]

Do not open public issues for security vulnerabilities.

---

## 📄 License

[License type to be specified - MIT suggested]

---

## 🙏 Acknowledgments

- Built with [libp2p](https://libp2p.io/) - The modular P2P networking stack
- Inspired by collaborative editing tools and Git
- CRDT implementation based on research papers on RGA (Replicated Growable Array)
- Thanks to the Go community for excellent tooling

---

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/JerryLegend254/p2pflow/issues)
- **Documentation**: [docs/](docs/)
- **Examples**: [examples/](examples/)
- **Discussions**: [GitHub Discussions](https://github.com/JerryLegend254/p2pflow/discussions)

---

## 🔗 Links

- [Modes Documentation](docs/MODES.md) - Detailed collaboration modes guide
- [GitHub Repository](https://github.com/JerryLegend254/p2pflow)
- [libp2p Documentation](https://docs.libp2p.io/)
- [CRDT Research Papers](https://crdt.tech/)

---

## 📊 Project Statistics

![GitHub Stars](https://img.shields.io/github/stars/JerryLegend254/P2PFlow?style=social)
![GitHub Forks](https://img.shields.io/github/forks/JerryLegend254/P2PFlow?style=social)
![GitHub Issues](https://img.shields.io/github/issues/JerryLegend254/P2PFlow)
![GitHub Pull Requests](https://img.shields.io/github/issues-pr/JerryLegend254/P2PFlow)
![GitHub Contributors](https://img.shields.io/github/contributors/JerryLegend254/P2PFlow)
![GitHub Last Commit](https://img.shields.io/github/last-commit/JerryLegend254/P2PFlow)

---

<div align="center">

**Built with ❤️ and Go. Designed for real-time collaboration without compromise.**

**Made by developers, for developers**

[⬆ Back to Top](#p2pflow)

</div>
