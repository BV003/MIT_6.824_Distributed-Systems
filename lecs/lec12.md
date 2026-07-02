# Spanner

1. Spanner's Core Architecture: 2PC over Paxos
- The Structure: Data in Spanner is sharded into segments called directories (or tablets), and each shard is replicated across multiple datacenters using a Paxos consensus group.
- The Consensus Marriage: When a transaction spans multiple shards, Spanner uses Two-Phase Commit (2PC) to coordinate across shards. 
- Solving the 2PC Blocking Flaw: Ordinarily, if a coordinator in 2PC crashes, participants are locked and blocked. Spanner solves this by making the Coordinator and each Participant a replicated Paxos group. If a physical coordinator fails, Paxos instantly elects a new leader to resume the 2PC protocol without holding blocks indefinitely.

2. Multi-Version Concurrency Control (MVCC) & Snapshot Isolation
(From: Using MVCC & Cloud Spanner Replication Docs)
- No-Lock Reads: Since 99.9% of database transactions are read-only, Spanner uses Multi-Version Concurrency Control (MVCC) to serve reads without taking any locks.
- When a write occurs, Spanner does not overwrite the record; it writes a new version tagged with a specific Timestamp ($TS$).
- A read-only transaction is assigned a single read timestamp ($T_{read}$). It can then read the values of any record at exactly that timestamp from its local replica, avoiding slow cross-datacenter roundtrips entirely.

3. The TrueTime API & External Consistency
(From: TrueTime and Facebook NTP Service)
- The Problem: If clocks across datacenters are not perfectly synchronized, a read-only transaction might get a timestamp that is slightly in the past, missing a write that completed in real-world time (violating External Consistency / Linearizability).
- The Solution (TrueTime): Google uses custom GPS receivers and atomic clocks in every datacenter. Instead of returning a single time, the TrueTime API returns a time interval:
$$\text{TT.now()} = [earliest, latest]$$
where the true physical time is guaranteed to be within the window. The maximum error is bounded by $\epsilon$ (typically $< 1\text{ms}$).
- The Commit Wait Rule (The Magic):
- When a read-write transaction $T_1$ commits, the leader assigns it a timestamp $S = \text{TT.now().latest}$.
- The leader must wait to release its locks and respond to the client until:
$$\text{TT.now().earliest} > S$$
- This guarantees that the transaction's timestamp $S$ is absolutely in the past before any subsequent transaction can start. Thus, any later transaction $T_2$ is guaranteed to get a timestamp $S_2 > S_1$, maintaining perfect linearizability across the globe.

4. Living Without Atomic Clocks (The CockroachDB Approach)
(From: CockroachDB: Living Without Atomic Clocks)
- Google's Hardware Advantage: TrueTime requires specialized atomic clocks in proprietary datacenters. Standard cloud users (AWS, Azure) cannot deploy atomic clocks.
- CockroachDB's Software Hybrid Logical Clocks (HLC):
- CockroachDB achieves external consistency on commodity hardware using Hybrid Logical Clocks (HLC).
- HLC combines physical time (synchronized via NTP, which has a higher $\epsilon \approx 100\text{ms}-250\text{ms}$) with logical Lamport clocks.
- If a transaction encounters a record with a timestamp within the "uncertainty window" (the clock offset limit $\epsilon$), it performs a restart (advancing its logical clock past the conflicting timestamp) to ensure causality and serializability without requiring hardware-synchronized clocks.

5. Transitioning from NoSQL to a SQL System
(From: Spanner: Becoming a SQL System, 2017)
- Spanner originally started as a NoSQL key-value store (similar to Bigtable but with transactions). 
- However, Google engineers realized that application developers spent massive amounts of time writing custom query and join logic.
- Google migrated Spanner to support a full SQL query engine with distributed query execution, query compilation, and schema changes. This proved that you can have both NoSQL-scale horizontal sharding and SQL-grade transactional consistency in a single system.