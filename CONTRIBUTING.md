# Contributing to SwitchBack

Thank you for your interest in contributing to **SwitchBack**!

## 🚀 Quick Start (Local Development)

### Prerequisites
* **Go 1.24+**: Download from [golang.org](https://golang.org)
* **Windows 10 / 11 (x64)**

### Building from Source

```powershell
# Clone repository
git clone https://github.com/JAIstudio-source/Switchback.git
cd Switchback

# Run tests
go test ./...

# Build the main CLI executable
go build -o switchback.exe ./cmd/switchback

# Build the Setup Installer
go build -o install-hooks.exe ./cmd/installer
```

## 🧪 Testing Guidelines

Before opening a pull request, ensure all tests pass:

```powershell
go test -v ./...
```

## 🤝 Code Style & Guidelines

* Keep window management code safe and non-blocking under `internal/win32`.
* Avoid adding external heavy CGO dependencies to maintain light executable sizes.
* Follow standard Go formatting (`go fmt ./...`).

## 🐛 Submitting Issues & Features

* For bug reports or feature requests, open an issue using GitHub Issues.
* For security concerns, please refer to [SECURITY.md](SECURITY.md).
