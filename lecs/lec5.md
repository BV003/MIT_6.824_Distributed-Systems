## Raft

Raft is a consensus algorithm—a set of rules that helps a group of independent computers agree on a shared sequence of decisions, even if some of those computers crash or messages get lost. 


1. The Core Idea: State Machine Replication
- What it means: We want multiple servers to behave exactly like a single, perfect server.
- How we do it: Every server runs the same list of commands (the Log) in the same exact order. If they start with the same data and run the same commands, they will end up with the same final data.

2. Majority Rule (The Power of 3 or 5)
- In a cluster of 3 servers, a majority is 2. In a cluster of 5, a majority is 3.
- Why it matters: 
- If the network breaks and cuts the servers into two groups, only the group with the majority is allowed to make decisions. 
- The other group (the minority) must stop and wait. This prevents "split-brain" (where two leaders make conflicting decisions).

3. Raft Leader Election (Lab 3A)
- The Leader is Boss: Only one server is the leader. All clients send commands to the leader.
- Heartbeats: The leader constantly sends "I am alive" messages to the followers.
- Election Timeout: If a follower doesn't hear from the leader for a short time, it assumes the leader is dead. It increments its "Term" (generation number) and asks others to vote for it to be the new leader.(Only Server can be the leader)
- Random Timing is Key: Every server waits a random amount of time before starting an election. If they didn't, they would all try to become leader at the same time, split the votes, and no one would win.

4. What Can Go Wrong? (The Tricky Parts)
- The "Disruptive Follower": A server that is cut off from the leader might keep increasing its Term number and asking for votes. If its messages somehow reach the healthy leader, the leader wil see the higher Term and step down, hurting the healthy servers.
- Unreliable Networks: Messages can be delayed, repeated, or lost. The system must be designed to never make mistakes even if messages arrive out of order or multiple times.