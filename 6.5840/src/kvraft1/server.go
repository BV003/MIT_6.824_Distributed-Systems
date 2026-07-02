package kvraft

import (
	"bytes"
	"sync"
	"sync/atomic"

	"6.5840/kvraft1/rsm"
	"6.5840/kvsrv1/rpc"
	"6.5840/labgob"
	"6.5840/labrpc"
	"6.5840/tester1"
)

type valueVersion struct {
	Val string
	Ver rpc.Tversion
}

type KVServer struct {
	mu   sync.Mutex
	me   int
	dead int32 // set by Kill()
	rsm  *rsm.RSM

	// In-memory key-value database
	store map[string]valueVersion
}

// DoOp is called by the RSM layer when a replicated command is committed.
func (kv *KVServer) DoOp(req any) any {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	switch args := req.(type) {
	case rpc.GetArgs:
		reply := rpc.GetReply{}
		item, exists := kv.store[args.Key]
		if !exists {
			reply.Err = rpc.ErrNoKey
		} else {
			reply.Value = item.Val
			reply.Version = item.Ver
			reply.Err = rpc.OK
		}
		return reply

	case rpc.PutArgs:
		reply := rpc.PutReply{}
		item, exists := kv.store[args.Key]
		if !exists {
			if args.Version == 0 {
				kv.store[args.Key] = valueVersion{
					Val: args.Value,
					Ver: 1,
				}
				reply.Err = rpc.OK
			} else {
				reply.Err = rpc.ErrNoKey
			}
		} else {
			if item.Ver != args.Version {
				reply.Err = rpc.ErrVersion
			} else {
				kv.store[args.Key] = valueVersion{
					Val: args.Value,
					Ver: item.Ver + 1,
				}
				reply.Err = rpc.OK
			}
		}
		return reply
	}

	return nil
}

func (kv *KVServer) Snapshot() []byte {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	if err := e.Encode(kv.store); err != nil {
		panic(err)
	}
	return w.Bytes()
}

func (kv *KVServer) Restore(data []byte) {
	if len(data) == 0 {
		return
	}

	kv.mu.Lock()
	defer kv.mu.Unlock()

	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	var store map[string]valueVersion
	if err := d.Decode(&store); err != nil {
		panic(err)
	}
	kv.store = store
}

func (kv *KVServer) Get(args *rpc.GetArgs, reply *rpc.GetReply) {
	err, res := kv.rsm.Submit(*args)
	if err != rpc.OK {
		reply.Err = err
		return
	}
	*reply = res.(rpc.GetReply)
}

func (kv *KVServer) Put(args *rpc.PutArgs, reply *rpc.PutReply) {
	err, res := kv.rsm.Submit(*args)
	if err != rpc.OK {
		reply.Err = err
		return
	}
	*reply = res.(rpc.PutReply)
}

// the tester calls Kill() when a KVServer instance won't
// be needed again.
func (kv *KVServer) Kill() {
	atomic.StoreInt32(&kv.dead, 1)
}

func (kv *KVServer) killed() bool {
	z := atomic.LoadInt32(&kv.dead)
	return z == 1
}

// StartKVServer() and MakeRSM() must return quickly, so they should
// start goroutines for any long-running work.
func StartKVServer(servers []*labrpc.ClientEnd, gid tester.Tgid, me int, persister *tester.Persister, maxraftstate int) []tester.IService {
	// Register RPC types for labgob
	labgob.Register(rsm.Op{})
	labgob.Register(rpc.PutArgs{})
	labgob.Register(rpc.GetArgs{})

	kv := &KVServer{
		me:    me,
		store: make(map[string]valueVersion),
	}

	kv.rsm = rsm.MakeRSM(servers, me, persister, maxraftstate, kv)
	return []tester.IService{kv, kv.rsm.Raft()}
}
