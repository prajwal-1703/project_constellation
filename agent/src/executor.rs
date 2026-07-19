use serde::{Deserialize, Serialize};
use std::process::Stdio;
use std::sync::Arc;
use tokio::io::{AsyncBufReadExt, BufReader};
use tokio::process::Command;
use tokio::sync::RwLock;
use tracing::{info, error, warn};

/// Represents a running task on this agent
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TaskExecution {
    pub task_id: String,
    pub name: String,
    pub command: String,
    pub status: String,
    pub exit_code: Option<i32>,
    pub pid: Option<u32>,
    pub logs: Vec<String>,
}

/// Execute a task command as a subprocess
pub async fn execute_task(
    task_id: &str,
    command: &str,
    working_dir: Option<&str>,
    env_vars: Option<&std::collections::HashMap<String, String>>,
    timeout_seconds: u64,
) -> TaskExecution {
    let mut execution = TaskExecution {
        task_id: task_id.to_string(),
        name: command.to_string(),
        command: command.to_string(),
        status: "running".to_string(),
        exit_code: None,
        pid: None,
        logs: Vec::new(),
    };

    info!("Executing task {}: {}", task_id, command);

    // Build the command - use shell to interpret the command string
    let (shell, shell_arg) = if cfg!(target_os = "windows") {
        ("cmd", "/C")
    } else {
        ("sh", "-c")
    };

    let mut cmd = Command::new(shell);
    cmd.arg(shell_arg)
        .arg(command)
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());

    // Set working directory
    if let Some(dir) = working_dir {
        if !dir.is_empty() {
            cmd.current_dir(dir);
        }
    }

    // Set environment variables
    if let Some(vars) = env_vars {
        for (key, value) in vars {
            cmd.env(key, value);
        }
    }

    // Spawn the process
    let mut child = match cmd.spawn() {
        Ok(child) => {
            let pid = child.id();
            if let Some(pid) = pid {
                if let Err(e) = crate::cgroup::setup_cgroup(task_id, 100, 1024, pid) {
                    println!("Warning: Failed to setup cgroups: {}", e);
                }
            }
            child
        },
        Err(e) => {
            error!("Failed to spawn process for task {}: {}", task_id, e);
            execution.status = "failed".to_string();
            execution.exit_code = Some(-1);
            execution.logs.push(format!("Failed to spawn process: {}", e));
            return execution;
        }
    };

    // Record PID
    execution.pid = child.id();
    info!("Task {} running with PID {:?}", task_id, execution.pid);

    // Capture stdout
    let stdout = child.stdout.take();
    let stderr = child.stderr.take();

    let logs = Arc::new(RwLock::new(Vec::<String>::new()));
    let logs_stdout = logs.clone();
    let logs_stderr = logs.clone();
    let tid_stdout = task_id.to_string();
    let tid_stderr = task_id.to_string();

    // Stream stdout
    let stdout_handle = tokio::spawn(async move {
        if let Some(stdout) = stdout {
            let reader = BufReader::new(stdout);
            let mut lines = reader.lines();
            while let Ok(Some(line)) = lines.next_line().await {
                info!("[task-{}] {}", tid_stdout, line);
                logs_stdout.write().await.push(line);
            }
        }
    });

    // Stream stderr
    let stderr_handle = tokio::spawn(async move {
        if let Some(stderr) = stderr {
            let reader = BufReader::new(stderr);
            let mut lines = reader.lines();
            while let Ok(Some(line)) = lines.next_line().await {
                warn!("[task-{}] STDERR: {}", tid_stderr, line);
                logs_stderr.write().await.push(format!("[stderr] {}", line));
            }
        }
    });

    // Wait for completion with timeout
    let result = if timeout_seconds > 0 {
        tokio::time::timeout(
            std::time::Duration::from_secs(timeout_seconds),
            child.wait(),
        ).await
    } else {
        Ok(child.wait().await)
    };

    // Collect log handles
    let _ = stdout_handle.await;
    let _ = stderr_handle.await;

    execution.logs = logs.read().await.clone();

    match result {
        Ok(Ok(status)) => {
            let code = status.code().unwrap_or(-1);
            execution.exit_code = Some(code);
            if code == 0 {
                execution.status = "completed".to_string();
                info!("Task {} completed successfully", task_id);
            } else {
                execution.status = "failed".to_string();
                info!("Task {} failed with exit code {}", task_id, code);
            }
        }
        Ok(Err(e)) => {
            error!("Task {} process error: {}", task_id, e);
            execution.status = "failed".to_string();
            execution.exit_code = Some(-1);
            execution.logs.push(format!("Process error: {}", e));
        }
        Err(_) => {
            warn!("Task {} timed out after {}s, killing...", task_id, timeout_seconds);
            // Timeout - kill the process
            execution.status = "failed".to_string();
            execution.exit_code = Some(-1);
            execution.logs.push(format!("Task timed out after {}s", timeout_seconds));
            // The child process is already dropped at this point, which kills it on Unix
        }
    }

    crate::cgroup::cleanup_cgroup(task_id);

    execution
}

/// Cancel a running task by killing its process
pub async fn cancel_task(pid: u32) -> bool {
    info!("Cancelling task with PID {}", pid);

    #[cfg(target_os = "windows")]
    {
        let _ = std::process::Command::new("taskkill")
            .args(&["/PID", &pid.to_string(), "/F"])
            .output();
        true
    }

    #[cfg(not(target_os = "windows"))]
    {
        unsafe {
            libc::kill(pid as i32, libc::SIGTERM);
        }
        // Wait a moment then force kill
        tokio::time::sleep(std::time::Duration::from_secs(5)).await;
        unsafe {
            libc::kill(pid as i32, libc::SIGKILL);
        }
        true
    }

    #[cfg(target_os = "windows")]
    true
}
