# k8s-pods-debug

A powerful Kubernetes debugging tool that creates secure debug pods with non-root privileges and advanced management capabilities.

## ✨ Features

- **🔒 Secure by default**: Non-root execution, dropped capabilities, resource limits
- **🎯 Multiple debugging modes**: Standalone pods, pod copies, or ephemeral containers
- **🛠️ Advanced management**: List, clean, and manage debug pods with ease
- **🔧 Auto-completion**: Full shell completion support (bash, zsh, fish, PowerShell)
- **📊 Multiple output formats**: Table, JSON, and YAML output for automation
- **⚡ Smart error handling**: Clear, actionable error messages with suggestions
- **🔐 Security profiles**: Configurable security contexts (restricted, baseline, privileged)
- **🏷️ Smart labeling**: Automatic pod discovery and cleanup via labels

## 📦 Installation

### Using Go

```bash
go install github.com/the-kernel-panics/k8s-pods-debug/cmd/kpdbug@latest
```

### Using Binary Releases

Download the latest release from the [releases page](https://github.com/the-kernel-panics/k8s-pods-debug/releases).

### Shell Completion (Recommended)

Enable auto-completion for your shell:

```bash
# Bash
kpdbug completion bash > /usr/local/etc/bash_completion.d/kpdbug

# Zsh
kpdbug completion zsh > "${fpath[1]}/_kpdbug"

# Fish
kpdbug completion fish > ~/.config/fish/completions/kpdbug.fish
```

## 🚀 Usage

### Quick Start

```bash
# Create a standalone debug pod
kpdbug -it

# Debug an existing pod (ephemeral container)
kpdbug -p my-app-pod -it

# Create a pod copy for debugging
kpdbug -p my-app-pod --copy -it
```

### 🎯 Debugging Modes

#### 1. **Standalone Debug Pod**
Perfect for general cluster debugging and troubleshooting.

```bash
kpdbug -it --image debug:latest
```

#### 2. **Pod Copy with Debug Container**
Creates an exact copy of your pod with debugging tools, preserving the original environment.

```bash
kpdbug -p <target-pod> --copy -it --image debug:latest
```

**Benefits:**
- 🔄 Exact replica of target pod environment
- 🔗 Process namespace sharing
- 🏷️ Label inheritance (minus deployment selectors)
- 🛡️ Security context preservation

#### 3. **Ephemeral Debug Container**
Adds a temporary debugging container to a running pod without restarts.

```bash
kpdbug -p <target-pod> -it --image debug:latest
```

**Benefits:**
- 🚀 No pod restarts required
- 🔗 Shared process namespace
- 🛡️ Inherited security context
- ⚡ Immediate access

### 📋 Management Commands

#### List Active Debug Pods
```bash
# List debug pods in current namespace
kpdbug list

# List across all namespaces
kpdbug list -A

# JSON output for automation
kpdbug list -o json
```

#### Clean Up Debug Pods
```bash
# Interactive cleanup
kpdbug clean

# Force cleanup across all namespaces
kpdbug clean -A --force

# Clean pods older than 1 hour
kpdbug clean --older-than 1h
```

### 🏃‍♂️ Common Workflows

#### Quick Pod Debugging
```bash
# Find problematic pod
kubectl get pods | grep Error

# Start debugging session
kpdbug -p problematic-pod -it

# In debug container, inspect processes
ps aux
netstat -tulpn
```

#### Network Troubleshooting
```bash
# Create network debug pod
kpdbug -it --image nicolaka/netshoot:latest

# Test connectivity
nslookup my-service
curl -v http://my-service:8080
```

#### Resource Investigation
```bash
# Create debug pod with elevated privileges
kpdbug -it --profile privileged

# Check node resources
df -h
free -h
lsof | head -20
```

## ⚙️ Configuration

### Command Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-n, --namespace` | Target namespace | `default` |
| `-p, --pod` | Target pod name | - |
| `--image` | Debug container image | `debug:latest` |
| `-i, --stdin` | Keep stdin open | `false` |
| `-t, --tty` | Allocate TTY | `false` |
| `--rm` | Auto-remove after session | `false` |
| `--copy` | Create pod copy instead of ephemeral container | `false` |
| `--profile` | Security profile | `general` |
| `--memory-limit` | Memory limit | `128Mi` |
| `--cpu-request` | CPU request | `100m` |
| `--memory-request` | Memory request | `128Mi` |
| `-f, --force` | Force action without prompts | `false` |

### Security Profiles

Choose the appropriate security profile for your debugging needs:

| Profile | Use Case | Security Level |
|---------|----------|----------------|
| `restricted` | Production debugging | 🔒 Highest - Non-root, no capabilities |
| `baseline` | Standard debugging | 🔐 High - Some restrictions |
| `general` | Development debugging | ⚖️ Balanced - Default choice |
| `privileged` | System-level debugging | ⚠️ Low - Full privileges |

```bash
# Use restricted profile for production
kpdbug -p prod-pod --profile restricted -it

# Use privileged for system debugging
kpdbug --profile privileged -it
```

## 🔒 Security Features

- **🛡️ Secure by default**: Non-root execution (UID 1000)
- **🔐 Capability management**: Dropped capabilities with profile-based control
- **🚫 Privilege escalation**: Prevention mechanisms enabled
- **📋 Seccomp profiles**: Runtime default security profiles
- **📊 Resource limits**: Memory and CPU constraints
- **💓 Health checks**: Automatic liveness and readiness probes
- **🏷️ Smart labeling**: Automatic cleanup and discovery

## 🔧 Building from Source

```bash
# Clone the repository
git clone https://github.com/the-kernel-panics/k8s-pods-debug
cd k8s-pods-debug

# Build the binary
make build

# Run tests
make test

# Install locally
go install ./cmd/kpdbug
```

### Development Setup

```bash
# Install pre-commit hooks
pre-commit install

# Run linting
golangci-lint run

# Generate completion scripts
./kpdbug completion bash > kpdbug.bash
```

## 🐛 Troubleshooting

### Common Issues

<details>
<summary><strong>❌ Permission denied errors</strong></summary>

Check your RBAC permissions:
```bash
kubectl auth can-i create pods
kubectl auth can-i create pods/ephemeralcontainers
```

Ensure you have the necessary permissions to create pods and ephemeral containers.
</details>

<details>
<summary><strong>🔌 Cannot connect to cluster</strong></summary>

Verify cluster connectivity:
```bash
kubectl cluster-info
kubectl get nodes
```

Check your kubeconfig configuration and ensure you're connected to the right cluster.
</details>

<details>
<summary><strong>⏱️ Pod creation timeout</strong></summary>

Check cluster resources:
```bash
kubectl top nodes
kubectl describe nodes
```

Ensure sufficient resources are available for pod scheduling.
</details>

## 🤝 Contributing

We welcome contributions! Here's how to get started:

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b feature/amazing-feature`
3. **Commit** your changes: `git commit -m 'Add amazing feature'`
4. **Push** to the branch: `git push origin feature/amazing-feature`
5. **Open** a Pull Request

### Development Guidelines

- Write tests for new features
- Follow Go best practices
- Update documentation
- Run `pre-commit` hooks before submitting

## 📄 License

This project is licensed under the [MIT License](LICENSE) - see the LICENSE file for details.

## 🌟 Acknowledgments

- Inspired by kubectl's built-in debug functionality
- Built with [Cobra](https://github.com/spf13/cobra) CLI framework
- Security best practices from [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/)
