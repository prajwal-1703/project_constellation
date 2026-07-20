# Contributing to Constellation

First off, thank you for considering contributing to Constellation! It's people like you that make Constellation such a great tool.

## 🧠 Understanding the Codebase

Constellation uses a multi-language architecture to take advantage of the best tools for the job:

*   **`controller/` (Go)**: The cluster manager. It handles scheduling, state management via embedded SQLite, and exposes a REST API.
*   **`agent/` (Rust)**: The worker node daemon. Written in Rust for safety and performance, it heavily utilizes the `cgroups-rs` crate to enforce strict Linux CPU/Memory isolation.
*   **`cli/` (Go)**: The command-line tool for users to interact with the controller.
*   **`dashboard/` (React/Node.js)**: A sleek web interface for monitoring the cluster.
*   **`proto/`**: Protocol Buffer definitions for gRPC communication between the Controller and Agents.
*   **`deploy/`**: Scripts and files (`build_ubuntu.sh`, `install.sh`) for packaging and deployment.

## 🛠️ Setting Up Your Development Environment

To work on all parts of the system, you will need the following installed:
1.  **Go (1.21+)**: For `controller` and `cli`.
2.  **Rust & Cargo**: For `agent`.
3.  **Node.js (18+) & npm**: For `dashboard`.
4.  **Protobuf Compiler (`protoc`)**: For regenerating gRPC code if you modify the `.proto` files.
5.  **C Build Tools**: (`build-essential`, `libc6-dev`, `gcc` on Ubuntu) for compiling the Rust agent's cgroup bindings.

### Running Components Locally

**Controller:**
```bash
cd controller
go run main.go
```
*The controller will listen on port 8080 for HTTP and 9090 for gRPC by default.*

**Agent:**
```bash
cd agent
cargo build
sudo cargo run # Requires sudo to manage Linux cgroups!
```
*Make sure the controller is running before starting the agent so it can connect via gRPC.*

**Dashboard:**
```bash
cd dashboard
npm install
npm run dev
```
*The dashboard will connect to the controller's REST API at `http://localhost:8080`.*

## 📝 How to Contribute

### 1. Reporting Bugs
*   Ensure the bug was not already reported by searching existing issues.
*   Open a new issue with a clear title and description.
*   Provide detailed steps to reproduce the issue, your OS, and the version of Constellation you are using. Include logs if possible.

### 2. Suggesting Enhancements
*   Open an issue to discuss the proposed change before putting in significant effort.
*   Explain the problem your enhancement solves and how it will benefit the ecosystem.
*   If you plan to implement the feature yourself, indicate that in the issue!

### 3. Submitting Pull Requests
1.  **Fork the repository** and create your branch from `main`.
2.  **Write clear code** and include comments where necessary. Use meaningful variable names and keep functions focused.
3.  **Follow styling guidelines**:
    *   *Go*: Run `gofmt` and `go vet`. Ensure your code matches standard Go idioms.
    *   *Rust*: Run `cargo fmt` and `cargo clippy`. Resolve any warnings before committing.
    *   *React*: Run `npm run lint` and ensure there are no errors in the console.
4.  **Update documentation** if your changes affect user-facing features, CLI commands, or APIs. Don't forget to update the `README.md` if necessary.
5.  **Test your changes** thoroughly. Ensure the agent can still accurately isolate resources via cgroups if you touched that area.
6.  **Create the Pull Request** with a detailed description of what you changed, why you changed it, and how to test it.

## 🛡️ Code of Conduct

By participating in this project, you agree to abide by common open-source standards of conduct. Be respectful, inclusive, and constructive in discussions and code reviews. We are here to learn and build together!
