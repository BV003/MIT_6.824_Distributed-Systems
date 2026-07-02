package rsm

import (
	"crypto/rand"
	"math/big"
	"sync"
	"time"

	"6.5840/kvsrv1/rpc"
	"6.5840/labrpc"
	"6.5840/raft1"
	"6.5840/raftapi"
	"6.5840/tester1"
)

var useRaftStateMachine bool // to plug in another raft besides raft1

type Op struct {
	Id      int64
	Term    int
	Command any
}

// A server (i.e., ../server.go) that wants to replicate itself calls
// MakeRSM and must implement the StateMachine interface.  This
// interface allows the rsm package to interact with the server for
// server-specific operations: the server must implement DoOp to
// execute an operation (e.g., a Get or Put request), and
// Snapshot/Restore to snapshot and restore the server's state.
type StateMachine interface {
	DoOp(any) any
	Snapshot() []byte
	Restore([]byte)
}

type Notification struct {
	id     int64
	term   int
	result any
}

type RSM struct {
	mu           sync.Mutex
	me           int
	rf           raftapi.Raft
	applyCh      chan raftapi.ApplyMsg
	maxraftstate int // snapshot if log grows this big
	sm           StateMachine

	pendingOps map[int]chan Notification
}

func nrand() int64 {
	max := big.NewInt(1)
	max.Lsh(max, 62)
	n, _ := rand.Int(rand.Reader, max)
	return n.Int64()
}

// MakeRSM() must return quickly, so it should start goroutines for
// any long-running work.
func MakeRSM(servers []*labrpc.ClientEnd, me int, persister *tester.Persister, maxraftstate int, sm StateMachine) *RSM {
	rsm := &RSM{
		me:           me,
		maxraftstate: maxraftstate,
		applyCh:      make(chan raftapi.ApplyMsg),
		sm:           sm,
		pendingOps:   make(map[int]chan Notification),
	}
	if !useRaftStateMachine {
		rsm.rf = raft.Make(servers, me, persister, rsm.applyCh)
	}

	// Read existing snapshot on restart
	snapshot := persister.ReadSnapshot()
	if len(snapshot) > 0 {
		rsm.sm.Restore(snapshot)
	}

	// Start the background reader goroutine
	go rsm.applyReader()

	return rsm
}

func (rsm *RSM) Raft() raftapi.Raft {
	return rsm.rf
}

func (rsm *RSM) applyReader() {
	for msg := range rsm.applyCh {
		if msg.CommandValid {
			op, ok := msg.Command.(Op)
			if !ok {
				continue
			}

			// Execute the command on the StateMachine
			res := rsm.sm.DoOp(op.Command)

			// Notify waiting Submit goroutine
			rsm.mu.Lock()
			ch, exists := rsm.pendingOps[msg.CommandIndex]
			if exists {
				delete(rsm.pendingOps, msg.CommandIndex)
				notification := Notification{
					id:     op.Id,
					term:   op.Term,
					result: res,
				}
				rsm.mu.Unlock()
				// Send notification without holding lock
				ch <- notification
			} else {
				rsm.mu.Unlock()
			}

			// Check if we need to snapshot
			if rsm.maxraftstate != -1 && rsm.rf.PersistBytes() >= rsm.maxraftstate {
				snapshotBytes := rsm.sm.Snapshot()
				rsm.rf.Snapshot(msg.CommandIndex, snapshotBytes)
			}
		} else if msg.SnapshotValid {
			// Restore the StateMachine using the snapshot
			rsm.sm.Restore(msg.Snapshot)
		}
	}

	// When applyCh is closed, it means the server is shutting down.
	// Clean up all pending channels to wake up any blocked Submit() calls.
	rsm.mu.Lock()
	for index, ch := range rsm.pendingOps {
		delete(rsm.pendingOps, index)
		close(ch)
	}
	rsm.mu.Unlock()
}

// Submit a command to Raft, and wait for it to be committed.  It
// should return ErrWrongLeader if client should find new leader and
// try again.
func (rsm *RSM) Submit(req any) (rpc.Err, any) {
	// Call Start first to find out if we are leader and get the initial index/term
	term, isLeader := rsm.rf.GetState()
	if !isLeader {
		return rpc.ErrWrongLeader, nil
	}

	op := Op{
		Id:      nrand(),
		Term:    term,
		Command: req,
	}

	index, startTerm, isLeader := rsm.rf.Start(op)
	if !isLeader {
		return rpc.ErrWrongLeader, nil
	}

	// Just in case the term changed between GetState() and Start()
	op.Term = startTerm

	rsm.mu.Lock()
	ch := make(chan Notification, 1)
	rsm.pendingOps[index] = ch
	rsm.mu.Unlock()

	defer func() {
		rsm.mu.Lock()
		delete(rsm.pendingOps, index)
		rsm.mu.Unlock()
	}()

	// Loop to periodically check for leadership changes or server shutdown
	for {
		select {
		case notification, ok := <-ch:
			if !ok {
				// Channel closed due to shutdown
				return rpc.ErrWrongLeader, nil
			}
			if notification.term != startTerm || notification.id != op.Id {
				return rpc.ErrWrongLeader, nil
			}
			return rpc.OK, notification.result
		default:
			// Check if we lost leadership or term changed
			currTerm, currIsLeader := rsm.rf.GetState()
			if !currIsLeader || currTerm != startTerm {
				return rpc.ErrWrongLeader, nil
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}
