# Key/Value Server

KV Server（Key-Value Server，键值存储服务器），是分布式系统和数据库领域中最基础、最常用的一种非关系型存储服务（NoSQL）。    

We use KV server as the database in Distributed systems.

## Key/value server with reliable network

我们的目标是实现一个分布式的键值存储（Key-Value Storage）系统，支持两个最基本的操作：读取（Get） 和 写入（Put）。

在分布式（网络）环境下，有两大难题：
1. 网络极其不可靠：消息可能延迟、可能丢失、可能重发。
2. 并发冲突：多个客户端可能同时在读写同一个数据。

为了防止“旧的数据覆盖新的数据”（Stale Write），这个系统为每个 Key 都引入了一个版本号（Version）。
- 初始时，Key 不存在（版本为 0）。
- 每次成功的写入，都会把版本号 加 1。
- 写入时，客户端必须指定“我期望当前服务器上的版本是多少”。只有期望版本与服务器实际版本一致时，服务器才允许写入。

You can only change code when you have lock.

we use a version to make sure synchronous.

## Implementing a lock using key/value clerk

To implement a lock, we use lock to Handle concurrency.

We use a lock state to 

The Acquire Protocol (Acquire())
To acquire the lock, a client runs a loop to periodically check and attempt to claim the lock:
Call Get(l) to retrieve the lock's current state (value and version):
- If the key does not exist (rpc.ErrNoKey):
- Attempt to claim the lock by calling Put(l, myID, 0) (version 0 represents a new key).
- If Put returns rpc.OK, we successfully acquired the lock and can return!
- If Put returns rpc.ErrMaybe, our attempt might have succeeded, but the reply was lost. We call Get(l) again. If the value is our myID, we successfully acquired the lock; otherwise, someone else got it.
- If the key exists and is free (val == ""):
- Attempt to claim the lock by calling Put(l, myID, ver) (using the version retrieved from Get).
- If Put returns rpc.OK, we successfully acquired the lock!
- If Put returns rpc.ErrMaybe, we double-check with Get(l). If the value is myID, we acquired the lock.
- If the key is held by someone else (val != "" and val != myID):
- The lock is busy. We sleep for a short duration (e.g., 20ms) and retry.

The Release Protocol (Release())
To release the lock safely and idempotently (handling potential packet drops), a client does the following in a loop:
Call Get(l) to check if we still hold the lock:
- If we hold the lock (val == myID):
- Call Put(l, "", ver) to reset the value to "" and increment the version.
- If Put returns rpc.OK, the lock is successfully released. We can return!
- If Put returns rpc.ErrMaybe or rpc.ErrVersion, our previous release attempt might have actually succeeded (meaning the value has already changed to "" or been claimed by someone else). We loop back to Get(l) to verify.
- If the lock is already released or held by someone else (val != myID):
- We don't hold the lock anymore, meaning the release is complete. We return!

## Key/value server with dropped messages 

## Implementing a lock using key/value clerk and unreliable network 

