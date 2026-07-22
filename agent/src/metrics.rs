use sysinfo::{System, CpuRefreshKind};
use tracing::debug;

/// Metrics collection loop — gathers system metrics at regular intervals.
pub async fn metrics_loop(interval_seconds: u64) {
    let mut sys = System::new_all();
    let mut interval = tokio::time::interval(
        std::time::Duration::from_secs(interval_seconds),
    );

    loop {
        interval.tick().await;

        // Refresh all metrics
        sys.refresh_cpu_specifics(CpuRefreshKind::everything());
        sys.refresh_memory();

        let cpu_usage = sys.global_cpu_usage();
        let memory_total = sys.total_memory();
        let memory_used = sys.used_memory();
        let _memory_available = sys.available_memory();

        // Per-CPU usage
        let per_cpu: Vec<f32> = sys.cpus().iter()
            .map(|cpu| cpu.cpu_usage())
            .collect();

        // Load averages (Linux/macOS only, returns [0,0,0] on Windows)
        let load_avg = System::load_average();

        debug!(
            "Metrics: CPU={:.1}%, MEM={}/{} ({:.1}%), Load={:.2}/{:.2}/{:.2}, CPUs={:?}",
            cpu_usage,
            format_bytes(memory_used),
            format_bytes(memory_total),
            if memory_total > 0 { (memory_used as f64 / memory_total as f64) * 100.0 } else { 0.0 },
            load_avg.one, load_avg.five, load_avg.fifteen,
            per_cpu.iter().map(|u| format!("{:.0}%", u)).collect::<Vec<_>>()
        );
    }
}

fn format_bytes(bytes: u64) -> String {
    const KB: u64 = 1024;
    const MB: u64 = KB * 1024;
    const GB: u64 = MB * 1024;

    if bytes >= GB {
        format!("{:.1}GB", bytes as f64 / GB as f64)
    } else if bytes >= MB {
        format!("{:.1}MB", bytes as f64 / MB as f64)
    } else if bytes >= KB {
        format!("{:.1}KB", bytes as f64 / KB as f64)
    } else {
        format!("{}B", bytes)
    }
}
