use std::sync::Arc;
use tokio::sync::RwLock;
use tracing::{info, warn, debug};
use sysinfo::System;

use crate::AgentState;

/// Heartbeat loop — periodically sends health status to the controller.
pub async fn heartbeat_loop(
    state: Arc<RwLock<AgentState>>,
    controller_url: &str,
    interval_seconds: u64,
) {
    let client = reqwest::Client::new();
    let mut interval = tokio::time::interval(
        std::time::Duration::from_secs(interval_seconds),
    );

    // System info for metrics
    let mut sys = System::new();

    loop {
        interval.tick().await;

        // Refresh system metrics
        sys.refresh_cpu_all();
        sys.refresh_memory();

        let cpu_usage = sys.global_cpu_usage() as f64;
        let memory_total = sys.total_memory();
        let memory_used = sys.used_memory();
        let memory_usage = if memory_total > 0 {
            (memory_used as f64 / memory_total as f64) * 100.0
        } else {
            0.0
        };

        let s = state.read().await;
        if !s.is_registered {
            debug!("Not registered, skipping heartbeat");
            continue;
        }

        let node_id = s.node_id.clone();
        let running_tasks = s.running_tasks;
        drop(s); // Release lock before HTTP call

        // Send heartbeat via REST (in production, this would be gRPC streaming)
        let heartbeat_data = serde_json::json!({
            "node_id": node_id,
            "cpu_usage": cpu_usage,
            "memory_usage": memory_usage,
            "running_tasks": running_tasks,
            "uptime": System::uptime(),
        });

        // Heartbeat is sent as a POST to update the node's last_heartbeat timestamp
        // We use the node update endpoint
        match client
            .put(format!("{}/api/v1/nodes/{}/status", controller_url, node_id))
            .json(&serde_json::json!({"status": "online"}))
            .send()
            .await
        {
            Ok(resp) => {
                if resp.status().is_success() {
                    debug!("Heartbeat sent: CPU={:.1}%, MEM={:.1}%, tasks={}",
                        cpu_usage, memory_usage, running_tasks);
                } else {
                    warn!("Heartbeat rejected: HTTP {}", resp.status());
                }
            }
            Err(e) => {
                warn!("Heartbeat failed: {} — controller may be unreachable", e);
            }
        }
    }
}
