# ROR 🦁  – Rootless OCI Runner

### Security Model

This container runtime implements rootless execution using Linux namespaces and user ID mapping via `newuidmap`/`newgidmap`. 

### What's Isolated:
- **User Namespace**: This is the core success of the runtime. The container runs with a proper UID/GID mapping, where the internal `root` user is mapped to an unprivileged user on the host. This is proven by the `whoami` command returning `root`.

- **PID Namespace**: A new PID namespace is successfully created. However, because the host environment prevents the mounting of a new `/proc` filesystem, tools like `ps` inside the container still read the host's `/proc` and see all host processes.

- **UTS Namespace**: A new UTS namespace is created, giving the container the *potential* for its own hostname. However, the initial `sethostname` call is blocked by host security policies, so the container inherits the host's name by default.

- **Mount Namespace**: A new mount namespace is created, but it is not utilized. The container inherits the host's view of the filesystem mounts.


### Known Limitation: Filesystem Isolation

**Important**: ... This means **the container can access the host filesystem**. **As a result, this runtime uses a simple `Chdir` into the rootfs instead of `chroot` as a pragmatic workaround.** This is a fundamental limitation...

**Important**: Due to kernel security restrictions in modern Linux (kernel 5.x+ with LSM modules like AppArmor), unprivileged user namespaces cannot perform:
- `chroot()` system calls
- `pivot_root()` operations  
- Bind mounts to create new root filesystems

This means **the container can access the host filesystem**. This is a fundamental limitation of truly unprivileged containers, not a bug in this implementation.

### Why This Happens

The Linux kernel prevents these operations to avoid privilege escalation vulnerabilities. Even with proper UID mapping via `newuidmap` (a setuid helper), the kernel still restricts filesystem-altering operations in user namespaces.


### Security Implications

Without filesystem isolation:
- Container processes can read host files (subject to DAC permissions)
- Container cannot truly hide host filesystem structure
- Path traversal attacks are possible if container apps are malicious

**Recommendation**: Use this for development/testing, not production isolation of untrusted workloads.
