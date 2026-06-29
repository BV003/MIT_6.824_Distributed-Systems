package mr

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sync"
	"time"
)

type TaskState int

const (
	Idle TaskState = iota
	InProgress
	Completed
)

type TaskInfo struct {
	State     TaskState
	StartTime time.Time
	Filename  string
}

type Coordinator struct {
	mu          sync.Mutex
	nReduce     int
	nMap        int
	mapTasks    []TaskInfo
	reduceTasks []TaskInfo
	phase       TaskType
}

func (c *Coordinator) GetTask(args *GetTaskArgs, reply *GetTaskReply) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.phase == MapTask {
		allMapsDone := true
		for i := 0; i < len(c.mapTasks); i++ {
			if c.mapTasks[i].State == Idle {
				c.mapTasks[i].State = InProgress
				c.mapTasks[i].StartTime = time.Now()

				reply.Type = MapTask
				reply.TaskId = i
				reply.Filename = c.mapTasks[i].Filename
				reply.NReduce = c.nReduce
				reply.NMap = c.nMap
				return nil
			}
			if c.mapTasks[i].State != Completed {
				allMapsDone = false
			}
		}

		if allMapsDone {
			c.phase = ReduceTask
			// Fall through to reduce phase
		} else {
			reply.Type = WaitTask
			return nil
		}
	}

	if c.phase == ReduceTask {
		allReducesDone := true
		for i := 0; i < len(c.reduceTasks); i++ {
			if c.reduceTasks[i].State == Idle {
				c.reduceTasks[i].State = InProgress
				c.reduceTasks[i].StartTime = time.Now()

				reply.Type = ReduceTask
				reply.TaskId = i
				reply.NReduce = c.nReduce
				reply.NMap = c.nMap
				return nil
			}
			if c.reduceTasks[i].State != Completed {
				allReducesDone = false
			}
		}

		if allReducesDone {
			c.phase = ExitTask
			// Fall through to exit
		} else {
			reply.Type = WaitTask
			return nil
		}
	}

	reply.Type = ExitTask
	return nil
}

func (c *Coordinator) FinishedTask(args *FinishedTaskArgs, reply *FinishedTaskReply) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if args.Type == MapTask && c.phase == MapTask {
		c.mapTasks[args.TaskId].State = Completed
	} else if args.Type == ReduceTask && c.phase == ReduceTask {
		c.reduceTasks[args.TaskId].State = Completed
	}

	return nil
}

func (c *Coordinator) checkTimeouts() {
	for {
		time.Sleep(1 * time.Second)
		c.mu.Lock()
		if c.phase == ExitTask {
			c.mu.Unlock()
			return
		}

		now := time.Now()
		if c.phase == MapTask {
			for i := 0; i < len(c.mapTasks); i++ {
				if c.mapTasks[i].State == InProgress && now.Sub(c.mapTasks[i].StartTime) > 10*time.Second {
					c.mapTasks[i].State = Idle
				}
			}
		} else if c.phase == ReduceTask {
			for i := 0; i < len(c.reduceTasks); i++ {
				if c.reduceTasks[i].State == InProgress && now.Sub(c.reduceTasks[i].StartTime) > 10*time.Second {
					c.reduceTasks[i].State = Idle
				}
			}
		}
		c.mu.Unlock()
	}
}


//
// start a thread that listens for RPCs from worker.go
//
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

//
// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
//
func (c *Coordinator) Done() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.phase == ExitTask
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{}

	c.nReduce = nReduce
	c.nMap = len(files)
	c.phase = MapTask

	c.mapTasks = make([]TaskInfo, len(files))
	for i, file := range files {
		c.mapTasks[i] = TaskInfo{
			State:    Idle,
			Filename: file,
		}
	}

	c.reduceTasks = make([]TaskInfo, nReduce)
	for i := 0; i < nReduce; i++ {
		c.reduceTasks[i] = TaskInfo{
			State: Idle,
		}
	}

	c.server()
	go c.checkTimeouts()

	return &c
}
