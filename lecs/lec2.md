1. Why Go?
MIT uses Go for three primary reasons:
1. Goroutines (Threads): It is incredibly easy to run code in parallel.
2. Garbage Collection (GC): You don't have to manually free memory (no malloc or free bugs).
3. Type Safety: Prevents memory corruption bugs that are a nightmare to debug in concurrent systems.
2. Goroutines and Shared Memory (The Danger)
A Goroutine is just a lightweight thread. You start one simply by putting go in front of a function call (e.g., go worker()).
- The Danger: All goroutines share the same memory. If two goroutines read and write to the same variable (like a map) at the same time, it causes a Race Condition which will crash your program or corrupt your data.
- The Rule: You must protect shared variables using a Mutex (sync.Mutex).
mu.Lock()
// Critical Section: Only one goroutine can be in here at a time
fs.fetched[url] = true 
mu.Unlock()
- Tip: Always run your tests with the Go race detector enabled: go test -race. It catches hidden race bugs instantly.
3. Mutexes vs. Channels
Go offers two ways to coordinate threads:
1. Mutexes (Shared State): Use these when you have a piece of data (like a database or a status map) that many threads need to read and update. (Recommended for Labs 2, 3, and 4).
2. Channels (Communication): Use these when one thread needs to pass a message or notify another thread (e.g., "I finished task X! Here is the output").
4. What is an RPC (Remote Procedure Call)?
RPC is the core communication mechanism of distributed systems. It makes a network request to another computer look like a regular local function call.
Client App ---> Call("Server.Get", args, &reply) ---> Network ---> Server executes Get() ---> Network ---> Reply back to Client
- Marshalling: The RPC library automatically converts Go structures/objects into a stream of bytes to send over the network, and converts them back on the other side.
- Capitalization Rule: Go's RPC library only marshals struct fields that start with a Capital Letter (exported fields). If you use lowercase letters for your RPC arguments/replies, they will arrive empty!
5. The RPC Failure Nightmare
In a single computer, a function call always runs to completion. In a distributed system, a network call can fail in three confusing ways:
1. The request was lost on the way to the server (the server did nothing).
2. The server executed the request but crashed before sending the reply.
3. The server executed the request and sent the reply, but the network lost the reply.
To the client, all three look identical (no response). 
Semantics (How we handle this):
- Best-Effort: Keep retrying until it works. Danger: If a client sends "Deduct \$10 from my bank account" and the reply is lost, retrying might deduct \$10 multiple times!
- At-Most-Once: The server remembers previous requests and filters out duplicates, ensuring a request is executed at most once. (This is what you will implement in Labs 3 and 4).