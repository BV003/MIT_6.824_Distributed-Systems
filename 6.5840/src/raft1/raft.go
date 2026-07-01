package raft

// The file raftapi/raft.go defines the interface that raft must
// expose to servers (or the tester), but see comments below for each
// of these functions for more details.
//
// Make() creates a new raft peer that implements the raft interface.

import (
	"bytes"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"6.5840/labgob"
	"6.5840/labrpc"
	"6.5840/raftapi"
	"6.5840/tester1"
)

type Role int
const (
	Follower Role = iota
	Candidate
	Leader
)

type LogEntry struct {
	Command interface{}
	Term    int
}

type AppendEntriesArgs struct {
	Term         int
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term    int
	Success bool
	ConflictTerm  int // Add this
	ConflictIndex int // Add this
}

type InstallSnapshotArgs struct {
	Term              int
	LeaderId          int
	LastIncludedIndex int
	LastIncludedTerm  int
	Data              []byte
}

type InstallSnapshotReply struct {
	Term int
}

// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *tester.Persister   // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (3A, 3B, 3C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	// Figure 2 persistent state
	currentTerm int
	votedFor    int
	log         []LogEntry // 1-indexed (using a sentinel at index 0)
	lastIncludedIndex int
	lastIncludedTerm  int

		// Volatile state on all servers
	commitIndex int
	lastApplied int

	// Volatile state on leaders
	nextIndex  []int
	matchIndex []int

	// Additional state for tracking role and election timeout
	role          Role
	lastHeartbeat time.Time

	applyCh chan raftapi.ApplyMsg
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.currentTerm, rf.role == Leader
}

// how many bytes in Raft's persisted log?
func (rf *Raft) PersistBytes() int {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.persister.RaftStateSize()
}


// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if index <= rf.lastIncludedIndex {
		return
	}

	relIdx := rf.getRelativeIndex(index)
	rf.lastIncludedTerm = rf.log[relIdx].Term

	newLog := make([]LogEntry, 1 + rf.lastLogIndex() - index)
	newLog[0] = LogEntry{Term: rf.lastIncludedTerm}
	copy(newLog[1:], rf.log[relIdx+1:])
	rf.log = newLog

	rf.lastIncludedIndex = index

	if rf.commitIndex < index {
		rf.commitIndex = index
	}
	if rf.lastApplied < index {
		rf.lastApplied = index
	}

	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	_ = e.Encode(rf.currentTerm)
	_ = e.Encode(rf.votedFor)
	_ = e.Encode(rf.log)
	_ = e.Encode(rf.lastIncludedIndex)
	_ = e.Encode(rf.lastIncludedTerm)
	raftstate := w.Bytes()
	rf.persister.Save(raftstate, snapshot)
}


// example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	Term         int // Candidate's term
	CandidateId  int // Candidate requesting vote
	LastLogIndex int // Index of candidate's last log entry (for Election Restriction)
	LastLogTerm  int // Term of candidate's last log entry (for Election Restriction)
}

// example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	Term        int  // Current term of voting server, for candidate to update itself
	VoteGranted bool // True means candidate received vote
}

// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// Rule 1: Term check
	// If the candidate's term is older than ours, reject the vote immediately.
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}

	// Rule 2: Higher term check (Term update & state reset)
	// If the candidate's term is larger than ours, we must step down and update our term.
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.role = Follower
		rf.votedFor = -1
		rf.persist()
	}

	reply.Term = rf.currentTerm
	reply.VoteGranted = false

	// Rule 3: Election Restriction (Up-to-date log check)
	// Find our last log entry's index and term.
	lastLogIndex := rf.lastLogIndex()
	lastLogTerm := rf.lastLogTerm()

	// Check if the candidate's log is "at least as up-to-date" as ours.
	logUpToDate := false
	if args.LastLogTerm > lastLogTerm {
		logUpToDate = true
	} else if args.LastLogTerm == lastLogTerm {
		if args.LastLogIndex >= lastLogIndex {
			logUpToDate = true
		}
	}

	// Grant vote if we haven't voted for anyone else in this term,
	// and the candidate's log is up-to-date.
	if (rf.votedFor == -1 || rf.votedFor == args.CandidateId) && logUpToDate {
		rf.votedFor = args.CandidateId
		reply.VoteGranted = true
		rf.persist() 
		
		// Reset the election timeout since we voted for a valid candidate!
		rf.lastHeartbeat = time.Now()
	}
}

// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}


// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.role != Leader {
		return -1, -1, false
	}

	// 1. Append entry to local log
	entry := LogEntry{
		Command: command,
		Term:    rf.currentTerm,
	}
	rf.log = append(rf.log, entry)
	rf.persist()

	index := rf.lastLogIndex()
	term := rf.currentTerm

	// 2. Proactively trigger replication to all peers immediately
	go rf.broadcastAppendEntries()

	return index, term, true
}

func (rf *Raft) broadcastAppendEntries() {
	rf.mu.Lock()
	if rf.role != Leader {
		rf.mu.Unlock()
		return
	}
	term := rf.currentTerm
	rf.mu.Unlock()

	for peer := range rf.peers {
		if peer == rf.me {
			continue
		}
		go rf.sendAppendEntriesToPeer(peer, term)
	}
}

func (rf *Raft) sendAppendEntriesToPeer(peer int, term int) {
	rf.mu.Lock()
	if rf.role != Leader || rf.currentTerm != term {
		rf.mu.Unlock()
		return
	}

	// If the follower is too far behind and the entries they need are compacted:
	if rf.nextIndex[peer] <= rf.lastIncludedIndex {
		args := InstallSnapshotArgs{
			Term:              term,
			LeaderId:          rf.me,
			LastIncludedIndex: rf.lastIncludedIndex,
			LastIncludedTerm:  rf.lastIncludedTerm,
			Data:              rf.persister.ReadSnapshot(),
		}
		rf.mu.Unlock()

		var reply InstallSnapshotReply
		if rf.peers[peer].Call("Raft.InstallSnapshot", &args, &reply) {
			rf.mu.Lock()
			defer rf.mu.Unlock()

			if rf.role != Leader || rf.currentTerm != term {
				return
			}

			if reply.Term > rf.currentTerm {
				rf.currentTerm = reply.Term
				rf.role = Follower
				rf.votedFor = -1
				rf.persist()
				return
			}

			if rf.matchIndex[peer] < args.LastIncludedIndex {
				rf.matchIndex[peer] = args.LastIncludedIndex
			}
			rf.nextIndex[peer] = rf.matchIndex[peer] + 1
		}
		return
	}

	prevLogIndex := rf.nextIndex[peer] - 1
	prevLogTerm := rf.log[rf.getRelativeIndex(prevLogIndex)].Term
	
	// Copy slice of log starting from nextIndex
	entries := make([]LogEntry, len(rf.log[rf.getRelativeIndex(rf.nextIndex[peer]):]))
	copy(entries, rf.log[rf.getRelativeIndex(rf.nextIndex[peer]):])

	args := AppendEntriesArgs{
		Term:         term,
		LeaderId:     rf.me,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  prevLogTerm,
		Entries:      entries,
		LeaderCommit: rf.commitIndex,
	}
	rf.mu.Unlock()

	var reply AppendEntriesReply
	if rf.peers[peer].Call("Raft.AppendEntries", &args, &reply) {
		rf.mu.Lock()
		defer rf.mu.Unlock()

		if rf.role != Leader || rf.currentTerm != term {
			return
		}

		if reply.Term > rf.currentTerm {
			rf.currentTerm = reply.Term
			rf.role = Follower
			rf.votedFor = -1
			rf.persist()
			return
		}

		if reply.Success {
			// Update nextIndex and matchIndex for follower
			rf.matchIndex[peer] = prevLogIndex + len(entries)
			rf.nextIndex[peer] = rf.matchIndex[peer] + 1

			// Check if we can commit any new entries
			rf.updateCommitIndex()
		} else {
			// Fast Backup Optimization
			if reply.ConflictTerm == -1 {
				// Follower's log was too short
				rf.nextIndex[peer] = reply.ConflictIndex
			} else {
				// Follower had a conflicting term
				leaderHasConflictTerm := false
				conflictIndexInLeader := -1
				for i := len(rf.log) - 1; i > 0; i-- {
					if rf.log[i].Term == reply.ConflictTerm {
						leaderHasConflictTerm = true
						conflictIndexInLeader = rf.getAbsoluteIndex(i)
						break
					}
				}
				if leaderHasConflictTerm {
					rf.nextIndex[peer] = conflictIndexInLeader + 1
				} else {
					rf.nextIndex[peer] = reply.ConflictIndex
				}
			}
		}
	}
}


