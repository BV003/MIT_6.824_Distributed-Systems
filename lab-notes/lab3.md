# Raft

A replicated service achieves fault tolerance by storing complete copies of its state (i.e., data) on multiple replica servers.

Raft organizes client requests into a sequence, called the log, and ensures that all the replica servers see the same log.

