# Consistency and Linearizability

1. What is Linearizability? (The Standard of "Correctness")
Linearizability is the gold standard for "Strong Consistency." It is a specification for how a concurrent system must behave from the client's perspective. 
A key/value store is linearizable if:
Every operation appears to execute instantaneously at a single point in time (the linearization point) between its start and finish.
The values returned by all operations are consistent with a single, serial execution in that order.
2. The Three Rules of Thumb for Checking Linearizability
To prove a history of concurrent client requests is linearizable, you must be able to assign a single point in time to each request such that:
No Stale Reads: Once a write operation completes, any get/read that starts after that completion must return the new value (or an even newer one).
Single Order of Writes: All clients must agree on the sequence of state changes. If Client A writes x=1 and Client B writes x=2, we can linearized them in either order, but Client C and Client D cannot see them in opposite orders.
Non-decreasing Time: Real-time ordering must be respected. If Operation 1 completely finishes (returns a reply) before Operation 2 starts, Operation 1's linearization point must come before Operation 2's.
3. What Linearizability Rules Out (And why it's hard to build)
Linearizability strictly outlaws common distributed systems optimizations and bugs:
Lagging Replicas: If a client reads from a backup server that hasn't received the latest primary update yet, the client gets stale data. This is non-linearizable (seen in GFS).
Split-Brain: If two partition nodes think they are the leader and accept writes, clients will see conflicting data.
Duplicate Execution (Retransmissions): If a client's RPC request times out, they will re-send it. If the server does not filter out duplicates, a non-idempotent operation like Append will run twice. Suppression of duplicates is mandatory for linearizability (used in Lab 2/3/4).
4. Linearizability vs. Serializability (Crucial distinction!)
People often confuse these two, but Jepsen clarifies them perfectly:
Linearizability (Single-Operation, Real-Time): Focuses on single-object operations (e.g., Get, Put) and real-time ordering. It ensures reads are fresh.
Serializability (Multi-Operation Transactions): Focuses on transactions (groups of operations on multiple objects). It ensures that transactions execute as if they ran in some serial order, but doesn't guarantee real-time freshness (it doesn't care about when requests start/finish).
Strict Serializability: The combination of both. It means transactions execute in a serial order, and that order respects real-time constraints.
5. CAP Theorem: The Ultimate Trade-off
You can never have both Strong Consistency (Linearizability) and 100% Availability in the presence of network partitions.
If a network partition occurs and isolates your replicas:
To maintain Linearizability: Replicas that cannot talk to the majority must refuse client writes/reads. You sacrifice availability.
To maintain Availability: Replicas continue to serve local client requests. Replicas diverge, and clients see stale/conflicting data. You sacrifice linearizability (Eventual Consistency).