func (rf *Raft) updateCommitIndex() {
	lastLogIndex := rf.lastLogIndex()
	for N := lastLogIndex; N > rf.commitIndex; N-- {
		if rf.log[rf.getRelativeIndex(N)].Term != rf.currentTerm {
			// Raft paper Figure 8: Leader cannot commit entries from previous terms by counting replicas
			break
		}
		
		count := 1 // Count ourselves
		for peer := range rf.peers {
			if peer != rf.me && rf.matchIndex[peer] >= N {
				count++
			}
		}

		if count > len(rf.peers)/2 {
			rf.commitIndex = N
			break
		}
	}
}
// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) ticker() {
	for rf.killed() == false {
		rf.mu.Lock()
		timeout := time.Duration(300+rand.Int63()%300) * time.Millisecond
		if rf.role != Leader && time.Since(rf.lastHeartbeat) > timeout {
			rf.startElection()
		}
		rf.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
	}
}

func (rf *Raft) startElection() {
	rf.role = Candidate
	rf.currentTerm++
	rf.votedFor = rf.me
	rf.persist()
	rf.lastHeartbeat = time.Now()

	term := rf.currentTerm
	lastLogIndex := rf.lastLogIndex()
	lastLogTerm := rf.lastLogTerm()

	votes := 1 // self vote
	for peer := range rf.peers {
		if peer == rf.me {
			continue
		}
		go func(peer int) {
			args := RequestVoteArgs{
				Term:         term,
				CandidateId:  rf.me,
				LastLogIndex: lastLogIndex,
				LastLogTerm:  lastLogTerm,
			}
			var reply RequestVoteReply
			if rf.sendRequestVote(peer, &args, &reply) {
				rf.mu.Lock()
				defer rf.mu.Unlock()

				if rf.role != Candidate || rf.currentTerm != term {
					return
				}

				if reply.Term > rf.currentTerm {
					rf.currentTerm = reply.Term
					rf.role = Follower
					rf.votedFor = -1
					rf.persist()
					return
				}

				if reply.VoteGranted {
					votes++
					if votes > len(rf.peers)/2 && rf.role == Candidate {
						rf.role = Leader
						rf.lastHeartbeat = time.Now()
						// Initialize volatile leader state (Figure 2)
						lastLogIndex := rf.lastLogIndex()
						for i := range rf.peers {
							rf.nextIndex[i] = lastLogIndex + 1
							rf.matchIndex[i] = 0
						}
						// Immediately broadcast heartbeats upon winning
						go rf.sendHeartbeats()
					}
				}
			}
		}(peer)
	}
}

func (rf *Raft) getRelativeIndex(absoluteIndex int) int {
	return absoluteIndex - rf.lastIncludedIndex
}

func (rf *Raft) getAbsoluteIndex(relativeIndex int) int {
	return relativeIndex + rf.lastIncludedIndex
}

func (rf *Raft) lastLogIndex() int {
	return len(rf.log) - 1 + rf.lastIncludedIndex
}

func (rf *Raft) lastLogTerm() int {
	return rf.log[len(rf.log)-1].Term
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *tester.Persister, applyCh chan raftapi.ApplyMsg) raftapi.Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.applyCh = applyCh

	// Initializing Raft State
	rf.currentTerm = 0
	rf.votedFor = -1
	rf.log = make([]LogEntry, 1) // sentinel at index 0
	rf.log[0] = LogEntry{Term: 0}

	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.nextIndex = make([]int, len(peers))
	rf.matchIndex = make([]int, len(peers))

	rf.role = Follower
	rf.lastHeartbeat = time.Now()

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	rf.commitIndex = rf.lastIncludedIndex
	rf.lastApplied = rf.lastIncludedIndex

	// start ticker goroutine to start elections
	go rf.ticker()

	go rf.applierLoop(applyCh)

	return rf
}

