package lock

import (
	"time"
	"6.5840/kvsrv1/rpc"
	"6.5840/kvtest1"
)

type Lock struct {
	ck   kvtest.IKVClerk
	l    string
	myID string
}


func MakeLock(ck kvtest.IKVClerk, l string) *Lock {
	lk := &Lock{
		ck:   ck,
		l:    l,
		myID: kvtest.RandValue(8),
	}
	return lk
}

func (lk *Lock) Acquire() {
	for {
		val, ver, err := lk.ck.Get(lk.l)
		if err == rpc.ErrNoKey {
			// Key doesn't exist, try to acquire it using version 0
			putErr := lk.ck.Put(lk.l, lk.myID, 0)
			if putErr == rpc.OK {
				return
			}
			if putErr == rpc.ErrMaybe {
				v, _, e := lk.ck.Get(lk.l)
				if e == rpc.OK && v == lk.myID {
					return
				}
			}
		} else if err == rpc.OK {
			if val == "" {
				// Key exists but is free, try to acquire it using the current version
				putErr := lk.ck.Put(lk.l, lk.myID, ver)
				if putErr == rpc.OK {
					return
				}
				if putErr == rpc.ErrMaybe {
					v, _, e := lk.ck.Get(lk.l)
					if e == rpc.OK && v == lk.myID {
						return
					}
				}
			}
		}
		// Lock is busy, or Put failed due to concurrency. Sleep and retry.
		time.Sleep(20 * time.Millisecond)
	}
}

func (lk *Lock) Release() {
	for {
		val, ver, err := lk.ck.Get(lk.l)
		if err == rpc.OK && val == lk.myID {
			putErr := lk.ck.Put(lk.l, "", ver)
			if putErr == rpc.OK {
				return
			}
			// If putErr is ErrMaybe or ErrVersion, we loop back to check if the lock
			// was successfully released (or taken by someone else) in Get().
		} else {
			// We do not hold the lock anymore, or it doesn't exist, so we are done.
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}
