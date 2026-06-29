package kvsrv

import (
	"log"
	"sync"

	"6.5840/kvsrv1/rpc"
	"6.5840/labrpc"
	"6.5840/tester1"
)

const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

type valueVersion struct {
	val string
	ver rpc.Tversion
}



type KVServer struct {
	mu sync.Mutex
	store map[string]valueVersion
}

func MakeKVServer() *KVServer {
	kv := &KVServer{}
	kv.store = make(map[string]valueVersion)
	return kv
}

// Get returns the value and version for args.Key, if args.Key
// exists. Otherwise, Get returns ErrNoKey.
func (kv *KVServer) Get(args *rpc.GetArgs, reply *rpc.GetReply) {
		kv.mu.Lock()
	defer kv.mu.Unlock()

	item, exists := kv.store[args.Key]
	if !exists {
		reply.Err = rpc.ErrNoKey
		return
	}

	reply.Value = item.val
	reply.Version = item.ver
	reply.Err = rpc.OK
}

// Put updates key with value if version matches
func (kv *KVServer) Put(args *rpc.PutArgs, reply *rpc.PutReply) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	item, exists := kv.store[args.Key]
	if !exists {
		if args.Version == 0 {
			kv.store[args.Key] = valueVersion{
				val: args.Value,
				ver: 1,
			}
			reply.Err = rpc.OK
		} else {
			reply.Err = rpc.ErrNoKey
		}
		return
	}

	if item.ver != args.Version {
		reply.Err = rpc.ErrVersion
		return
	}

	kv.store[args.Key] = valueVersion{
		val: args.Value,
		ver: item.ver + 1,
	}
	reply.Err = rpc.OK
}

// You can ignore Kill() for this lab
func (kv *KVServer) Kill() {
}


// You can ignore all arguments; they are for replicated KVservers
func StartKVServer(ends []*labrpc.ClientEnd, gid tester.Tgid, srv int, persister *tester.Persister) []tester.IService {
	kv := MakeKVServer()
	return []tester.IService{kv}
}
