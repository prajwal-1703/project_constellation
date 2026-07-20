use sysinfo::{System, Disks, Networks, CpuRefreshKind};
use serde::{Deserialize, Serialize};

/// Comprehensive hardware information about this machine.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HardwareInfo {
    pub cpu_model: String,
    pub cpu_vendor: String,
    pub cpu_cores: u32,
    pub cpu_threads: u32,
    pub cpu_freq_mhz: f64,
    pub arch: String,
    pub memory_total: u64,
    pub memory_available: u64,
    pub disk_total: u64,
    pub disk_available: u64,
    pub gpu_count: u32,
    pub gpu_memory: u64,
    pub os_name: String,
    pub os_version: String,
    pub kernel_version: String,
    pub hostname: String,
    pub uptime: u64,
    pub disks: Vec<DiskDetail>,
    pub networks: Vec<NetworkDetail>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DiskDetail {
    pub name: String,
    pub mount_point: String,
    pub filesystem: String,
    pub total_bytes: u64,
    pub available_bytes: u64,
    pub disk_type: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NetworkDetail {
    pub name: String,
    pub mac_address: String,
    pub is_up: bool,
}

/// Discover all hardware information about this machine.
/// Uses the `sysinfo` crate for cross-platform support (Linux, macOS, Windows).
pub fn discover_hardware() -> HardwareInfo {
    let mut sys = System::new_all();
    sys.refresh_all();
    sys.refresh_cpu_specifics(CpuRefreshKind::everything());

    // CPU info
    let cpus = sys.cpus();
    let cpu_model = cpus.first()
        .map(|c| c.brand().to_string())
        .unwrap_or_else(|| "Unknown".to_string());
    let cpu_vendor = cpus.first()
        .map(|c| c.vendor_id().to_string())
        .unwrap_or_else(|| "Unknown".to_string());
    let cpu_threads = cpus.len() as u32;
    let cpu_cores = sys.physical_core_count().unwrap_or(cpu_threads as usize) as u32;
    let cpu_freq_mhz = cpus.first()
        .map(|c| c.frequency() as f64)
        .unwrap_or(0.0);

    // Memory info
    let memory_total = sys.total_memory();
    let memory_available = sys.available_memory();

    // Disk info
    let disk_sys = Disks::new_with_refreshed_list();
    let mut disk_total: u64 = 0;
    let mut disk_available: u64 = 0;
    let mut disk_details = Vec::new();

    for disk in disk_sys.list() {
        let total = disk.total_space();
        let avail = disk.available_space();
        disk_total += total;
        disk_available += avail;

        let disk_type = if disk.is_removable() {
            "Removable"
        } else {
            match disk.kind() {
                sysinfo::DiskKind::SSD => "SSD",
                sysinfo::DiskKind::HDD => "HDD",
                _ => "Unknown",
            }
        };

        disk_details.push(DiskDetail {
            name: disk.name().to_string_lossy().to_string(),
            mount_point: disk.mount_point().to_string_lossy().to_string(),
            filesystem: disk.file_system().to_string_lossy().to_string(),
            total_bytes: total,
            available_bytes: avail,
            disk_type: disk_type.to_string(),
        });
    }

    // Network info
    let networks = Networks::new_with_refreshed_list();
    let mut network_details = Vec::new();
    for (name, data) in networks.list() {
        network_details.push(NetworkDetail {
            name: name.clone(),
            mac_address: data.mac_address().to_string(),
            is_up: true, // sysinfo doesn't provide is_up directly
        });
    }

    // OS info
    let os_name = System::name().unwrap_or_else(|| "Unknown".to_string());
    let os_version = System::os_version().unwrap_or_else(|| "Unknown".to_string());
    let kernel_version = System::kernel_version().unwrap_or_else(|| "Unknown".to_string());
    let hostname_str = System::host_name().unwrap_or_else(|| "Unknown".to_string());
    let uptime = System::uptime();
    let arch = std::env::consts::ARCH.to_string();

    let (gpu_count, gpu_memory) = detect_gpus();

    HardwareInfo {
        cpu_model,
        cpu_vendor,
        cpu_cores,
        cpu_threads,
        cpu_freq_mhz,
        arch,
        memory_total,
        memory_available,
        disk_total,
        disk_available,
        gpu_count,
        gpu_memory,
        os_name,
        os_version,
        kernel_version,
        hostname: hostname_str,
        uptime,
        disks: disk_details,
        networks: network_details,
    }
}

/// Collect installed software versions by checking common tools.
pub fn detect_installed_software() -> Vec<SoftwareInfo> {
    let tools = vec![
        ("python3", "--version"),
        ("python", "--version"),
        ("node", "--version"),
        ("npm", "--version"),
        ("docker", "--version"),
        ("gcc", "--version"),
        ("g++", "--version"),
        ("rustc", "--version"),
        ("cargo", "--version"),
        ("go", "version"),
        ("java", "--version"),
        ("git", "--version"),
        ("make", "--version"),
        ("cmake", "--version"),
    ];

    let mut software = Vec::new();

    for (cmd, flag) in tools {
        if let Ok(output) = std::process::Command::new(cmd)
            .arg(flag)
            .output()
        {
            if output.status.success() {
                let version_str = String::from_utf8_lossy(&output.stdout)
                    .lines()
                    .next()
                    .unwrap_or("")
                    .trim()
                    .to_string();

                if !version_str.is_empty() {
                    software.push(SoftwareInfo {
                        name: cmd.to_string(),
                        version: version_str,
                    });
                }
            }
        }
    }

    software
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SoftwareInfo {
    pub name: String,
    pub version: String,
}

pub fn detect_gpus() -> (u32, u64) {
    let mut count = 0;
    let mut mem_total = 0;
    
    if let Ok(output) = std::process::Command::new("nvidia-smi")
        .args(&["--query-gpu=memory.total", "--format=csv,noheader,nounits"])
        .output()
    {
        if output.status.success() {
            let out_str = String::from_utf8_lossy(&output.stdout);
            for line in out_str.lines() {
                if let Ok(mem) = line.trim().parse::<u64>() {
                    count += 1;
                    mem_total += mem * 1024 * 1024; // MiB to Bytes
                }
            }
        }
    }
    
    (count, mem_total)
}
