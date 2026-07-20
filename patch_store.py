

file_path = r'e:\Projects\project_constellation\controller\state\store.go'
with open(file_path, 'r') as f:
    content = f.read()

# Models
content = content.replace(
    "DiskTotal       int64     `json:\"disk_total\"`",
    "DiskTotal       int64     `json:\"disk_total\"`\n\tGPUCount        int       `json:\"gpu_count\"`\n\tGPUMemory       int64     `json:\"gpu_memory\"`"
)

content = content.replace(
    "MemoryRequired  int64     `json:\"memory_required\"`",
    "MemoryRequired  int64     `json:\"memory_required\"`\n\tGPURequired     int       `json:\"gpu_required\"`\n\tGPUMemoryRequired int64     `json:\"gpu_memory_required\"`"
)

content = content.replace(
    "Chunks          []TaskChunk `json:\"chunks,omitempty\"`\n}",
    "Chunks          []TaskChunk `json:\"chunks,omitempty\"`\n\tIsDistributed   bool      `json:\"is_distributed\"`\n\tWorldSize       int       `json:\"world_size\"`\n\tGangID          string    `json:\"gang_id,omitempty\"`\n}"
)

# Schema nodes
content = content.replace(
    "disk_total INTEGER DEFAULT 0,",
    "disk_total INTEGER DEFAULT 0,\n\t\tgpu_count INTEGER DEFAULT 0,\n\t\tgpu_memory INTEGER DEFAULT 0,"
)

# Schema tasks
content = content.replace(
    "memory_required INTEGER DEFAULT 0,",
    "memory_required INTEGER DEFAULT 0,\n\t\tgpu_required INTEGER DEFAULT 0,\n\t\tgpu_memory_required INTEGER DEFAULT 0,"
)
content = content.replace(
    "split_by TEXT DEFAULT ''\n\t);",
    "split_by TEXT DEFAULT '',\n\t\tis_distributed BOOLEAN DEFAULT 0,\n\t\tworld_size INTEGER DEFAULT 1,\n\t\tgang_id TEXT DEFAULT ''\n\t);"
)

# RegisterNode
content = content.replace(
    "cpu_freq_mhz, memory_total, disk_total,",
    "cpu_freq_mhz, memory_total, disk_total, gpu_count, gpu_memory,"
)
content = content.replace(
    "os_name, kernel_version, arch, cluster_id, joined_at, last_heartbeat, labels)",
    "os_name, kernel_version, arch, cluster_id, joined_at, last_heartbeat, labels)"
)
content = content.replace(
    "VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
    "VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
)
content = content.replace(
    "node.CPUFreqMHz, node.MemoryTotal, node.DiskTotal, node.OSName,",
    "node.CPUFreqMHz, node.MemoryTotal, node.DiskTotal, node.GPUCount, node.GPUMemory, node.OSName,"
)

# Node SELECTs (getNodeUnsafe, ListNodes, GetOnlineWorkerNodes)
# Replace all instances of `cpu_freq_mhz, memory_total, disk_total, os_name, kernel_version`
content = content.replace(
    "memory_total, disk_total, os_name, kernel_version,",
    "memory_total, disk_total, gpu_count, gpu_memory, os_name, kernel_version,"
)

# scanNode
content = content.replace(
    "&n.CPUCores, &n.CPUThreads, &n.CPUFreqMHz, &n.MemoryTotal,\n\t\t&n.DiskTotal, &n.OSName, &n.KernelVersion, &n.Arch,",
    "&n.CPUCores, &n.CPUThreads, &n.CPUFreqMHz, &n.MemoryTotal,\n\t\t&n.DiskTotal, &n.GPUCount, &n.GPUMemory, &n.OSName, &n.KernelVersion, &n.Arch,"
)

# CreateTask
content = content.replace(
    "docker_image, cpu_required, memory_required, priority, retry_max, retry_count,",
    "docker_image, cpu_required, memory_required, gpu_required, gpu_memory_required, priority, retry_max, retry_count,"
)
content = content.replace(
    "reduce_command, max_nodes, chunk_size, split_by)",
    "reduce_command, max_nodes, chunk_size, split_by, is_distributed, world_size, gang_id)"
)
content = content.replace(
    "VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
    "VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
)
content = content.replace(
    "task.Command, task.Runtime, task.DockerImage, task.CPURequired,\n\t\ttask.MemoryRequired, task.Priority, task.RetryMax, task.RetryCount,",
    "task.Command, task.Runtime, task.DockerImage, task.CPURequired,\n\t\ttask.MemoryRequired, task.GPURequired, task.GPUMemoryRequired, task.Priority, task.RetryMax, task.RetryCount,"
)
content = content.replace(
    "task.MaxNodes, task.ChunkSize, task.SplitBy,",
    "task.MaxNodes, task.ChunkSize, task.SplitBy, task.IsDistributed, task.WorldSize, task.GangID,"
)

# Task SELECTs
content = content.replace(
    "docker_image, cpu_required, memory_required, priority, retry_max,",
    "docker_image, cpu_required, memory_required, gpu_required, gpu_memory_required, priority, retry_max,"
)
content = content.replace(
    "max_nodes, chunk_size, split_by\n",
    "max_nodes, chunk_size, split_by, is_distributed, world_size, gang_id\n"
)

# scanTask
content = content.replace(
    "&t.Runtime, &t.DockerImage, &t.CPURequired, &t.MemoryRequired,\n\t\t&t.Priority, &t.RetryMax, &t.RetryCount, &t.TimeoutSeconds,",
    "&t.Runtime, &t.DockerImage, &t.CPURequired, &t.MemoryRequired, &t.GPURequired, &t.GPUMemoryRequired,\n\t\t&t.Priority, &t.RetryMax, &t.RetryCount, &t.TimeoutSeconds,"
)
content = content.replace(
    "&t.ReduceCommand, &t.MaxNodes, &t.ChunkSize, &t.SplitBy,\n\t)",
    "&t.ReduceCommand, &t.MaxNodes, &t.ChunkSize, &t.SplitBy, &t.IsDistributed, &t.WorldSize, &t.GangID,\n\t)"
)

# Default world_size if IsDistributed is false? Let's handle it in CreateTask
content = content.replace(
    "if task.Type == \"\" {\n\t\ttask.Type = \"simple\"\n\t}",
    "if task.Type == \"\" {\n\t\ttask.Type = \"simple\"\n\t}\n\tif task.WorldSize == 0 {\n\t\ttask.WorldSize = 1\n\t}"
)

with open(file_path, 'w') as f:
    f.write(content)
print("store.go updated successfully!")
