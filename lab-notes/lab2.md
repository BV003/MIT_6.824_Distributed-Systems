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

