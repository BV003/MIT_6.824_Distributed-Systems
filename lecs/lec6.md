# Go patterns

1. The Core Golden Rules of Goroutines
If you only learn three things from Go concurrency, they should be these:
- Rule A: Know why and when each goroutine will exit.
- The Trap: Starting a goroutine with go func() { ... }() and forgetting about it. If it gets blocked on a channel send or receive forever, it leaks memory. Over time, your server will run out of memory and crash (a silent killer in distributed systems).
- The Lesson: Never start a goroutine without knowing exactly what event or condition will cause it to finish and return.
- Rule B: Know why and when each communication (channel send/receive) will proceed.
- The Trap: Sending to an unbuffered channel when no one is receiving, or receiving when no one is sending. This causes immediate deadlock.
- The Lesson: Always trace the matching sender/receiver pair in your head. If a channel send is unbuffered, the sender must block until the receiver is ready.
- Rule C: Use the Race Detector (go test -race).
- The Lesson: Go has a built-in race detector. Always run your tests with -race. If it reports any warning (even if the test passes), you have a bug. Never ignore a race detector warning.


2. When to Use Mutexes vs. Goroutines & Channels
Go developers often argue about whether to use Mutexes (sync.Mutex) or Channels for synchronization. Russ Cox gives a very pragmatic answer: Use whatever makes the code the clearest.
- Convert data state into code state:
- Instead of managing complex state variables (e.g., state = 0, state = 1, state = 2), you can often use goroutines and natural code flow (loops, if, function calls) to represent state. It is much easier to read sequential code than a giant switch state block.
- Convert mutexes into goroutines (and vice versa):
- Sometimes a shared map guarded by a sync.Mutex is the cleanest (like in your KVServer).
- Other times, running a single background coordinator go loop() that processes incoming requests on channels is much cleaner and avoids lock contention.
- Hint: Use goroutines, channels, and mutexes together if that is the clearest way to write the code. They are not mutually exclusive!


3. Key Patterns Explained Simply
Pattern #1: Publish/Subscribe (Handling Slow Consumers)

- If a publisher sends events to subscribers, what happens if one subscriber is slow?
- If you block on c <- event, the slow subscriber blocks the entire system (including healthy subscribers).
- Solution: Introduce helper goroutines with an unbounded queue (using a slice as a buffer) so slow subscribers don't slow down event generation. But be careful: unbounded queues can grow forever if a consumer is permanently stuck!

Pattern #2: Work Scheduler (The Buffered Channel Trick)

- Buffered channels as Semaphore/Blocking Queue:
- A buffered channel of size $N$ (e.g., idle := make(chan string, len(servers))) can represent $N$ available resources (like idle servers).
- Reading from it (<-idle) acquires a resource; writing back to it (idle <- srv) releases it. If no resources are available, the caller blocks automatically. This is a very clean way to throttle concurrency.
- The Closure Capture Bug (Crucial Go Gotcha!):
- Look at this common mistake:
for task := 0; task < numTask; task++ {
    go func() {
        call(srv, task) // BUG: task is shared and changes!
    }()
}
- In Go, loop variables are reused. By the time the goroutine starts, task might have already incremented to the end.
- Solution: Pass the variable as an argument, or re-declare it inside the loop:
for task := 0; task < numTask; task++ {
    task := task // Local copy for each iteration
    go func() {
        call(srv, task) // Safe!
    }()
}

Pattern #3: Replicated Service Client (Hedging / Racing Requests)
- When calling multiple servers, you can fire off requests to all of them concurrently and use a channel to return the first successful result:
done := make(chan result, len(servers)) // Buffered to avoid leaking goroutines!
- Why buffer done? If the channel is unbuffered, and we return as soon as the first server responds, the other goroutines will get blocked forever trying to send to done <- result. By buffering it to the number of servers, the slower goroutines can write to the channel and exit cleanly without getting stuck.