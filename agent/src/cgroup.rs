use std::process::Child;

#[cfg(target_os = "linux")]
pub fn setup_cgroup(task_id: &str, cpu_cores: u32, memory_bytes: u64, pid: u32) -> Result<(), String> {
    use cgroups_rs::{cgroup_builder::CgroupBuilder, hierarchies, CgroupPid, MaxValue};
    
    let cgroup_name = format!("constellation-{}", task_id);
    let hier = hierarchies::auto();
    
    let cg = CgroupBuilder::new(&cgroup_name)
        .cpu()
            .quota((cpu_cores * 100000) as i64)
            .period(100000)
            .done()
        .memory()
            .memory_hard_limit(if memory_bytes > 0 { memory_bytes as i64 } else { -1 })
            .done()
        .build(hier)
        .map_err(|e| format!("Failed to create cgroup: {}", e))?;

    cg.add_task(CgroupPid::from(pid as u64))
        .map_err(|e| format!("Failed to add PID to cgroup: {}", e))?;

    Ok(())
}

#[cfg(not(target_os = "linux"))]
pub fn setup_cgroup(task_id: &str, cpu_cores: u32, memory_bytes: u64, pid: u32) -> Result<(), String> {
    // No-op for non-Linux OS (Windows/macOS)
    println!("cgroups not supported on this OS. Skipping isolation for task {}", task_id);
    Ok(())
}

#[cfg(target_os = "linux")]
pub fn cleanup_cgroup(task_id: &str) {
    use cgroups_rs::{cgroup_builder::CgroupBuilder, hierarchies};
    let cgroup_name = format!("constellation-{}", task_id);
    let hier = hierarchies::auto();
    if let Ok(cg) = cgroups_rs::cgroup::Cgroup::load(hier, &cgroup_name) {
        let _ = cg.delete();
    }
}

#[cfg(not(target_os = "linux"))]
pub fn cleanup_cgroup(_task_id: &str) {
    // No-op
}
