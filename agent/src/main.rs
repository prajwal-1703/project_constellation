mod discovery;
mod executor;
mod health;
mod metrics;

use clap::Parser;
use std::sync::Arc;
use tokio::sync::RwLock;
use tracing::{info, error, warn};

/// Constellation Node Agent — runs on every worker machine
#[derive(Parser, Debug)]
#[command(name = "constellation-agent")]
#[command(about = "Constellation Node Agent — hardware discovery, task execution, health reporting")]
struct Args {
    /// Controller address (e.g., http://192.168.1.10:8080)
    #[arg(long, default_value = "http://localhost:8080")]
    controller: String,

    /// Agent listen port for incoming gRPC connections
    #[arg(long, default_value_t = 9091)]
    port: u16,

    /// Join token for cluster authentication
    #[arg(long, default_value = "")]
    token: String,

    /// Node ID (auto-generated if empty)
    #[arg(long, default_value = "")]
    node_id: String,

    /// Heartbeat interval in seconds
    #[arg(long, default_value_t = 5)]
    heartbeat_interval: u64,

    /// Metrics collection interval in seconds
    #[arg(long, default_value_t = 2)]
    metrics_interval: u64,
}

/// Shared state for the agent
pub struct AgentState {
    pub node_id: String,
    pub controller_url: String,
    pub token: String,
    pub cluster_id: Option<String>,
    pub cluster_name: Option<String>,
    pub running_tasks: u32,
    pub is_registered: bool,
}

#[tokio::main]
async fn main() {
    // Initialize tracing/logging
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| "info".into()),
        )
        .with_target(false)
        .init();

    let args = Args::parse();

    info!("╔══════════════════════════════════════════════╗");
    info!("║       ✦ Constellation Node Agent ✦          ║");
    info!("║       Hardware · Execution · Health          ║");
    info!("╚══════════════════════════════════════════════╝");

    // Discover hardware
    info!("Discovering hardware...");
    let hw_info = discovery::discover_hardware();
    info!("  CPU: {} ({} cores, {} threads)",
        hw_info.cpu_model, hw_info.cpu_cores, hw_info.cpu_threads);
    info!("  Memory: {} MB total",
        hw_info.memory_total / (1024 * 1024));
    info!("  OS: {} {} ({})",
        hw_info.os_name, hw_info.kernel_version, hw_info.arch);

    // Initialize state
    let state = Arc::new(RwLock::new(AgentState {
        node_id: if args.node_id.is_empty() {
            format!("node-{}", &uuid::Uuid::new_v4().to_string()[..8])
        } else {
            args.node_id.clone()
        },
        controller_url: args.controller.clone(),
        token: args.token.clone(),
        cluster_id: None,
        cluster_name: None,
        running_tasks: 0,
        is_registered: false,
    }));

    // Register with controller
    info!("Connecting to controller at {}...", args.controller);
    match register_with_controller(&args.controller, &args.token, &hw_info, &state).await {
        Ok(_) => {
            let s = state.read().await;
            info!("  ✓ Registered as {} in cluster '{}'",
                s.node_id,
                s.cluster_name.as_deref().unwrap_or("unknown"));
        }
        Err(e) => {
            warn!("  ⚠ Failed to register with controller: {}", e);
            warn!("  Agent will continue running and retry registration...");
        }
    }

    // Start background services
    let state_heartbeat = state.clone();
    let controller_url = args.controller.clone();
    let heartbeat_interval = args.heartbeat_interval;

    // Heartbeat loop
    let heartbeat_handle = tokio::spawn(async move {
        health::heartbeat_loop(
            state_heartbeat,
            &controller_url,
            heartbeat_interval,
        ).await;
    });

    // Metrics collection loop
    let metrics_handle = tokio::spawn(async move {
        metrics::metrics_loop(args.metrics_interval).await;
    });

    info!("Agent is running. Press Ctrl+C to stop.");

    // Wait for shutdown signal
    tokio::signal::ctrl_c().await.expect("Failed to listen for Ctrl+C");
    info!("Shutting down agent...");

    heartbeat_handle.abort();
    metrics_handle.abort();

    info!("Agent stopped.");
}

/// Register this agent with the controller
async fn register_with_controller(
    controller_url: &str,
    token: &str,
    hw: &discovery::HardwareInfo,
    state: &Arc<RwLock<AgentState>>,
) -> Result<(), Box<dyn std::error::Error>> {
    let client = reqwest::Client::new();
    let hostname = hostname::get()
        .map(|h| h.to_string_lossy().to_string())
        .unwrap_or_else(|_| "unknown".to_string());

    let body = serde_json::json!({
        "hostname": hostname,
        "ip_address": get_local_ip(),
        "agent_port": 9091,
        "join_token": token,
        "cpu_model": hw.cpu_model,
        "cpu_cores": hw.cpu_cores,
        "cpu_threads": hw.cpu_threads,
        "cpu_freq_mhz": hw.cpu_freq_mhz,
        "memory_total": hw.memory_total,
        "disk_total": hw.disk_total,
        "gpu_count": hw.gpu_count,
        "gpu_memory": hw.gpu_memory,
        "os_name": hw.os_name,
        "kernel_version": hw.kernel_version,
        "arch": hw.arch,
    });

    let resp = client
        .post(format!("{}/api/v1/nodes/join", controller_url))
        .json(&body)
        .send()
        .await?;

    if !resp.status().is_success() {
        let error_body: serde_json::Value = resp.json().await.unwrap_or_default();
        let error_msg = error_body["error"]
            .as_str()
            .unwrap_or("unknown error");
        return Err(format!("Registration failed: {}", error_msg).into());
    }

    let result: serde_json::Value = resp.json().await?;
    let node_id = result["node_id"].as_str().unwrap_or("").to_string();
    let cluster_id = result["cluster_id"].as_str().unwrap_or("").to_string();
    let cluster_name = result["cluster_name"].as_str().unwrap_or("").to_string();

    let mut s = state.write().await;
    s.node_id = node_id;
    s.cluster_id = Some(cluster_id);
    s.cluster_name = Some(cluster_name);
    s.is_registered = true;

    Ok(())
}

/// Get the local IP address
fn get_local_ip() -> String {
    // Try to determine the outbound IP
    if let Ok(socket) = std::net::UdpSocket::bind("0.0.0.0:0") {
        if socket.connect("8.8.8.8:80").is_ok() {
            if let Ok(addr) = socket.local_addr() {
                return addr.ip().to_string();
            }
        }
    }
    "127.0.0.1".to_string()
}
