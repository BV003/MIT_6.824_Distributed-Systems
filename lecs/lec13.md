# Lecture 13: FaRM (Fast Remote Memory) & Optimistic Concurrency Control (OCC)

## 1. What is FaRM?
FaRM is a high-performance, sharded, replicated, in-memory distributed database designed for a single data center. It achieves **millions of transactions(交易) per second** with **microsecond-level latencies** by exploiting modern hardware: **NVRAM** and **one-sided RDMA**.

---

## 2. Key Architectural Components

### A. Non-Volatile RAM (NVRAM)
* **The Problem**: Writing to standard RAM is fast (200 ns) but volatile (loses data on power failure). Writing to SSDs/Disks is slow (100 us - 10 ms).
* **FaRM's Solution**: Writes are made directly to RAM. To handle data-center-wide power failures, each rack is equipped with dedicated batteries. If power fails:
  1. Hardware notifies the software.
  2. Software halts（停止） transaction processing.
  3. The battery keeps the machine alive long enough to dump the entire RAM state onto SSDs.
  4. On reboot, the memory image is restored from the SSDs.
* **Result**: Zero-latency disk writes during transactions while still guaranteeing durability.

### B. Kernel Bypass
* Standard TCP/IP networking involves expensive system calls, memory copies, interrupts, and context switches.
* FaRM bypasses the operating system kernel entirely. Applications interact directly with the Network Interface Card (NIC) via user-level polling of DMA queues.

### C. One-Sided（单边） RDMA (Remote Direct Memory Access)
* Allows a machine's NIC to **directly read or write RAM on a remote machine** over the network.
* **Remote CPU is not involved**: The hardware NIC directly fetches/modifies cache lines atomically without interrupting the remote operating system.
* **Latency**: ~5 microseconds.

---

## 3. Optimistic Concurrency Control (OCC)
Traditional databases use pessimistic locking (Two-Phase Locking/2PL) which requires active server CPU participation to coordinate locks. Since FaRM wants to utilize one-sided RDMA (where the remote CPU is inactive), it must use **Optimistic Concurrency Control (OCC)**.

### Transaction Lifecycle:
1. **Execute Phase**:
   * Client (Transaction Coordinator/TC) reads all needed records using ultra-fast **one-sided RDMA reads**.
   * No locks are acquired during reads.
   * TC remembers the **version numbers** of the records and buffers all writes locally.
2. **Lock Phase**:
   * TC writes a `LOCK` record to the primary server of each record it intends to write.
   * The primary checks if the record is locked or if the version has changed. If neither, it sets the lock flag atomically using Compare-and-Swap (CAS).
3. **Validate Phase**:
   * For records that were only *read* (not written), the TC performs a one-sided RDMA read to re-fetch the version number and check the lock flag.
   * If any lock is active or any version number changed, the TC aborts.
4. **Commit Phase**:
   * TC writes `COMMIT-BACKUP` to backups, followed by `COMMIT-PRIMARY` to primaries.
   * The primary updates the records, increments version numbers, and releases locks.

---

## 4. FaRM vs. Spanner

| Feature | Spanner | FaRM |
| :--- | :--- | :--- |
| **Primary Goal** | Geographic replication & tolerating speed-of-light delays. | Maximizing throughput & minimizing CPU overhead. |
| **Hardware** | Standard commodity servers + GPS/Atomic clocks (TrueTime). | Specialized hardware (one-sided RDMA NICs + NVRAM). |
| **Replication** | Paxos (active CPU consensus). | Primary/Backup logging via RDMA. |
| **Concurrency** | Pessimistic Locking (2PL) + Multi-Version Read-Only transactions. | Optimistic Concurrency Control (OCC). |
| **Write Latency** | 10ms - 100ms | 50 - 100 microseconds |

---

## 5. Main Limitations of FaRM
* **Workload-Dependent**: High transaction conflict rates cause massive abort rates under OCC.
* **Memory Constrained**: All active data must fit entirely in the collective cluster RAM.
* **Single Datacenter**: Geographically distributed networks cannot support low-latency one-sided RDMA.
* **Low-Level Interface**: Lacks high-level abstraction (like SQL); must use custom key-value pointers (`oid`).
