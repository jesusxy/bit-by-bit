# Rootless Container Implementation

## Security Model

This container runtime implements rootless execution using Linux namespaces and user ID mapping via `newuidmap`/`newgidmap`. 

### What's Isolated:
- **Process Namespace (PID)**: Container processes cannot see host processes
- **UTS Namespace**: Container has its own hostname
- **User Namespace**: UID/GID mapping (container root = unprivileged host user)
- **Mount Namespace**: Separate mount point view (though limited operations allowed)

### Known Limitation: Filesystem Isolation

**Important**: Due to kernel security restrictions in modern Linux (kernel 5.x+ with LSM modules like AppArmor), unprivileged user namespaces cannot perform:
- `chroot()` system calls
- `pivot_root()` operations  
- Bind mounts to create new root filesystems

This means **the container can access the host filesystem**. This is a fundamental limitation of truly unprivileged containers, not a bug in this implementation.

### Why This Happens

The Linux kernel prevents these operations to avoid privilege escalation vulnerabilities. Even with proper UID mapping via `newuidmap` (a setuid helper), the kernel still restricts filesystem-altering operations in user namespaces.

### Industry Context

Popular "rootless" container runtimes work around this by:
- **Podman/Docker**: Use additional setuid helpers or run partially privileged
- **gVisor**: Intercept syscalls with a user-space kernel
- **Kata**: Use lightweight VMs

This implementation demonstrates the real limitations of unprivileged containers and why true security requires defense in depth.

### Security Implications

Without filesystem isolation:
- Container processes can read host files (subject to DAC permissions)
- Container cannot truly hide host filesystem structure
- Path traversal attacks are possible if container apps are malicious

**Recommendation**: Use this for development/testing, not production isolation of untrusted workloads.