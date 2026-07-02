# Distributed Transactions

分布式事务

1. The Core Consensus Dilemma: 2PC vs. Paxos/Raft
A common point of confusion for students is how Two-Phase Commit (2PC) relates to Consensus Protocols (Raft/Paxos). They solve fundamentally opposite problems:

- Raft/Paxos (High Availability): 
- Goal: Make sure a single logical service stays up even if some servers crash.
- How: All servers do the same thing. You only need a majority to agree and move forward.

- Two-Phase Commit (Atomic Commit): 
- Goal: Make sure a distributed transaction across different shards (doing different tasks, like Debit on Server A and Credit on Server B) executes completely or not at all.
- How: Every server must agree. It requires unanimous agreement ($100\%$ consensus).


2. The Core Criticisms of 2PC (Why Abadi Argues We Must Move On)
Professor Abadi's post details three major flaws that make classic 2PC a performance and availability nightmare for modern distributed databases:
A. The Blocking Problem (The Vulnerability)
- In 2PC, if a Participant votes YES in Phase 1, it enters the Prepared state and must wait for the Coordinator's COMMIT/ABORT decision.
- If the Coordinator crashes at this exact moment, the Participant cannot unilaterally decide what to do. 
- The Result: The Participant is blocked, and it must continue to hold all database locks on those records. No other transaction can access or modify those records until the Coordinator restarts. This destroys system availability.
B. Massive Latency (The Speed Bottleneck)
- 2PC requires multiple round-trips of messages across the network (Client $\rightarrow$ Coordinator $\rightarrow$ Participants $\rightarrow$ Coordinator $\rightarrow$ Participants $\rightarrow$ Client).
- It also requires writing states to non-volatile disks at multiple stages (both Coordinator and Participants) to survive crashes.
- In modern cloud databases, holding exclusive locks on data records over multiple wide-area network round-trips is incredibly slow.
C. Shard Interdependency reduces Availability
- If you shard your database across 100 servers, and a transaction needs to touch 5 of those servers, all 5 must be online for 2PC to succeed.
- The probability of a transaction failing or blocking increases exponentially with the number of shards it touches.

3. The Modern Alternatives (What to Learn)
Professor Abadi highlights that modern databases are abandoning traditional 2PC in favor of smarter, faster protocols:
A. Replication-Layer Consensus (Raft-Based 2PC)
- This is the architecture used by Google Spanner and CockroachDB (which you will learn next week).
- Instead of running 2PC on bare single servers, each participant shard is itself a replicated Raft/Paxos group, and the Coordinator is also a Raft/Paxos group.
- If a physical Coordinator machine crashes, its Raft group instantly elects a new leader to take over, preventing the indefinite "blocking" problem of traditional 2PC.
B. Deterministic Databases (e.g., Calvin / FaunaDB)
- Instead of locking records dynamically and running 2PC, deterministic systems agree on the transaction order beforehand (using a Paxos log).
- Once the order is agreed upon, each shard executes the transactions locally and deterministically. Because the order is pre-determined, there is no need for 2PC or locks during execution, completely bypassing the 2PC bottleneck!
Summary Takeaway for distributed systems:
Traditional 2PC is a "share-nothing" protocol with zero tolerance for failure (requires $100\%$ server uptime). Modern systems mitigate its blocking and latency flaws either by nesting 2PC inside Raft consensus to ensure the coordinator/participants are always highly available, or by using deterministic ordering to eliminate dynamic locking entirely.