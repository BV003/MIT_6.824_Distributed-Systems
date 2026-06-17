Here is the most important information you need to know about Primary/Backup Replication and the VMware FT (Fault Tolerance) paper, explained in simple words:

1. Replicated State Machine (RSM)
- The Core Idea: If two computers start in the same initial state, receive the same inputs, in the exact same order, and execute deterministically, they will end up in the exact same state.
- Why VMware FT is special: Instead of replicating a single application (like GFS or a database), VMware FT replicates the entire virtual machine (the Operating System, CPU registers, RAM, etc.). It can make any existing software fault-tolerant without changing a single line of code!

2. Divergence (The Enemy)
Most of the time, the Primary and Backup VMs run identical CPU instructions (like ADD), so no data needs to be sent over the network. However, some events are non-deterministic and will cause them to drift apart (diverge). VMware FT must intercept and forward these events:
1. Interrupts: A timer interrupt must occur at the exact same instruction number on both machines.
2. External Input: Network packets and disk reads (via DMA) must be injected into the Backup's memory at the exact same execution point.
3. Non-deterministic Instructions: Reading the CPU's current time (RDTSC instruction) or serial number. The Primary executes it, intercepts the result, and sends it to the Backup to use.
3. The Logging Channel and "Lag"
- The Primary VM records all non-deterministic events and inputs, and sends them to the Backup over a network link called the Logging Channel.
- The Rule: The Backup VM must always lag behind the Primary by at least one log entry. It cannot execute instruction $X$ until it receives the log entry telling it how to handle the event at instruction $X$.

4. The Output Rule (Crucial for Correctness)
If the Primary crashes, the Backup will take over. We must ensure the Backup's state is consistent with whatever the outside world (clients) saw before the crash.
- The Rule: The Primary cannot send any output (network packet or disk write) to the outside world until the Backup has received and acknowledged the log entry corresponding to the state that generated that output.
- Why? If the Primary sends a response "Account balance = \$11" to a client and then immediately crashes before the Backup learns about the deposit, the Backup would go live with an outdated state ("Account balance = \$10"). The client would see an impossible state transition.
- Performance Cost: The Primary must constantly pause and wait for Backup network confirmations before replying to clients.

5. Split-Brain and the Shared Disk (Tie-Breaker)
If the network link between the Primary and Backup breaks, how do they know if the other machine crashed, or if they are just partitioned?
- The Risk: If both went "live," they would both try to serve clients, leading to a Split-Brain scenario where data gets corrupted.
- The Solution: VMware FT uses a Shared Disk as a tie-breaker. 
- Whichever machine loses connection first tries to perform an atomic "test-and-set" lock operation on the shared disk.
- The winner of this lock goes live and becomes the sole active server.
- The loser halts/shuts itself down.

6. Machine-Level vs. Application-Level Replication
- VMware FT (Machine-Level): Very cool and transparent (works for any app), but has high overhead and is restricted to single-core CPUs because multi-core race conditions are impossible to synchronize efficiently at the machine level.
- Application-Level (What we do in Labs 2, 3, and 4): We replicate application commands (e.g., "Put(Key, Value)") rather than raw RAM bytes. This is much faster, uses less network bandwidth, and scales to multi-core servers, but requires the developer to write the replication logic themselves.