# Raft

1. Log Divergence & Quick Rollback (Lab 2B)
- The Problem: When leaders crash and reboot in rapid succession, different servers can end up with different, conflicting logs at the same index.
- The Rule: The current Leader's log is always right. Raft forces followers to throw away any parts of their log that conflict with the leader's log and copy the leader's entries.
- Fast Backup (Crucial for Lab): 
- Standard Raft backs up nextIndex by 1 entry at a time, which is extremely slow if a follower is far behind.
- To pass the lab tests, you must implement Fast Backup (using XTerm, XIndex, and XLen in the rejection reply). This allows the leader to back up past entire terms of mismatched logs in a single RPC round.

在 Raft 协议中，每个节点的日志（Log）都是一个数组，数组里的每一个元素叫做日志条目（Log Entry）。每个日志条目都包含两个最核心的整数属性：Index 和 Term。（当然还包含日志内容）

什么是 Index（索引）？
定义：日志条目在数组中的位置下标（从 1 开始递增）。
作用：用来确定日志的先后顺序。
例如：
- 数组第 1 个位置的日志，Index = 1。

什么是 Term（任期）？
定义：该日志条目被创建时，系统当前所处的选举周期号（一个递增的整数）。
作用：用来标识该日志是在哪一届领导者（Leader）在位时写入的。
过程：
1. 系统刚启动时，Term = 1。
2. 此时 Leader 收到客户端请求，生成一条日志，这条日志的 Term 就是 1。
3. 如果 Leader 挂了，系统重新选举，Term 就会变成 2。
4. 新 Leader 写入的日志，Term 就是 2。

Raft 协议有一个核心定理：
如果两个不同节点上的日志，在相同的 Index 处拥有相同的 Term，那么它们在这条日志以及之前的所有日志内容，都保证是完全相同的。


index
term

2. The Election Restriction (Safety)
- The Question: Why can't we just elect the server with the longest log?
- The Answer: Because a server with a shorter log might actually contain a committed entry, whereas a server with a longer log might just have uncommitted, stale entries from a crashed leader.
- How Raft guarantees safety: A server will only vote for a candidate if that candidate is "at least as up-to-date" as itself:
1. The candidate's last log entry has a higher term, OR
2. The last log entry has the same term, but the candidate's log is same length or longer.


3. Persistence: What to Save across Crashes (Lab 2C)
If a server crashes and reboots, it forgets everything in its RAM. However, to safely rejoin the cluster, it must have saved exactly three things to non-volatile storage (disk/SSD):
1. currentTerm: To prevent voting in or accepting messages from an old, expired election.
2. votedFor: To prevent voting for two different candidates in the same term (e.g., voting for $A$, crashing, rebooting, and then voting for $B$).
3. log[]: To ensure that any entry that was part of a committed majority is never lost.
Other variables like commitIndex and lastApplied do not need to be saved; they can be safely rebuilt by replaying the log after rebooting.


4. Log Compaction & Snapshots (Lab 2D)
- The Problem: Over time, the log gets too big, consuming too much memory and taking hours to replay on a reboot.
- The Solution: Periodically, the Key-Value database takes a "snapshot" of its current data (e.g., just storing x=5, instead of storing the history of 10,000 writes that led to x=5). 
- Raft saves this snapshot to disk and discards all log entries up to that point.
- InstallSnapshot RPC: If a follower has been offline for a long time, the leader might have already discarded the log entries the follower needs. The leader must send an InstallSnapshot RPC to give the follower the entire snapshot directly.（传递的不是历史日志，而是数据快照）


5. Why Get() (Read-Only) is Hard
- The Surprise: Even a read-only request like Get(key) cannot be answered immediately by the leader from its local memory.
- Why? Because a network partition（分割） could have occurred. The leader might have been disconnected and replaced by a new leader, but doesn't know it yet. If it answers immediately, it might serve old, stale data (violating Linearizability).
- The Solution: Standard Raft forces Get() requests to go through the entire log-commitment process just like Put() to prove that the leader still has the majority's support before replying.