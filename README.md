# Constellation 🚀

Constellation is a lightweight, zero-dependency distributed task orchestration cluster. It allows you to reliably schedule, isolate, and run tasks across a network of worker nodes. Built for simplicity and performance, Constellation natively leverages Linux cgroups for strict resource isolation and embeds SQLite for persistent state management, completely removing the need for external databases.

## 🌟 Key Features

*   **Zero-Dependency Deployment**: Compiled into self-contained binaries. Just drop them onto a fresh Ubuntu server and you're good to go.
*   **Strict Resource Isolation**: The Rust-based worker agent uses Linux cgroups to enforce strict CPU and Memory limits for tasks.
*   **Intelligent Scheduling**: The controller automatically scores connected nodes and assigns tasks to the most optimal worker.
*   **Multi-Interface Access**: Interact with the cluster via a powerful CLI, a REST API, or monitor it through a sleek React web dashboard.
*   **Built-in Persistence**: Controller state is securely managed using an embedded SQLite database.
*   **Secure Communication**: Utilizes gRPC with TLS between the controller and worker nodes (port 9090).

## 🏗️ Architecture overview

The Constellation ecosystem consists of four main components:

1.  **Controller (Go)**: The central brain of the cluster. It embeds an SQLite database, exposes a REST API (port 8080), manages node states, and schedules tasks.
2.  **Agent (Rust)**: Runs on the worker nodes. It executes tasks and strictly enforces CPU/Memory limits using Linux cgroups (`cgroups-rs`).
3.  **CLI (Go)**: A command-line interface tool to easily submit and manage tasks from your terminal.
4.  **Dashboard (React/Node.js)**: A read-only web user interface to view node status and task history.

## ⚙️ Prerequisites

### Runtime Dependencies (For Production Servers)
*   **Linux OS (Ubuntu 20.04+, Debian, CentOS)**: Required for Linux cgroups support.
*   **Systemd**: For managing the background services.

### Build-Time Dependencies (For Compiling)
*   **Go (1.21+)**: For compiling the Controller and CLI.
*   **Node.js (18+) & npm**: For bundling the React dashboard.
*   **Rust (Cargo)**: For compiling the worker Agent.
*   **C Build Tools (`build-essential`)**: Specifically `libc6-dev` and `gcc` for the Rust `cgroups-rs` crate bindings.

## 🚀 Installation & Deployment

We provide a streamlined deployment script for Ubuntu environments.

1.  **Clone & Build**:
    Clone the repository on your target build machine and run the packaging script:
    ```bash
    chmod +x build_ubuntu.sh
    ./build_ubuntu.sh
    ```
    This script cross-compiles all components and generates a deployment package in the `release/` directory (e.g., `release/constellation-v2.0-ubuntu.tar.gz`).

2.  **Deploy on the Server**:
    Transfer the `.tar.gz` package to your production server and install it:
    ```bash
    tar -xzvf constellation-v2.0-ubuntu.tar.gz
    cd constellation-v2.0-ubuntu
    sudo ./install.sh
    ```
    The installer copies binaries to `/opt/constellation/bin/`, adds the CLI to your `$PATH`, and configures the systemd services.

3.  **Start the Services**:
    ```bash
    sudo systemctl enable --now constellation-controller
    sudo systemctl enable --now constellation-agent
    ```
    *Tip: View logs using `sudo journalctl -fu constellation-controller` or `constellation-agent`.*

## 💻 Usage

Once installed, there are a few ways to assign and run tasks on the Constellation cluster.

### 1. Using the Constellation CLI (Recommended)

First, authenticate your session:
```bash
constellation login
```

**Basic Task:**
```bash
constellation run --cmd "echo 'Hello World'"
```

**Task with Resource Constraints (enforced by cgroups):**
```bash
constellation run --cmd "python train.py" --cpu 4 --memory 8GB
```

**High Priority Task:**
```bash
constellation run --cmd "make build" --priority high
```

**Pinning a Task to a Specific Node:**
```bash
constellation run --cmd "bash update_system.sh" --node "node-12345"
```

### 2. Using the REST API

For CI/CD or automated pipelines, you can hit the Controller's REST API using your JWT token.

```http
POST http://<controller-ip>:8080/api/v1/tasks
Authorization: Bearer <your-jwt-token>
Content-Type: application/json

{
  "name": "Data Processing",
  "command": "python process_data.py",
  "cpu_required": 2,
  "memory_required": "4GB",
  "priority": "normal"
}
```

## 🔐 Production Best Practices

When deploying in a real production environment, we highly recommend:
*   **TLS Certificates**: The current setup uses self-signed certs for gRPC. Replace these with valid Let's Encrypt / Certbot certificates.
*   **Reverse Proxy**: Place a reverse proxy (Nginx or Caddy) in front of the HTTP dashboard for HTTPS (SSL) termination, forwarding traffic to `http://localhost:8080`.
*   **Firewall (UFW)**: Open Port `8080` (HTTP) for users/proxy and Port `9090` (gRPC) strictly for internal communication between your Controller and Worker Nodes.

## 🗺️ Roadmap
*   **Web Dashboard Task Submission**: Currently, the dashboard is read-only. Adding task submission directly from the web UI is planned for a future release.

## 🤝 Contributing
Please read our [CONTRIBUTING.md](./CONTRIBUTING.md) for details on our code of conduct, and the process for submitting pull requests to us.
