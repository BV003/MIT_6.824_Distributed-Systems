# Lab4

## part A

you will implement a replicated-state machine package, rsm, using your raft implementation.

复制状态机（RSM）对其复制的请求内容是完全不知情的（不可知的）

你可以把 Goroutine（Go 协程） 理解为：“轻量级的、由 Go 语言自己管理的微型线程”。
如果说传统的操作系统线程（Thread）是一辆重型卡车，那么 Goroutine（协程） 就是一辆轻便的摩托车。

Only need to implement rsm code.

### RSM Class & Architecture Diagram

Below is the design of the classes and components inside the RSM layer:

```text
 ┌────────────────────────────────────────────────────────────────────────┐
 │                              StateMachine                              │
 │                              (Interface)                               │
 ├────────────────────────────────────────────────────────────────────────┤
 │ + DoOp(req: any) : any                                                 │
 │ + Snapshot() : []byte                                                  │
 │ + Restore(data: []byte)                                                │
 └───────────────────────────────────▲────────────────────────────────────┘
                                     │
                                     │ implements (in Part B)
                                     │
 ┌───────────────────────────────────┴────────────────────────────────────┐
 │                               KVServer                                 │
 │                              (Part B)                                  │
 └────────────────────────────────────────────────────────────────────────┘
                                     ▲
                                     │ references
                                     │
 ┌───────────────────────────────────┴────────────────────────────────────┐
 │                                  RSM                                   │
 ├────────────────────────────────────────────────────────────────────────┤
 │ - mu : sync.Mutex                                                      │
 │ - me : int                                                             │
 │ - rf : raftapi.Raft                                                    │
 │ - applyCh : chan raftapi.ApplyMsg                                      │
 │ - maxraftstate : int                                                   │
 │ - sm : StateMachine                                                    │
 │ - pendingOps : map[int]chan Notification  <-- Key: Log Index           │
 ├────────────────────────────────────────────────────────────────────────┤
 │ + MakeRSM(servers, me, persister, maxraftstate, sm) : *RSM             │
 │ + Submit(req: any) : (rpc.Err, any)                                    │
 │ - applyReader()                           <-- Background Goroutine     │
 └──────────────────┬──────────────────────────────────▲──────────────────┘
                    │                                  │
                    │ calls rf.Start(Op)               │ delivers committed logs
                    ▼                                  │ via applyCh
 ┌─────────────────────────────────────────────────────┴──────────────────┐
 │                                 Raft                                   │
 └────────────────────────────────────────────────────────────────────────┘


                        Helper Types Used by RSM
                        
        ┌────────────────────────┐         ┌────────────────────────┐
        │           Op           │         │      Notification      │
        ├────────────────────────┤         ├────────────────────────┤
        │ + Id : int64           │         │ + id : int64           │
        │ + Command : any        │         │ + term : int           │
        └────────────────────────┘         │ + result : any         │
                                           └────────────────────────┘
```

### Core Execution Flow

1. **Client Request**:
   When the KV Server receives a request, it calls `rsm.Submit(args)`.

2. **Wait for Consensus**:
   - `Submit` wraps the request into an `Op` with a unique random `Id`.
   - It calls `rsm.rf.Start(op)` to replicate via Raft.
   - It registers a channel in `pendingOps[index]` and blocks waiting.

3. **Background Reader**:
   - The background goroutine `applyReader` loops on `rsm.applyCh`.
   - When Raft commits the entry, `applyReader` receives it, executes it locally via `StateMachine.DoOp()`, and wakes up the blocked `Submit` goroutine using the channel stored in `pendingOps[index]`.


## part B

In part B, you will implement a replicated key/value service using rsm, but without using snapshots.

## part C

In part C, you will use your snapshot implementation from Lab 3D, which will allow Raft to discard old log entries.