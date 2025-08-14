# ROR 🦁 – Rootless OCI Runner

`ror` is a minimal, educational OCI container runtime written in Go from scratch. The purpose of this project is to explore and implement the core Linux technologies that power modern containers, with a special focus on achieving **rootless** execution.

It demonstrates how to build a container runtime that can launch a container with an internal `root` user, all without requiring any `sudo` privileges on the host machine.

---

### Features

* **OCI Bundle Compliant:** Runs containers from standard OCI-compliant filesystem bundles.
* **Rootless Execution:** Utilizes **User Namespaces** and external helpers (`newuidmap`/`newgidmap`) to map an unprivileged host user to the container's root user.
* **Process & Hostname Isolation:** Creates new **PID** and **UTS** namespaces to give the container its own process tree and hostname identity.
* **Basic Container Lifecycle:** Supports a complete `create`, `start`, and `delete` workflow for managing containers.

---

### Demo
![ror-demo2](https://github.com/user-attachments/assets/a31983f0-e877-4960-ac7d-25c91c5808ed)

---

### How It Works

The runtime is built around a parent/child process model:
1.  The **parent process** (`ror start`) uses the `syscall.Clone` function with specific flags (`CLONE_NEWUSER`, `CLONE_NEWPID`, etc.) to spawn a new, isolated child process.
2.  The parent then configures the user ID mapping for the child, giving it root privileges inside its new user namespace.
3.  The **child process** acts as the container's `init` process. It waits for the parent to finish setup, then executes the command specified in the OCI `config.json`.
4.  Synchronization between the parent and child is managed using a simple pipe.

---

### Quick Start

#### Prerequisites
* Go 1.18+ & Make
* `git`
* `newuidmap` and `newgidmap` (usually installed via the `uidmap` package)
* A configured subordinate UID/GID range for your user (in `/etc/subuid` and `/etc/subgid`).

#### Build
```bash
make
```

#### Get an OCI Image Bundle
```bash
# sudo apt install skopeo
mkdir alpine-bundle
skopeo copy docker://alpine:latest oci:alpine-bundle:latest
```

#### Run a Container
```bash
# Create the container state
./ror create my-alpine --bundle ./alpine-bundle

# Start the container
./ror start my-alpine

# You will now be in a shell inside the container.
# Verify you are root:
whoami

# In another terminal, delete the container:
./ror delete my-alpine
```

--- 

### Limitations & Security Model
This runtime is a learning tool and has significant security limitations compared to production runtimes like Podman or Docker.

- `No Filesystem Isolation:` The most important limitation is that the container can access the host filesystem. This is because modern Linux kernels, often hardened with security modules like AppArmor, prevent unprivileged users from using the chroot() or pivot_root() syscalls needed for true filesystem jailing. As a workaround, ror simply uses Chdir to change into the container's rootfs.
- `No Network Isolation:` The container currently shares the host's network.
- `Partial PID Isolation:` While a new PID namespace is created, tools like ps will still see host processes because a private /proc filesystem is not mounted (this is also blocked by host security policies).
- `Partial UTS Isolation:` A new UTS namespace is created, but the sethostname call is blocked, so the container inherits the host's name by default.

### What's Isolated:
- **`User Namespace`**: This is the core success of the runtime. The container runs with a proper UID/GID mapping, where the internal `root` user is mapped to an unprivileged user on the host. This is proven by the `whoami` command returning `root`.

- **`PID Namespace`**: A new PID namespace is successfully created. However, because the host environment prevents the mounting of a new `/proc` filesystem, tools like `ps` inside the container still read the host's `/proc` and see all host processes.

- **`UTS Namespace`**: A new UTS namespace is created, giving the container the *potential* for its own hostname. However, the initial `sethostname` call is blocked by host security policies, so the container inherits the host's name by default.

- **`Mount Namespace`**: A new mount namespace is created, but it is not utilized. The container inherits the host's view of the filesystem mounts.


### Security Implications

Without filesystem isolation:
- Container processes can read host files (subject to DAC permissions)
- Container cannot truly hide host filesystem structure
- Path traversal attacks are possible if container apps are malicious

> **Recommendation**: Use this for development/testing, not production isolation of untrusted workloads.
