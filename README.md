# kubectl-setimg

A kubectl plugin for updating container images in Kubernetes deployments with interactive selection and multi-registry support.

## Features

- **🧠 Interactive Selection**: Automatically provides interactive selection when arguments are omitted
- **🏷️ Automatic Tag Fetching**: Retrieves available tags from multiple container registries with timestamps
- **🔄 Multiple Operation Modes**: Interactive selection, direct command-line, and list modes
- **☁️ Multi-Registry Support**: AWS ECR, Google Cloud (GCR/Artifact Registry), and Docker Hub
- **📅 Smart Tag Sorting**: Tags sorted by creation date (newest first) with concurrent fetching
- **⏪ Automatic Rollback**: Watch deployment status and rollback on failure (optional)
- **🎨 Modern UI**: Rich terminal interface with filtering and keyboard navigation

## Installation

### Using Go Install (Recommended)
```bash
# Install directly from GitHub
go install github.com/tkuchiki/kubectl-setimg@latest

# Make sure $GOPATH/bin or $GOBIN is in your PATH
export PATH=$PATH:$(go env GOPATH)/bin
```

### Local Build
```bash
# Clone and build from source
git clone https://github.com/tkuchiki/kubectl-setimg.git
cd kubectl-setimg
go build -o kubectl-setimg .

# Install as kubectl plugin
sudo cp kubectl-setimg /usr/local/bin/kubectl-setimg
# or if using krew
cp kubectl-setimg ~/.krew/bin/kubectl-setimg
```

### Verify Installation
```bash
kubectl setimg --version
```

## Usage

### 🎯 Interactive Selection (Default)
When arguments are omitted, the plugin provides interactive selection:

```bash
# No arguments - interactive selection for deployment, container, and image
kubectl setimg

# Only deployment name - interactive selection for container and image
kubectl setimg my-app

# Deployment and container name - interactive selection for image
kubectl setimg my-app web
```

Selection flow: **Deployment** → **Container** → **Image Tag** with filtering and keyboard navigation.

### ⚡ Direct Mode
```bash
# Specify all arguments for direct execution
kubectl setimg my-app web=nginx:1.21.1

# With automatic rollback monitoring
kubectl setimg my-app web=nginx:1.21.1 --watch --timeout=5m
```

### 📋 List Mode
```bash
# Display all containers in a deployment
kubectl setimg my-app --list
kubectl setimg my-app -l
```

### ⏪ Watch Mode (Rollback on Failure)
```bash
# Monitor deployment and rollback if pods fail
kubectl setimg my-app web=nginx:1.21.1 --watch --timeout=10m
```

## Registry Support

### ✅ Fully Supported

#### AWS ECR (Amazon Elastic Container Registry)
- **Format**: `<account-id>.dkr.ecr.<region>.amazonaws.com/<repository>[:tag]`
- **Example**: `123456789012.dkr.ecr.us-west-2.amazonaws.com/my-app:v1.0.0`
- **Authentication**: AWS SDK v2 default credential chain
- **Setup**:
  ```bash
  # Using AWS CLI
  aws configure
  
  # Or environment variables
  export AWS_ACCESS_KEY_ID=your-access-key
  export AWS_SECRET_ACCESS_KEY=your-secret-key
  export AWS_DEFAULT_REGION=us-west-2
  
  # Required IAM permissions: ecr:DescribeImages, ecr:DescribeRepositories
  ```

#### Google Cloud (GCR/Artifact Registry)
- **GCR Format**: `gcr.io/<project>/<repository>[:tag]`
- **Artifact Registry**: `<region>-docker.pkg.dev/<project>/<repository>/<image>[:tag]`
- **Authentication**: Application Default Credentials (ADC)
- **Setup**:
  ```bash
  gcloud auth login
  # or
  gcloud auth application-default login
  ```

#### Docker Hub
- **Format**: `[docker.io/]<repository>[:tag]` or `<repository>[:tag]`
- **Example**: `nginx:latest`, `library/ubuntu:20.04`
- **Authentication**: Uses default Docker credentials (for private repos)

### 🚧 Extensible Architecture

