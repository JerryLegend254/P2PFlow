# P2PFlow - Build and Release Guide

Complete guide for building and releasing P2PFlow.

## Quick Links

- **Quick Start**: [QUICK_RELEASE.md](QUICK_RELEASE.md)
- **Full Guide**: [RELEASE.md](RELEASE.md)
- **Changelog**: [CHANGELOG.md](CHANGELOG.md)

---

## 📦 What We've Set Up

### 1. Makefile (`Makefile`)

A comprehensive build system with 40+ commands organized into categories:

#### **Development Commands**
```bash
make build          # Build for current platform
make build-dev      # Build with debug symbols
make install        # Install to $GOPATH/bin
make run            # Build and run
```

#### **Testing Commands**
```bash
make test              # Run all tests
make test-coverage     # Generate coverage report
make test-integration  # Run integration tests
make bench             # Run benchmarks
make check             # Run all checks (fmt, vet, lint, test)
```

#### **Code Quality**
```bash
make fmt       # Format code
make vet       # Run go vet
make lint      # Run staticcheck
```

#### **Cross-Platform Builds**
```bash
make build-all      # Build for all platforms
make build-linux    # Linux only
make build-darwin   # macOS only
make build-windows  # Windows only
```

#### **Release Commands**
```bash
make release-prep       # Full release preparation
make release-archives   # Create tar.gz/zip archives
make release-checksums  # Generate SHA256 checksums
make tag VERSION=v1.0.0 # Create and push git tag
```

#### **Maintenance**
```bash
make clean           # Remove build artifacts
make clean-all       # Full cleanup including deps
make deps-tidy       # Tidy dependencies
make info            # Show build information
```

### 2. GitHub Actions Workflows

#### **Release Workflow** (`.github/workflows/release.yml`)

Automatically triggers on tag push (e.g., `v1.0.0`):
- ✅ Runs tests
- ✅ Builds for all platforms
- ✅ Creates archives (.tar.gz, .zip)
- ✅ Generates checksums
- ✅ Creates GitHub release
- ✅ Uploads all binaries
- ✅ Generates changelog

**To use:**
```bash
git tag v1.0.0
git push origin v1.0.0
# GitHub Actions does the rest!
```

#### **Test Workflow** (`.github/workflows/test.yml`)

Runs on every push and pull request:
- ✅ Tests on Linux and macOS
- ✅ Runs linting (golangci-lint, staticcheck)
- ✅ Generates coverage reports
- ✅ Verifies builds

### 3. Documentation

#### **RELEASE.md** - Complete Release Guide
Comprehensive documentation covering:
- Prerequisites
- Step-by-step release process
- Release notes template
- Troubleshooting
- Best practices

#### **QUICK_RELEASE.md** - Quick Reference
Fast reference guide:
- 5-minute release process
- Command cheat sheet
- Common issues
- Quick fixes

#### **CHANGELOG.md** - Version History
Standard changelog format:
- Semantic versioning
- Keep a Changelog format
- Template for new releases
- Version history

#### **BUILD_AND_RELEASE.md** (this file)
Overview of the entire build and release system.

---

## 🚀 Release Methods

### Method 1: Automated Release (Recommended)

**Uses GitHub Actions for fully automated releases.**

```bash
# 1. Ensure everything is committed and pushed
git add .
git commit -m "chore: prepare for release v1.0.0"
git push origin main

# 2. Create and push tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# 3. GitHub Actions automatically:
#    - Runs tests
#    - Builds for all platforms
#    - Creates release
#    - Uploads binaries
```

**That's it!** GitHub Actions handles everything else.

### Method 2: Manual Release with Make

**Uses Makefile for manual control.**

```bash
# 1. Prepare (runs checks + builds all platforms)
make release-prep

# 2. Create archives
make release-archives

# 3. Tag version
make tag VERSION=v1.0.0

# 4. Create GitHub release manually or with gh CLI
gh release create v1.0.0 \
  --title "P2PFlow v1.0.0" \
  --notes "Release notes" \
  dist/archives/*
```

### Method 3: Quick Development Build

**For local development and testing.**

```bash
# Quick build
make build

# Or with make install
make install

# Run tests
make test

# Full check
make check
```

---

## 📋 Release Checklist

Use this checklist before every release:

