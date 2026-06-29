package mr

import "fmt"
import "log"
import "net/rpc"
import "hash/fnv"
import "encoding/json"
import "io"
import "os"
import "sort"
import "time"

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

type ByKey []KeyValue

func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}


//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	for {
		args := GetTaskArgs{}
		reply := GetTaskReply{}

		ok := call("Coordinator.GetTask", &args, &reply)
		if !ok || reply.Type == ExitTask {
			return
		}

		switch reply.Type {
		case MapTask:
			doMap(mapf, reply.TaskId, reply.Filename, reply.NReduce)
		case ReduceTask:
			doReduce(reducef, reply.TaskId, reply.NMap)
		case WaitTask:
			time.Sleep(1 * time.Second)
		}
	}
}

func doMap(mapf func(string, string) []KeyValue, taskId int, filename string, nReduce int) {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("cannot open %v", filename)
	}
	content, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("cannot read %v", filename)
	}
	file.Close()

	kva := mapf(filename, string(content))

	buckets := make([][]KeyValue, nReduce)
	for _, kv := range kva {
		idx := ihash(kv.Key) % nReduce
		buckets[idx] = append(buckets[idx], kv)
	}

	for r := 0; r < nReduce; r++ {
		tempFile, err := os.CreateTemp("", "mr-tmp-*")
		if err != nil {
			log.Fatalf("cannot create temp file: %v", err)
		}

		enc := json.NewEncoder(tempFile)
		for _, kv := range buckets[r] {
			if err := enc.Encode(&kv); err != nil {
				log.Fatalf("cannot encode json: %v", err)
			}
		}
		tempFile.Close()

		finalName := fmt.Sprintf("mr-%v-%v", taskId, r)
		if err := os.Rename(tempFile.Name(), finalName); err != nil {
			log.Fatalf("cannot rename intermediate file: %v", err)
		}
	}

	finArgs := FinishedTaskArgs{
		Type:   MapTask,
		TaskId: taskId,
	}
	finReply := FinishedTaskReply{}
	call("Coordinator.FinishedTask", &finArgs, &finReply)
}

func doReduce(reducef func(string, []string) string, taskId int, nMap int) {
	var kva []KeyValue

	for m := 0; m < nMap; m++ {
		filename := fmt.Sprintf("mr-%v-%v", m, taskId)
		file, err := os.Open(filename)
		if err != nil {
			continue
		}

		dec := json.NewDecoder(file)
		for {
			var kv KeyValue
			if err := dec.Decode(&kv); err != nil {
				break
			}
			kva = append(kva, kv)
		}
		file.Close()
	}

	sort.Sort(ByKey(kva))

	tempFile, err := os.CreateTemp("", "mr-out-tmp-*")
	if err != nil {
		log.Fatalf("cannot create temp file: %v", err)
	}

	i := 0
	for i < len(kva) {
		j := i + 1
		for j < len(kva) && kva[j].Key == kva[i].Key {
			j++
		}
		var values []string
		for k := i; k < j; k++ {
			values = append(values, kva[k].Value)
		}
		output := reducef(kva[i].Key, values)

		fmt.Fprintf(tempFile, "%v %v\n", kva[i].Key, output)

		i = j
	}
	tempFile.Close()

	finalName := fmt.Sprintf("mr-out-%v", taskId)
	if err := os.Rename(tempFile.Name(), finalName); err != nil {
		log.Fatalf("cannot rename reduce output file: %v", err)
	}

	finArgs := FinishedTaskArgs{
		Type:   ReduceTask,
		TaskId: taskId,
	}
	finReply := FinishedTaskReply{}
	call("Coordinator.FinishedTask", &finArgs, &finReply)
}

//
// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		return false
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	return false
}