The plugin uses a provider-based system for registry support:
```go
type Provider interface {
    ListTags(image string) ([]string, error)
    ListTagsWithInfo(image string) ([]TagInfo, error)
    SupportsImage(image string) bool
    Name() string
}
```

Additional registries (Azure ACR, Harbor, etc.) can be easily added through this interface.

## Requirements

- **Go**: 1.24.3+
- **Kubernetes**: Cluster access with valid kubeconfig
- **Registry Access**: Appropriate credentials for your container registries

## Configuration

### Kubernetes Context
```bash
# Use specific kubeconfig
kubectl setimg --kubeconfig=/path/to/config my-app

# Use specific context
kubectl setimg --context=production my-app

# Use specific namespace
kubectl setimg -n my-namespace my-app
```

### Registry Authentication

The plugin automatically detects and uses appropriate authentication methods:

- **AWS ECR**: AWS SDK credential chain (env vars, ~/.aws/credentials, IAM roles)
- **GCP**: Application Default Credentials
- **Docker Hub**: Default Docker authentication

## Examples

### Complete Workflow Examples

```bash
# 1. Fully interactive deployment update
kubectl setimg
# → Interactive selection of deployment
# → Interactive selection of container
# → Browse available tags with timestamps
# → Confirm update

# 2. Partial specification with interactive completion
kubectl setimg my-app
# → Use specified deployment
# → Interactive selection of container and image

# 3. Quick update with known values
kubectl setimg web-app frontend=nginx:1.21-alpine
# → Direct execution without interactive prompts

# 4. Safe update with automatic rollback
kubectl setimg web-app frontend=nginx:1.21-alpine --watch --timeout=5m
# → Updates image and monitors deployment
# → Automatically rolls back if deployment fails

# 5. ECR private repository
kubectl setimg api-service
# → Interactive container and image selection
# → Fetches tags from ECR with proper authentication
# → Shows creation timestamps for informed decisions
```

## Troubleshooting

### Common Issues

**Authentication Errors:**
```bash
# AWS ECR
aws sts get-caller-identity  # Verify AWS credentials
aws ecr describe-repositories --region us-west-2  # Test ECR access

# GCP
gcloud auth list  # Check active account
gcloud auth application-default print-access-token  # Test ADC

# Docker Hub
docker login  # For private repositories
```

**Network/Registry Issues:**
- Check firewall settings for registry endpoints
- Verify image names and registry URLs
- Use `--list` mode to debug deployment container names

**Kubernetes Access:**
```bash
kubectl get deployments  # Verify cluster access
kubectl get pods  # Check deployment status
```

## Dependencies

- **Kubernetes**: k8s.io/client-go, k8s.io/api, k8s.io/cli-runtime (v0.28.0)
- **CLI Framework**: github.com/spf13/cobra (v1.7.0)
- **TUI Framework**: github.com/charmbracelet/bubbletea + bubbles + lipgloss
- **Container Registries**:
  - google/go-containerregistry (GCP, Docker Hub)
  - github.com/aws/aws-sdk-go-v2 (AWS ECR)
- **Authentication**: golang.org/x/oauth2/google (GCP ADC)

## Contributing

This project follows standard Go practices and welcomes contributions:

1. **Package Structure**: Clean separation of concerns (cmd/, pkg/tui/, pkg/k8s/, pkg/registry/)
2. **Provider System**: Easy to add new registry providers
3. **Testing**: Comprehensive testing for all providers
4. **Documentation**: Detailed inline documentation

### Adding a New Registry Provider

1. Create a new file in `pkg/registry/`
2. Implement the `Provider` interface
3. Add to `NewClient()` in `pkg/registry/registry.go`
4. Add tests and documentation

Pull requests, issues, and feature requests are welcome!

## License

[Apache License 2.0](LICENSE)

## Roadmap

- [ ] Azure Container Registry support
- [ ] Harbor registry support
- [ ] Private registry with custom authentication
- [ ] Image vulnerability scanning integration
- [ ] Batch updates for multiple deployments
- [ ] Configuration file support
- [ ] Shell completion
- [ ] Homebrew distribution