func (rf *Raft) sendHeartbeats() {
	for !rf.killed() {
		rf.mu.Lock()
		if rf.role != Leader {
			rf.mu.Unlock()
			return
		}
		term := rf.currentTerm
		rf.mu.Unlock()

		for peer := range rf.peers {
			if peer == rf.me {
				continue
			}
			go rf.sendAppendEntriesToPeer(peer, term)
		}
		time.Sleep(100 * time.Millisecond) // 10 heartbeats per second
	}
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// Rule 1: Term check
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}

	// Rule 2: Higher term step down
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.role = Follower
		rf.votedFor = -1
		rf.persist()
	} else if rf.role == Candidate {
		rf.role = Follower
	}

	reply.Term = rf.currentTerm
	rf.lastHeartbeat = time.Now()

	// If PrevLogIndex is smaller than lastIncludedIndex, we return false but point the leader to lastIncludedIndex+1
	if args.PrevLogIndex < rf.lastIncludedIndex {
		reply.Success = false
		reply.ConflictIndex = rf.lastIncludedIndex + 1
		reply.ConflictTerm = -1
		return
	}

	// Fast Backup 1: Follower log is too short
	if args.PrevLogIndex > rf.lastLogIndex() {
		reply.Success = false
		reply.ConflictIndex = rf.lastLogIndex() + 1
		reply.ConflictTerm = -1
		return
	}

	// Fast Backup 2: Follower has mismatched term at PrevLogIndex
	relPrevLogIndex := rf.getRelativeIndex(args.PrevLogIndex)
	if rf.log[relPrevLogIndex].Term != args.PrevLogTerm {
		reply.Success = false
		reply.ConflictTerm = rf.log[relPrevLogIndex].Term
		
		// Find first index of that term
		firstIndex := args.PrevLogIndex
		for firstIndex > rf.lastIncludedIndex && rf.log[rf.getRelativeIndex(firstIndex-1)].Term == reply.ConflictTerm {
			firstIndex--
		}
		reply.ConflictIndex = firstIndex
		return
	}

	// Rule 3 & 4: Append new entries and handle conflicts
	hasNewEntries := false
	for i, entry := range args.Entries {
		index := args.PrevLogIndex + 1 + i
		relIndex := rf.getRelativeIndex(index)
		if relIndex < len(rf.log) {
			if rf.log[relIndex].Term != entry.Term {
				rf.log = rf.log[:relIndex] // delete conflicting entries
				rf.log = append(rf.log, entry)
				hasNewEntries = true
			}
		} else {
			rf.log = append(rf.log, entry)
			hasNewEntries = true
		}
	}
	if hasNewEntries {
		rf.persist() 
	}

	reply.Success = true

	// Rule 5: Update commitIndex
	if args.LeaderCommit > rf.commitIndex {
		lastNewEntryIndex := args.PrevLogIndex + len(args.Entries)
		newCommitIndex := args.LeaderCommit
		if lastNewEntryIndex < newCommitIndex {
			newCommitIndex = lastNewEntryIndex
		}
		if newCommitIndex > rf.commitIndex {
			rf.commitIndex = newCommitIndex
		}
	}
}