### Pre-Release
- [ ] All tests pass: `make test`
- [ ] Code formatted: `make fmt`
- [ ] Linting passes: `make lint`
- [ ] Integration tests pass: `make test-integration`
- [ ] Version updated in `cmd/p2pflowcli/main.go`
- [ ] CHANGELOG.md updated with new version
- [ ] README.md updated if needed
- [ ] Documentation updated

### Release Process
- [ ] Clean build: `make clean && make build-all`
- [ ] All platforms build successfully
- [ ] Archives created: `make release-archives`
- [ ] Checksums generated
- [ ] Tag created: `make tag VERSION=vX.Y.Z`
- [ ] GitHub release published
- [ ] Release notes complete
- [ ] Binaries uploaded

### Post-Release
- [ ] Release verified on GitHub
- [ ] Binaries tested on target platforms
- [ ] Documentation updated
- [ ] Announcements made (if applicable)
- [ ] Issues closed/milestones updated

---

## 🏗️ Build System Architecture

### Build Outputs

```
p2pflow/
├── bin/                          # Local builds
│   └── p2pflow                   # Current platform binary
├── dist/                         # Cross-platform builds
│   ├── p2pflow-darwin-amd64      # macOS Intel
│   ├── p2pflow-darwin-arm64      # macOS Apple Silicon
│   ├── p2pflow-linux-amd64       # Linux x86_64
│   ├── p2pflow-linux-arm64       # Linux ARM64
│   ├── p2pflow-windows-amd64.exe # Windows 64-bit
│   └── archives/                 # Release archives
│       ├── p2pflow-darwin-amd64.tar.gz
│       ├── p2pflow-darwin-arm64.tar.gz
│       ├── p2pflow-linux-amd64.tar.gz
│       ├── p2pflow-linux-arm64.tar.gz
│       ├── p2pflow-windows-amd64.zip
│       └── checksums.txt
└── coverage.txt                  # Test coverage
```

### Version Information

Version info is embedded in the binary at build time:

```go
// In cmd/p2pflowcli/main.go
var version = "0.0.0"

// Build flags inject additional info:
// -X 'main.version=$(VERSION)'
// -X 'main.commit=$(COMMIT)'
// -X 'main.buildDate=$(BUILD_DATE)'
```

Run `make info` to see current build information.

### Supported Platforms

| OS | Architecture | Binary Name |
|----|--------------|-------------|
| macOS | amd64 (Intel) | `p2pflow-darwin-amd64` |
| macOS | arm64 (M1/M2) | `p2pflow-darwin-arm64` |
| Linux | amd64 (x86_64) | `p2pflow-linux-amd64` |
| Linux | arm64 (ARM) | `p2pflow-linux-arm64` |
| Windows | amd64 (64-bit) | `p2pflow-windows-amd64.exe` |

---

## 🔧 Configuration

### Build Flags

The Makefile uses optimized build flags:

```makefile
LDFLAGS := -s -w \                          # Strip debug info
  -X 'main.version=$(VERSION)' \            # Embed version
  -X 'main.commit=$(COMMIT)' \              # Embed commit
  -X 'main.buildDate=$(BUILD_DATE)'         # Embed build date
```

### Environment Variables

Override defaults with environment variables:

```bash
# Custom version
VERSION=v2.0.0 make build

# Custom output directory
BUILD_DIR=output make build

# Verbose output
V=1 make build
```

---

## 📊 Versioning Strategy

### Semantic Versioning

