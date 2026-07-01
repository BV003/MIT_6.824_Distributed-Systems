# MIT 6.5840 Lecture 9: ZooKeeper Case Study

ZooKeeper (2010) is a highly reliable, centralized coordination service for distributed applications. It provides a simple, file-system-like API to solve complex distributed problems (leader election, configuration management, locking) without requiring application developers to write complex Raft/Paxos-like consensus algorithms.

---

## 1. ZK Data Model & API

ZooKeeper exposes a hierarchical(层级) namespace reminiscent of a traditional file system. Each node is called a **znode**.

### Znode Types
* **Regular (Persistent):** Remains until explicitly deleted.
* **Ephemeral:**（短暂的） Deleted automatically by ZooKeeper when the client session that created them terminates (extremely useful for failure detection).
* **Sequential（顺序节点）:** ZooKeeper automatically appends a monotonically increasing counter/sequence number to the znode's name (useful for ordering).

### Key APIs
* `create(path, data, flags)` (exclusive: fails if path already exists)
* `exists(path, watch)` (registers a callback if path changes/is deleted)
* `getData(path, watch) -> data, version`
* `setData(path, data, version)` (updates only if `version` matches, preventing stale writes)
* `getChildren(path, watch)`

---

## 2. Solving Coordination Challenges

ZooKeeper allows distributed clusters to manage coordination securely and gracefully:

### A. Leader (Coordinator) Election
To elect exactly one leader (e.g., MapReduce Coordinator):
1. Potential leaders concurrently call `create("/mr/leader", data, ephemeral=true)`.
2. Only **one** client succeeds (exclusive creation).
3. The successful client becomes the **Leader**.
4. Other clients call `exists("/mr/leader", watch=true)` to wait without polling.
5. If the leader crashes, its session times out, ZooKeeper deletes the ephemeral znode, and the watch fires（监听被触发）, prompting secondaries to race for leadership.

### B. "Fencing" and Preventing Split-Brain
* **The Threat:** A leader is alive but temporarily disconnected/slow. ZooKeeper declares it dead, deletes its ephemeral node, and elects a new leader. If the old leader tries to write to storage, both leaders might modify state concurrently (split-brain).
* **The Solution (Fencing):** When ZooKeeper terminates a session, it **atomically rejects all further writes** from that session. If the old leader tries to execute `setData` or any write, ZooKeeper raises an exception, forcing the old leader to realize it is no longer legitimate and must step down.

---

## 3. High Performance Design

To achieve high throughput (tens of thousands of operations per second), ZooKeeper optimizes aggressively for **read-heavy** workloads:

```
                  [ Clients ]
                   |   |   |
                   V   V   V
               [ Follower Servers ]  <--- Serves READS locally (Fast!)
                   |
            (Writes Forwarded)
                   |
                   V
               [ Leader Server ]     <--- Coordinates WRITES (Consensus)
```

1. **Locally Served Reads:** Clients are distributed across multiple ZK Followers. Followers answer `getData` and `exists` locally from their memory, without involving the Leader.
2. **Watches over Polling:** Instead of clients constantly polling for updates (which exhausts server bandwidth), clients register a `Watch`. Followers keep track of watches and send a single notification to the client when a state change occurs.
3. **Asynchronous Batched Writes:** Clients can pipeline many operations concurrently without blocking. ZooKeeper batches these writes on the leader, writing them to disk sequentially to bypass rotational disk latencies.

---

## 4. Consistency Guarantees & Trade-Offs

Because reads are served locally from potentially lagging followers, **reads are not strictly linearizable** (a client might read a slightly stale value). However, ZooKeeper provides powerful alternative guarantees:

* **Linearizable Writes (A-Linearizability):** All state updates are coordinated by the leader and applied in a single, globally agreed-upon order (using `ZXID` transaction numbers).
* **FIFO Client Order:** All requests from a single client are executed in the exact order they were sent.
* **Monotonic Reads:** A client's reads will only move forward in time. If a client connects to a new follower, that follower will delay serving the read until its state catches up to the highest `ZXID` the client has already seen.

---

## 5. ZooKeeper Comparison & Limitations

* **Memory Constrained:** All ZK data must fit in the servers' RAM to ensure reads remain fast. It is meant for metadata and coordination, **not** general object storage.
* **No Sharding:** The hierarchical znode tree cannot be easily partitioned across servers, so ZK cannot scale writes horizontally.
* **Modern Alternatives:** Tools like `etcd` (used by Kubernetes) and `Consul` provide similar coordination features using Raft, often with stronger consistency guarantees (e.g., linearizable reads via leader leases).
