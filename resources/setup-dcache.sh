#!/bin/bash

# DCache Project Setup Script
# This script automates the setup of the dcache project on macOS

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[✓]${NC} $1"
}

print_error() {
    echo -e "${RED}[✗]${NC} $1"
}

print_info() {
    echo -e "${YELLOW}[i]${NC} $1"
}

echo "======================================"
echo "DCache Project Setup Script"
echo "======================================"
echo ""

# Check if Homebrew is installed
print_info "Checking for Homebrew..."
if ! command -v brew &> /dev/null; then
    print_error "Homebrew not found. Installing..."
    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
    
    # Add Homebrew to PATH for Apple Silicon Macs
    if [[ $(uname -m) == "arm64" ]]; then
        echo 'eval "$(/opt/homebrew/bin/brew shellenv)"' >> ~/.zprofile
        eval "$(/opt/homebrew/bin/brew shellenv)"
    fi
else
    print_status "Homebrew is installed"
fi

# Install required tools
print_info "Installing required tools..."

# Install Git
if ! command -v git &> /dev/null; then
    brew install git
    print_status "Git installed"
else
    print_status "Git already installed"
fi

# Install VS Code
if ! command -v code &> /dev/null; then
    brew install --cask visual-studio-code
    print_status "VS Code installed"
else
    print_status "VS Code already installed"
fi

# Install Docker Desktop
if ! command -v docker &> /dev/null; then
    print_info "Installing Docker Desktop..."
    brew install --cask docker
    print_status "Docker Desktop installed"
    print_info "Please launch Docker Desktop from Applications and complete setup"
    print_info "Press Enter when Docker Desktop is running..."
    read
else
    print_status "Docker already installed"
fi

# Verify Docker is running
print_info "Checking Docker status..."
if docker info &> /dev/null; then
    print_status "Docker is running"
else
    print_error "Docker is not running. Please start Docker Desktop and re-run this script"
    exit 1
fi

# Install Go
if ! command -v go &> /dev/null; then
    brew install go
    print_status "Go installed"
    
    # Configure Go environment
    echo 'export GOPATH=$HOME/go' >> ~/.zshrc
    echo 'export PATH=$PATH:$GOPATH/bin' >> ~/.zshrc
    echo 'export GO111MODULE=on' >> ~/.zshrc
    source ~/.zshrc
    
    mkdir -p $HOME/go/{bin,src,pkg}
else
    print_status "Go already installed ($(go version))"
fi

# Install additional tools
print_info "Installing additional development tools..."
brew install wget curl jq make tree k6 wrk telnet netcat dive protobuf redis httpie grpcurl 2>/dev/null || true
print_status "Additional tools installed"

# Install VS Code extensions
print_info "Installing VS Code extensions..."
code --install-extension golang.go 2>/dev/null || true
code --install-extension ms-azuretools.vscode-docker 2>/dev/null || true
code --install-extension ms-vscode.makefile-tools 2>/dev/null || true
code --install-extension zxh404.vscode-proto3 2>/dev/null || true
code --install-extension 766b.go-outliner 2>/dev/null || true
code --install-extension donjayamanne.githistory 2>/dev/null || true
code --install-extension redhat.vscode-yaml 2>/dev/null || true
code --install-extension mikestead.dotenv 2>/dev/null || true
print_status "VS Code extensions installed"

# Create project directory
print_info "Setting up project directory..."
PROJECT_DIR="$HOME/projects/dcache"
mkdir -p "$PROJECT_DIR"
cd "$PROJECT_DIR"

# Initialize Git repository
if [ ! -d ".git" ]; then
    git init
    print_status "Git repository initialized"
else
    print_status "Git repository already exists"
fi

# Initialize Go module
if [ ! -f "go.mod" ]; then
    print_info "Enter your GitHub username (or press Enter for default):"
    read GITHUB_USER
    GITHUB_USER=${GITHUB_USER:-yourusername}
    go mod init github.com/$GITHUB_USER/dcache
    print_status "Go module initialized"
else
    print_status "Go module already exists"
fi

# Create directory structure
print_info "Creating project structure..."
mkdir -p cmd/{server,cli,benchmark}
mkdir -p internal/{cache,cluster,protocol,replication,storage}
mkdir -p pkg/client
mkdir -p configs
mkdir -p scripts
mkdir -p deployments/{docker,kubernetes}
mkdir -p test/{unit,integration,benchmark}
mkdir -p docs
mkdir -p examples
print_status "Project structure created"

# Copy configuration files from outputs (if they exist)
print_info "Setting up configuration files..."

# Create .gitignore
cat > .gitignore << 'EOF'
# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib
/bin/
/tmp/

# Test binary
*.test

# Output of go coverage tool
*.out
coverage.html

# Go workspace files
go.work

# Dependency directories
vendor/

# IDE directories
.idea/
.vscode/
*.swp
*.swo
*~

# OS files
.DS_Store
Thumbs.db

# Environment files
.env
.env.local

# Docker volumes
data/

# Log files
*.log
logs/
EOF
print_status ".gitignore created"

# Copy provided files if they're in the current directory
if [ -f "Makefile" ]; then
    cp Makefile "$PROJECT_DIR/"
    print_status "Makefile copied"
fi

if [ -f "docker-compose.yml" ]; then
    cp docker-compose.yml "$PROJECT_DIR/"
    print_status "docker-compose.yml copied"
fi

if [ -f "Dockerfile.dev" ]; then
    cp Dockerfile.dev "$PROJECT_DIR/"
    print_status "Dockerfile.dev copied"
fi

if [ -f ".air.toml" ]; then
    cp .air.toml "$PROJECT_DIR/"
    print_status ".air.toml copied"
fi

if [ -f "main.go" ]; then
    mkdir -p "$PROJECT_DIR/cmd/server"
    cp main.go "$PROJECT_DIR/cmd/server/"
    print_status "main.go copied to cmd/server/"
fi

# Build Docker images
print_info "Building Docker images..."
cd "$PROJECT_DIR"
docker compose build
print_status "Docker images built"

# Final message
echo ""
echo "======================================"
echo -e "${GREEN}Setup Complete!${NC}"
echo "======================================"
echo ""
echo "Project location: $PROJECT_DIR"
echo ""
echo "Next steps:"
echo "1. cd $PROJECT_DIR"
echo "2. make dev        # Start development server"
echo "3. make cluster    # Start 3-node cluster"
echo "4. make monitoring # Start monitoring stack"
echo ""
echo "Test the setup:"
echo "curl http://localhost:8080/health"
echo ""
echo "Happy coding! 🚀"
