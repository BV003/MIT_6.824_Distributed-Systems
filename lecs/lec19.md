# Lecture 19: Ray (Distributed Futures & Ownership)

## 1. What is Ray?
Ray is a highly popular, open-source distributed execution framework (used heavily by OpenAI and Anyscale) designed for dynamic, stateful AI and machine learning workloads. It blends the simplicity of **actors** and **futures** to achieve microsecond-level scheduling of fine-grained(细粒度), dynamic task graphs.

---

## 2. Why Ray over MapReduce or Spark?
* **MapReduce / Spark**: Designed for large, stateless batch-processing pipelines. They cannot handle low-latency, fine-grained tasks (taking only a few milliseconds) or stateful（有状态的） real-time（实时） servers (such as routing and model serving).
* **Ray's Solution**: Integrates:
  * **Actors**: Stateful worker instances that maintain local variables across multiple task invocations（调用）.
  * **First-Class Futures**: Handles representing the output of asynchronous computations, which can be passed freely as arguments to other tasks before the actual values are even calculated.

---

## 3. The Core Concept: Ownership
To manage millions of fine-grained futures efficiently, a centralized database or general sharded directory is too slow and introduces scalability bottlenecks. Ray solves this via **Ownership Sharding**:

* **Owner**: The worker that submits/calls a task is the absolute "owner" of the returned future (object reference).
* **Location**: The actual *value* of the object is stored in the local object store of the worker that executed the task.
* **Borrower**: Other workers that receive the future as an argument are "borrowers". They register themselves with the owner.

### Advantages:
* **Decentralization**: The owner manages the reference counts and garbage collection for its owned objects locally, without consulting a central coordinator.
* **Low Latency**: Worker `A` can invoke task `C` passing a future from `B` without any immediate network hops to `B` to fetch the data.

---

## 4. Failure Recovery & Lineage Reconstruction（重建）
To maintain high performance without writing everything to persistent storage, Ray utilizes **Lineage Reconstruction**:

* **Re-Execution**: If a worker node crashes and its local object store loses the data for an active future, the *owner* of that future detects the loss and re-submits the exact task that created it.
* **Fate（命运） Sharing**: If an **owner** crashes, any borrowers waiting on that owner's futures would be left with unresolved, dangling references. Instead of trying to recover from this, borrowers "share fate" with their owners: they terminate themselves (acting as a crash), letting the grandparent caller reconstruct the parent node and the sub-tree from scratch.

---

## 5. Homework Question & Analysis

### Scenario:
```python
def C(x):
  z = D(x)
  return get(z)  # Blocks and returns the actual value of future z
```
*Suppose the worker running task `D` crashes before finishing.*

### Questions & Answers:
1. **Who owns future `z`?**
   * **Answer**: Worker `C` (the worker running task `C`). Since `C` is the caller that invoked `D(x)` and received the future `z`, `C` is the designated owner of `z`.
   
2. **Which worker initiates the re-execution of `D`?**
   * **Answer**: Worker `C`. Since `C` owns `z`, it is responsible for monitoring `D`'s completion. When it detects that the worker executing `D` has failed, `C` initiates the re-execution of `D()` to resolve the future `z` and retrieve its value.
