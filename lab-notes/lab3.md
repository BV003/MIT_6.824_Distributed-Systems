# Raft

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