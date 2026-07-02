package kvraft

import (
	"time"

	"6.5840/kvsrv1/rpc"
	"6.5840/kvtest1"
	"6.5840/tester1"
)

type Clerk struct {
	clnt    *tester.Clnt
	servers []string
	leader  int
}

func MakeClerk(clnt *tester.Clnt, servers []string) kvtest.IKVClerk {
	ck := &Clerk{
		clnt:    clnt,
		servers: servers,
		leader:  0, // Start by talking to server 0
	}
	return ck
}

// Get fetches the current value and version for a key.  It returns
// ErrNoKey if the key does not exist. It keeps trying forever in the
// face of all other errors.
func (ck *Clerk) Get(key string) (string, rpc.Tversion, rpc.Err) {
	args := rpc.GetArgs{Key: key}
	leader := ck.leader

	for {
		reply := rpc.GetReply{}
		ok := ck.clnt.Call(ck.servers[leader], "KVServer.Get", &args, &reply)
		if ok && reply.Err != rpc.ErrWrongLeader {
			ck.leader = leader
			return reply.Value, reply.Version, reply.Err
		}
		// If call failed or wrong leader, try the next server
		leader = (leader + 1) % len(ck.servers)
		time.Sleep(20 * time.Millisecond)
	}
}

// Put updates key with value only if the version in the
// request matches the version of the key at the server.
func (ck *Clerk) Put(key string, value string, version rpc.Tversion) rpc.Err {
	args := rpc.PutArgs{
		Key:     key,
		Value:   value,
		Version: version,
	}
	leader := ck.leader
	first := true

	for {
		reply := rpc.PutReply{}
		ok := ck.clnt.Call(ck.servers[leader], "KVServer.Put", &args, &reply)
		if ok && reply.Err != rpc.ErrWrongLeader {
			ck.leader = leader
			if reply.Err == rpc.ErrVersion {
				if first {
					return rpc.ErrVersion
				} else {
					return rpc.ErrMaybe
				}
			}
			return reply.Err
		}
		// If call failed or wrong leader, try the next server
		leader = (leader + 1) % len(ck.servers)
		first = false
		time.Sleep(20 * time.Millisecond)
	}
}