func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	rf.mu.Lock()

	reply.Term = rf.currentTerm
	if args.Term < rf.currentTerm {
		rf.mu.Unlock()
		return
	}

	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.role = Follower
		rf.votedFor = -1
		rf.persist()
	} else if rf.role == Candidate {
		rf.role = Follower
	}

	rf.lastHeartbeat = time.Now()

	if args.LastIncludedIndex <= rf.lastIncludedIndex {
		rf.mu.Unlock()
		return
	}

	relIdx := rf.getRelativeIndex(args.LastIncludedIndex)
	if relIdx > 0 && relIdx < len(rf.log) && rf.log[relIdx].Term == args.LastIncludedTerm {
		newLog := make([]LogEntry, 1 + rf.lastLogIndex() - args.LastIncludedIndex)
		newLog[0] = LogEntry{Term: args.LastIncludedTerm}
		copy(newLog[1:], rf.log[relIdx+1:])
		rf.log = newLog
	} else {
		rf.log = make([]LogEntry, 1)
		rf.log[0] = LogEntry{Term: args.LastIncludedTerm}
	}

	rf.lastIncludedIndex = args.LastIncludedIndex
	rf.lastIncludedTerm = args.LastIncludedTerm

	if rf.commitIndex < args.LastIncludedIndex {
		rf.commitIndex = args.LastIncludedIndex
	}
	if rf.lastApplied < args.LastIncludedIndex {
		rf.lastApplied = args.LastIncludedIndex
	}

	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	_ = e.Encode(rf.currentTerm)
	_ = e.Encode(rf.votedFor)
	_ = e.Encode(rf.log)
	_ = e.Encode(rf.lastIncludedIndex)
	_ = e.Encode(rf.lastIncludedTerm)
	raftstate := w.Bytes()
	rf.persister.Save(raftstate, args.Data)

	rf.mu.Unlock()

	go func(applyCh chan raftapi.ApplyMsg, snapshotMsg raftapi.ApplyMsg) {
		applyCh <- snapshotMsg
	}(rf.applyCh, raftapi.ApplyMsg{
		SnapshotValid: true,
		Snapshot:      args.Data,
		SnapshotTerm:  args.LastIncludedTerm,
		SnapshotIndex: args.LastIncludedIndex,
	})
}

func (rf *Raft) applierLoop(applyCh chan raftapi.ApplyMsg) {
	for !rf.killed() {
		rf.mu.Lock()
		for rf.commitIndex <= rf.lastApplied && !rf.killed() {
			rf.mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			rf.mu.Lock()
		}

		if rf.killed() {
			rf.mu.Unlock()
			return
		}

		var messages []raftapi.ApplyMsg
		for rf.lastApplied < rf.commitIndex {
			rf.lastApplied++
			messages = append(messages, raftapi.ApplyMsg{
				CommandValid: true,
				Command:      rf.log[rf.getRelativeIndex(rf.lastApplied)].Command,
				CommandIndex: rf.lastApplied,
			})
		}
		rf.mu.Unlock()

		for _, msg := range messages {
			DPrintf("[%d] Applying: Index=%d Command=%v", rf.me, msg.CommandIndex, msg.Command)
			applyCh <- msg
		}
	}
}

func (rf *Raft) persist() {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	if err := e.Encode(rf.currentTerm); err != nil {
		DPrintf("persist encode currentTerm err: %v", err)
		return
	}
	if err := e.Encode(rf.votedFor); err != nil {
		DPrintf("persist encode votedFor err: %v", err)
		return
	}
	if err := e.Encode(rf.log); err != nil {
		DPrintf("persist encode log err: %v", err)
		return
	}
	if err := e.Encode(rf.lastIncludedIndex); err != nil {
		DPrintf("persist encode lastIncludedIndex err: %v", err)
		return
	}
	if err := e.Encode(rf.lastIncludedTerm); err != nil {
		DPrintf("persist encode lastIncludedTerm err: %v", err)
		return
	}
	raftstate := w.Bytes()
	rf.persister.Save(raftstate, rf.persister.ReadSnapshot())
}

func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	
	var currentTerm int
	var votedFor int
	var log []LogEntry
	var lastIncludedIndex int
	var lastIncludedTerm int

	if d.Decode(&currentTerm) != nil ||
	   d.Decode(&votedFor) != nil ||
	   d.Decode(&log) != nil ||
	   d.Decode(&lastIncludedIndex) != nil ||
	   d.Decode(&lastIncludedTerm) != nil {
		DPrintf("readPersist decode error!")
		return
	}

	rf.currentTerm = currentTerm
	rf.votedFor = votedFor
	rf.log = log
	rf.lastIncludedIndex = lastIncludedIndex
	rf.lastIncludedTerm = lastIncludedTerm
}