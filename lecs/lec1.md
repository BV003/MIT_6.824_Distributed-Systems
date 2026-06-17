1. What is a Distributed System?
- Definition: A group of computers working together to behave like a single system.
- Why do we want this? To process massive amounts of data and keep running even if some computers break.
- Why is it hard? Computers crash, network cables get unplugged, and keeping all computers in sync is extremely difficult.
2. The Great Trade-off
You can never have everything. You must balance three competing goals:
1. Performance: Making things fast.
2. Fault Tolerance: Surviving crashes.
3. Consistency: Making sure every computer has the exact same, correct data.
- The conflict: If you want absolute correctness, computers must talk to each other constantly, which makes the system slow. If you want speed, you might get outdated data.
3. What is MapReduce? (The focus of Lab 1)
MapReduce is a system created by Google to process terabytes of data across thousands of cheap computers without the programmer needing to worry about crashes or network details.
It splits work into two simple phases:
1. Map (Split & Label): Takes raw data and breaks it down into key-value pairs.
- Example (Word Count): Reads the sentence "apple banana apple", and outputs: ("apple", "1"), ("banana", "1"), ("apple", "1").
2. Reduce (Group & Combine): Collects all matching keys and combines their values.
- Example: Takes all "apple" entries and adds them up to output: ("apple", "2"), ("banana", "1").
4. The Coordinator (The Boss)
The Coordinator is a single master node that manages the work:
- It hands out Map tasks to idle workers.
- Once all Maps are finished, it hands out Reduce tasks.
- If a worker crashes: The Coordinator notices, forgets its progress, and assigns its work to a new worker.
5. Why Determinism Matters
For MapReduce to handle crashes safely:
- Rule: If you run a Map or Reduce task twice with the same input, it must produce the exact same output.
- Why? If a worker crashes halfway through and a new worker restarts its task, the output must match perfectly so the rest of the system doesn't get corrupted or confused.

## MapReduce: Simplified Data Processing on Large Clusters

1. The Core Programming Model (The Signatures)
The programmer only has to write two functions:
- Map (k1, v1) -> list(k2, v2)
- Takes an input key/value pair and outputs a list of intermediate key/value pairs.（Shuffle）
- Reduce (k2, list(v2)) -> list(v3)
- Takes an intermediate key and all values associated with that key, merges them, and outputs a smaller set of values (often just one).
2. The Execution Flow (How it actually runs)
When you start a MapReduce job, this sequence occurs:
1. Splitting: The input files are split into $M$ pieces (typically 16MB to 64MB per piece).
2. Starting Up: One master (now called Coordinator) and many worker machines are started.
3. Map Phase: The Coordinator finds idle workers and assigns them Map tasks. 
- The Map worker reads its assigned input split, parses the key/value pairs, and passes them to the user's Map function.
- Local Storage: The intermediate key/value pairs produced by Map are buffered in memory and periodically written to the worker's local disk, partitioned into $R$ regions by a partitioning function (e.g., hash(key) mod R).
4. The Shuffle: The Coordinator tells Reduce workers where the intermediate files are. 
- Reduce workers use RPCs (Remote Procedure Calls) to read their specific partition of intermediate data directly from the Map workers' local disks.
5. Sort/Group: Once a Reduce worker has read all intermediate data, it sorts the data by the intermediate keys so that all occurrences of the same key are grouped together.
6. Reduce Phase: The Reduce worker iterates over the sorted intermediate data. For each unique key, it passes the key and the list of values to the user’s Reduce function. The output is appended to a final output file (one file per Reduce task, stored in GFS).
3. Fault Tolerance (How it handles crashes)
This is the most critical part of the paper and the core of Lab 1.
A. Worker Failures
- Detection: The Coordinator pings every worker periodically. If a worker doesn't respond within a certain time, the Coordinator marks it as failed.
- Map Recovery: Any Map tasks completed by the failed worker must be re-executed from scratch. 
- Why? Because their intermediate outputs were stored on the failed worker's local disk and are now inaccessible.
- Reduce Recovery: Completed Reduce tasks do not need to be re-executed if they finished successfully.
- Why? Because their final outputs are stored on GFS (the Google File System), which is already replicated on other healthy machines.
- In-Progress Tasks: Any Map or Reduce task that was actively running on the failed worker is set back to Idle so another worker can take it over.
B. Coordinator Failures
- If the Coordinator crashes, the entire MapReduce job fails. Since there is only one Coordinator, it is a single point of failure (SPOF). In practice, users just restart the job.
C. Handling Slow Workers ("Stragglers")
- The Problem: Sometimes a machine is healthy but incredibly slow (due to bad disk, CPU throttling, etc.). The whole job has to wait for this last "straggler" to finish.
- The Solution (Backup Tasks): When a MapReduce job is close to completion, the Coordinator schedules backup executions of the remaining in-progress tasks. Whichever copy finishes first (the original or the backup) is used, and the other is killed. This simple trick reduces job times by up to 40%!
4. Crucial Optimizations
- Locality (Data Proximity): Network bandwidth was the main bottleneck in 2004. To save network usage, the Coordinator attempts to schedule Map tasks on workers that physically store the input data on their own local disks.
- Combiner Function: An optional function that does partial merging of data on the Map worker's machine before it is sent over the network (e.g., merging ten ("apple", 1) pairs into one ("apple", 10)), saving massive network bandwidth.