P2PFlow follows [SemVer](https://semver.org/):

**Format**: `vMAJOR.MINOR.PATCH`

- **MAJOR** (`v2.0.0`): Breaking changes
- **MINOR** (`v1.1.0`): New features (backwards compatible)
- **PATCH** (`v1.0.1`): Bug fixes (backwards compatible)

### Pre-Release Versions

- **Alpha**: `v1.0.0-alpha.1` - Early testing
- **Beta**: `v1.0.0-beta.1` - Feature complete, testing
- **RC**: `v1.0.0-rc.1` - Release candidate

### Version Timeline

```
v0.1.0  → Initial features
v0.2.0  → CRDT support added
v0.3.0  → Analytics added
v0.9.0  → Release candidate
v1.0.0  → First stable release
v1.1.0  → New features
v1.1.1  → Bug fixes
v2.0.0  → Breaking changes
```

---

## 🤖 GitHub Actions Details

### Workflow Triggers

**Release Workflow** (`.github/workflows/release.yml`):
```yaml
on:
  push:
    tags:
      - 'v*'  # Triggers on v1.0.0, v2.1.3, etc.
```

**Test Workflow** (`.github/workflows/test.yml`):
```yaml
on:
  push:
    branches: [ main, develop, feat/* ]
  pull_request:
    branches: [ main, develop ]
```

### Workflow Jobs

**Release Workflow**:
1. **Test** - Runs tests and staticcheck
2. **Build** - Builds for all platforms (matrix build)
3. **Release** - Creates GitHub release with all binaries

**Test Workflow**:
1. **Test** - Runs tests on Linux and macOS
2. **Lint** - Runs golangci-lint and staticcheck
3. **Build** - Verifies build succeeds

### Artifacts

All workflows upload artifacts that persist for 90 days:
- Test coverage reports
- Build binaries
- Release archives

---

## 📚 Common Workflows

### Daily Development

```bash
# Start working
git checkout -b feature/new-feature

# Code, test, iterate
make test
make build
./bin/p2pflow --help

# Before commit
make check

# Commit and push
git commit -m "feat: add new feature"
git push
```

### Before Pull Request

```bash
# Run all checks
make check

# Test on all platforms (optional)
make build-all

# Clean up
make clean
```

### Release Day

```bash
# Ensure clean state
git checkout main
git pull

# Final checks
make release-prep

# Tag and let GitHub Actions handle it
git tag v1.0.0
git push origin v1.0.0

# Or manual release
make release-archives
gh release create v1.0.0 --notes "..." dist/archives/*
```

---

## 🐛 Troubleshooting

### Build Issues

**Problem**: Build fails with missing dependencies
```bash
make clean
make deps-tidy
make build
```

**Problem**: Wrong Go version
```bash
go version  # Check version
# Update to Go 1.25+
make build
```

**Problem**: Binary is too large
```bash
# Already optimized with -s -w flags
# Check size
ls -lh bin/p2pflow

# Strip further if needed
strip bin/p2pflow
```

### Release Issues

**Problem**: Tag already exists
```bash
git tag -d v1.0.0
git push --delete origin v1.0.0
make tag VERSION=v1.0.0
```

**Problem**: GitHub Actions fails
- Check workflow logs in GitHub
- Verify secrets are set (if needed)
- Test build locally: `make build-all`

**Problem**: Checksums don't match
```bash
cd dist/archives
rm checksums.txt
shasum -a 256 * > checksums.txt
```

---

## 🔗 Resources

### Documentation
- [RELEASE.md](RELEASE.md) - Complete release guide
- [QUICK_RELEASE.md](QUICK_RELEASE.md) - Quick reference
- [CHANGELOG.md](CHANGELOG.md) - Version history
- [README.md](README.md) - Project overview

### External Links
- [Go Documentation](https://go.dev/doc/)
- [GitHub Actions Docs](https://docs.github.com/en/actions)
- [Semantic Versioning](https://semver.org/)
- [Keep a Changelog](https://keepachangelog.com/)

### Tools
- [Make Documentation](https://www.gnu.org/software/make/manual/)
- [GitHub CLI](https://cli.github.com/)
- [GoReleaser](https://goreleaser.com/) (alternative)

---

## 💡 Tips and Best Practices

### 1. Always Test Before Release
```bash
make check  # Runs fmt, vet, lint, test
```

### 2. Use Semantic Versioning
- Breaking changes = major version bump
- New features = minor version bump
- Bug fixes = patch version bump

### 3. Keep CHANGELOG Updated
Update CHANGELOG.md with every significant change.

### 4. Tag Early, Tag Often
Create tags for releases, even pre-releases:
```bash
git tag v1.0.0-beta.1
```

### 5. Test Binaries on Target Platforms
Before releasing, test on actual platforms when possible.

### 6. Automate What You Can
GitHub Actions handles releases automatically - use it!

### 7. Document Everything
Keep release notes detailed and user-friendly.

---

## 🎯 Next Steps

1. **First Release**
   ```bash
   make release-prep
   make tag VERSION=v0.1.0
   ```

2. **Set Up CI/CD**
   - Ensure GitHub Actions workflows are enabled
   - Test automated builds

3. **Document Changes**
   - Keep CHANGELOG.md up to date
   - Write clear release notes

4. **Iterate**
   - Gather feedback
   - Fix bugs
   - Add features
   - Release new versions

---

**Happy Building! 🚀**

For questions or issues, see:
- [GitHub Issues](https://github.com/JerryLegend254/p2pflow/issues)
- [Discussions](https://github.com/JerryLegend254/p2pflow/discussions)
