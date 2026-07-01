# MIT 6.5840 Lecture 8: The Google File System (GFS) & Colossus

This study guide summarizes the core architectural principles, real-world failures, and subsequent evolution of Google's flagship file systems: **GFS (2003)** and its modern successor, **Colossus**.

---

## 1. GFS Core Architecture (What Worked)

GFS was designed to handle batch big-data processing (e.g., MapReduce, Web Crawlers) across thousands of commodity Linux machines. It introduced several key ideas to achieve parallel throughput, scalability, and fault tolerance.

### Key Components
* **Single Master (Coordinator):** Manages all metadata (file namespace, chunk mappings, access control, garbage collection). To ensure high performance, the Master stores all metadata in its volatile **RAM**.
* **Chunkservers:** Store the actual data. Files are split into fixed-size **64 MB chunks**, each identified by a globally unique 64-bit chunk handle. Every chunk is replicated across **3 chunkservers** by default.
* **Clients:** Library code linked into applications. Clients query the Master only for metadata (i.e., "Which chunkservers hold chunk X?") and then talk directly to Chunkservers for data transmission.

### Crucial Concepts

#### **A. Leases（租约） to Prevent Split-Brain**
* **The Problem:** If the Master needs to select a new replica to be the primary（主备份） but the old primary is still alive (due to transient network partition), having two active primaries would lead to diverging states (split-brain).
* **The Solution:** The Master grants a **Lease** (60 seconds) to one replica, designating it as the **Primary**. 
  * The Master promises not to assign another Primary until the lease expires.
  * The Primary alone dictates（规定） the serialization order for concurrent writes on a chunk and propagates those operations to the secondaries.

#### **B. Replication Protocol (Primary-Backup)**
1. Client asks the Master which chunkserver holds the current lease (Primary) and the locations of secondaries（备份节点）.
2. Client pushes write data to all replicas. Replicas buffer the data in a cache (data flow is decoupled from control flow to maximize bandwidth).
3. Once all replicas acknowledge receiving the data, Client sends a write request to the Primary.
4. Primary assigns a sequential number to the write and applies it to its local storage.
5. Primary forwards the write request with the sequence number to all Secondaries, ensuring they apply writes in the exact same order.
6. Primary replies to the client once all Secondaries have successfully acknowledged completion.

---

## 2. GFS Failures at Scale (Why it Broke)

Despite its initial success, as Google's data footprint grew 1,000x over the subsequent decade, GFS ran into severe architectural limitations:

* **Metadata Memory Bottleneck:** Since the Master stored all metadata in RAM（Random Access Memory，中文通常称为“随机存取存储器”，也就是我们俗称的“内存”）, the Master ran out of memory as the total number of files and chunks multiplied. Master startup/reboot times (which required scanning all chunkservers to rebuild chunk locations) grew to tens of minutes.
* **The "Small Files" Problem:** GFS's 64 MB chunk size was optimized for massive files. When applications began storing billions of small files (e.g., Gmail, search indices), each small file still consumed a chunk and Master metadata entry, wasting disk space and exhausting Master RAM.
* **Master CPU Exhaustion:** A single Master CPU had to handle polling, load balancing, garbage collection, and metadata requests for thousands of clients, causing severe latency spikes（尖峰）.
* **Lack of Automatic Failover:** Early GFS Master failover(故障转移) was manual, requiring human intervention and causing significant system downtimes during Master crashes.

---

## 3. The Colossus Revolution

To address the limitations of GFS, Google developed **Colossus**, its next-generation distributed file system.

### Key Upgrades

#### **A. Distributed Metadata**
* Instead of a single master keeping metadata in RAM, Colossus shards metadata horizontally and stores it in **Bigtable** (a distributed NoSQL database) which in turn runs on Colossus itself.
* This completely eliminated the Master RAM/CPU bottleneck, allowing metadata to scale independently.

#### **B. Erasure Coding (Reed-Solomon) vs. 3x Replication**
* **GFS 3x Replication:** Had a **200% storage overhead** (3 copies of everything).
* **Colossus Erasure（擦除）Coding:** Breaks files into smaller blocks (e.g., 1MB or 8MB) and uses Reed-Solomon algorithms (such as $8+3$ or $9+6$) to generate parity fragments.
为什么 GFS 一开始不用，后来 Colossus 却用了？（技术折中 Trade-off）
既然纠删码这么省钱，为什么早期 GFS 要用笨重的 3 副本呢？
因为：
纠删码需要消耗 CPU 进行数学计算。在 2003 年（GFS 诞生时），服务器的 CPU 算力非常珍贵，经不起高频的纠删码编解码计算。
数据恢复延迟高：3 副本坏了一个，直接去读另外两个就行，速度极快；而纠删码一旦坏了机器，必须通过网络把剩余 8 台机器的数据全读出来，在内存中进行矩阵乘法运算，才能还原出丢失的数据，网络和计算开销极大。
到了 2010 年以后（Colossus 时代），随着 CPU 算力暴涨，以及网络带宽从 1Gbps 演进到 10Gbps/100Gbps，计算和网络不再是瓶颈，存储成本变成了最大痛点，因此纠删码迅速取代了 3 副本，成为了现代工业界大厂的绝对标配。
  * Under $8+3$, data is split into 8 data blocks and 3 parity blocks. The system can tolerate any 3 simultaneous disk failures.
  * The storage overhead drops from **200% to only 37.5%**, saving Google billions of dollars in hardware costs while providing *higher* data durability.

#### **C. Client-Driven Real-time Recovery**
* When reading data, the client directly contacts chunkservers.
* If a chunkserver is slow or dead, the client does not block. Instead, it reads the other fragments and dynamically reconstructs the missing data on the client side in real-time, achieving sub-millisecond tail latencies.

#### **D. Fast, Parallel Re-replication**
* Because GFS chunks were massive (64MB), copying a lost chunkserver's data took hours.
* Colossus's smaller block sizes and highly distributed fragment layout allow lost blocks to be reconstructed in parallel across the entire cluster, restoring data durability in **minutes** instead of hours.

---

## 4. GFS vs. Colossus Comparison

| Feature | GFS (2003) | Colossus (Modern) |
| :--- | :--- | :--- |
| **Metadata Management** | Single Master RAM (Bottleneck) | Distributed & Sharded (via Bigtable) |
| **Fault Tolerance** | 3x Replication (200% overhead) | Erasure Coding (typically ~37.5% overhead) |
| **Standard Block Size** | 64 Megabytes | 1 Megabyte / 8 Megabytes |
| **Recovery Speed** | Slow (hours, limited by disk write speeds) | Instantaneous (minutes, parallelized over cluster) |
| **Target Workload** | Batch Big-Data (MapReduce) | Low-latency, Interactive (Gmail, YouTube, Search) |
| **Consistency** | Relaxed (anomalies / duplicates possible) | Strong, highly consistent |
