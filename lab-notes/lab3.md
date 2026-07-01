# Raft

实现多服务器之间的分布式协议，对外暴露为一个服务器的抽象

A replicated service achieves fault tolerance by storing complete copies of its state (i.e., data) on multiple replica servers.

Raft organizes client requests into a sequence, called the log, and ensures that all the replica servers see the same log.

每个服务器（Server）都有自己独立的、完全本地的数据库服务

在一个 $N$ 台服务器组成的集群中，每一台服务器上都同时运行着两个组件，它们是一对一绑定的：
1. 一个 Raft 模块（负责同步和决定日志顺序）
2. 一个本地数据库服务（上层服务，负责真正存储数据）

## leader election

Roles: A peer can be a Follower, Candidate, or Leader. We can represent this with an int or string (e.g., const (Follower = 0; Candidate = 1; Leader = 2)).

Persistent State on All Servers:
- currentTerm int: Latest term server has seen (initialized to 0, increases monotonically).
- votedFor int: Candidate ID that this server voted for in the current term (or -1 if none).
- log []LogEntry: Log entries; each entry contains a command and the term when the entry was received by the leader. For 3A, even if logs are empty, we need the array initialized.

Volatile State on All Servers:(保存在内存中的，可以丢失的)
- commitIndex int: Index of the highest log entry known to be committed (initialized to 0).
- lastApplied int: Index of the highest log entry applied to the state machine (initialized to 0).

Election & Heartbeat Timers:
- lastHeartbeat time.Time: The time when we last heard from the current leader (or voted for a candidate). This is crucial to decide when we have timed out.

GetState() 的作用是：获取当前 Raft 节点的最新状态（包括它所处的 Term，以及它是否认为自己是 Leader）

RequestVoteArgs (请求结构体)
这是候选节点（Candidate）向其他节点（Voter）发送投票请求时携带的数据参数

RequestVoteReply (响应结构体)
这是接收节点对投票请求做出的回复数据：

- true：表示接收节点同意向该候选节点投票。
- false：表示接收节点拒绝投票。

一致性检查规则 (Election Restriction)
为了保证新选出的领导者（Leader）一定包含所有已提交的日志，接收节点在决定是否将 VoteGranted 设为 true 时，必须使用 LastLogTerm 和 LastLogIndex 满足以下条件（即候选节点的日志至少与接收节点一样新）：
如果候选节点的 LastLogTerm 大于接收节点的最新日志任期，或者：
如果两者 LastLogTerm 相同，且候选节点的 LastLogIndex 大于或等于接收节点的最新日志索引。
只有满足上述两条件之一，且接收节点在当前 Term 内还没有投过票时，接收节点才会同意投票（VoteGranted = true）。

这两个结构体定义了 Raft 协议中 AppendEntries（附加日志条目）RPC 的请求和响应数据格式。
这个 RPC 有两个核心作用：
1. 心跳机制（Heartbeat）：领导者（Leader）定期向跟随者（Follower）发送空请求，以维持其领导地位，防止跟随者发起新的选举。
2. 日志复制（Log Replication）：领导者将新的日志条目同步到跟随者的本地日志中。

ticker 是一个在每个 Raft 节点后台运行的、无限循环的 Go 协程（Goroutine）。它的核心功能是：周期性检测是否需要发起领导者选举。

## log

client will only get access with leader.


func (rf *Raft) Start(command interface{}) (int, int, bool) 
当客户端发送一个写请求（例如 Set x = 5）到服务器时，上层服务无法直接将其写入本地数据库，必须先通过调用 rf.Start("Set x = 5") 将这个指令交由 Raft 模块进行多节点间的共识同步。
index：该指令在日志数组中的预期索引位置（如果未来被成功提交的话）。
term：当前的任期号。
isLeader：返回 true，代表自己是 Leader。

Implement the Applier Loop (applyCh)
applyCh（全称 Apply Channel）是 Go 语言中的一个通道（Channel），其本质上是一个线程安全的先进先出（FIFO）数据管道。
在 MIT 6.5840 实验中，它是底层 Raft 模块与上层应用服务（如键值对数据库）之间进行通信的唯一桥梁。

第一阶段：Raft 模块在后台默默干活（同步到大多数节点）
这个阶段完全发生在 Raft 协议层，数据库（状态机）此时根本不知情，也没有执行任何写操作。
1. Leader 的 Raft 模块通过网络向各个 Follower 的 Raft 模块发送 AppendEntries RPC。
2. 此时，各台机器的数据库里都还没有 x = 5 这个数据。
3. 当大多数节点的 Raft 模块回复“已成功将该日志写入本地日志数组”后，Leader 的 Raft 模块判定：这条日志安全了（已提交 / Committed）。
第二阶段：服务器的数据库真正执行 x = 5（Apply 到状态机）
这个阶段是在第一阶段成功判定安全之后，才触发的后续操作。
1. Leader 的 Raft 模块判定安全（已提交）后，会把这个消息放入 applyCh 管道。
2. 服务器的数据库服务从 applyCh 管道中取出这个消息。
3. 数据库将 x = 5 写入自己的内存中。直到此时，数据才真正写入了数据库，客户端才能读到它。

To prevent blocking the core locks of your Raft peers, committed entries are applied to the database asynchronously through a dedicated thread started in Make()

领导者（Leader）在没有新日志写入时，也一定会每隔一段时间自动发送 AppendEntriesArgs。领导者不会为了专门宣告“某条日志已提交”而发明一个新的 RPC。相反，它会顺票搭车（Piggyback），利用现有的 AppendEntries RPC（附加日志/心跳） 来通知跟随者。

Raft 明确划分了领导者选举、日志复制和安全性保障三个子问题，这让工程师在编写代码、排查 Bug 时思路极为清晰。

## Part 3C: persistence
If a Raft-based server reboots it should resume service where it left off. This requires that Raft keep persistent state that survives a reboot.

Raft 论文中明确规定，有且仅有以下 3 个变量 必须在做出任何改变之前，强行刷入磁盘（Must be persisted to stable storage before responding to RPCs）：
1. currentTerm（当前任期号）：
- 为什么：防止节点重启后任期倒退。如果重启后任期变小，它可能会给一个已经过期的候选者投票，或者发起一个本不该发生的低任期选举，导致脑裂。
2. votedFor（在当前任期投给谁了）：
- 为什么：防止一票多投。如果节点在 Term 2 投给了 A，然后崩溃重启，内存数据丢失。重启后它如果又在 Term 2 投给了 B，这就违反了“一个节点在一届任期内最多只能投一票”的铁律，直接导致选出两个 Leader。
3. log[]（所有的日志条目，包含 Index、Term 和 Command）：
- 为什么：这是系统的数据根本。一旦 Leader 确认某条日志已提交，该日志就绝对不能丢失。

## Part 3D: log compaction

implement version can get rid of log. And we use snapshot to replace.