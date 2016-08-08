package task

import (
	"container/heap"
	"fmt"
	"sync"
	"time"
)

const (
	TASK_TYPE_PERIODIC = 1
	TASK_TYPE_ONESHOT  = 2

	TASK_SOURCE_PUSH = 1

	TICK = time.Minute
)

type Task struct {
	Id                int
	UserIdentifier    string
	Type              int
	TaskSource        int //task type could be implement by 'stub' like concept
	Period            int
	LastExecutionTime time.Time
	NextExecutionTime time.Time
	CreateTime        time.Time
	UpdateTime        time.Time

	Context interface{}
}

type TaskLog struct {
	ID            int
	TaskId        int
	Status        int
	ExecutionTime int
}

//NOTE context must json-able
type TaskHandler func(task *Task) error

func (Task) TableName() string {
	return "tb_task"
}

type PriorityQueue []*Task
type TaskManager struct {
	TaskMap struct {
		sync.RWMutex
		inner map[string]*Task
	}
	TaskPriorityQueue struct {
		sync.RWMutex
		inner PriorityQueue
	}

	stop chan bool
	wake chan bool
}

func NewTaskManager() (*TaskManager, error) {
	m := &TaskManager{
		TaskMap: struct {
			sync.RWMutex
			inner map[string]*Task
		}{
			inner: make(map[string]*Task),
		},
		TaskPriorityQueue: struct {
			sync.RWMutex
			inner PriorityQueue
		}{
			inner: make(PriorityQueue, 0),
		},

		stop: make(chan bool),
		wake: make(chan bool),
	}

	heap.Init(&m.TaskPriorityQueue.inner)

	return m, nil
}

//implement taskmanager queue
//not these function is NOT thread-safe
func (q *PriorityQueue) Swap(i, j int) {
	(*q)[i], (*q)[j] = (*q)[j], (*q)[i]
}

func (q *PriorityQueue) Len() int {
	return len(*q)
}

func (q *PriorityQueue) Less(i, j int) bool {
	return (*q)[i].NextExecutionTime.After((*q)[j].NextExecutionTime)
}

func (q *PriorityQueue) Pop() interface{} {
	old := *q
	n := len(*q)
	item := (*q)[n-1]
	*q = old[0 : n-1]
	return item
}

func (q *PriorityQueue) Push(x interface{}) {
	*q = append(*q, x.(*Task))
}

//load task from db
func (*TaskManager) syncTask() error {
	return nil
}

func (*TaskManager) saveTaskToDB(task *Task) error {
	return nil
}

func (*TaskManager) doTask(task *Task) {

}

func (*TaskManager) logTaskExecution(task *Task) {

}

func (taskManager *TaskManager) internalAddTask(task *Task) error {
	var ok bool
	taskManager.TaskMap.RLock()
	_, ok = taskManager.TaskMap.inner[task.UserIdentifier]
	taskManager.TaskMap.RUnlock()

	if ok {
		return fmt.Errorf("task exists")
	}

	taskManager.TaskMap.Lock()
	_, ok = taskManager.TaskMap.inner[task.UserIdentifier]
	if ok {
		taskManager.TaskMap.Unlock()
		return fmt.Errorf("tasks exists")
	}

	taskManager.TaskMap.inner[task.UserIdentifier] = task
	taskManager.TaskMap.Unlock()

	taskManager.TaskPriorityQueue.Lock()
	heap.Push(&taskManager.TaskPriorityQueue.inner, task)
	taskManager.TaskPriorityQueue.Unlock()

	return nil
}

func (taskManager *TaskManager) getNextWakeupTime() time.Time {
	taskManager.TaskPriorityQueue.RLock()
	defer taskManager.TaskPriorityQueue.RUnlock()

	if taskManager.TaskPriorityQueue.inner.Len() == 0 {
		return time.Now().Add(TICK)
	} else {
		return taskManager.TaskPriorityQueue.inner[0].NextExecutionTime
	}
}

func (taskManager *TaskManager) getAvaliableTasks() []*Task {
	now := time.Now()
	taskManager.TaskPriorityQueue.Lock()
	defer taskManager.TaskPriorityQueue.Unlock()

	ret := make([]*Task, 1)
	for len(taskManager.TaskPriorityQueue.inner) > 0 {
		if taskManager.TaskPriorityQueue.inner[0].NextExecutionTime.After(now) {
			task := taskManager.TaskPriorityQueue.inner.Pop()
			ret = append(ret, task.(*Task))
		} else {
			break
		}
	}

	return ret
}

func (*TaskManager) GetTaskLog(id int) (*TaskLog, error) {
	return nil, nil
}

func (taskManager *TaskManager) AddPeriodicTask(crontab string, identifier string, context interface{}, handler TaskHandler) {
}

func (taskManager *TaskManager) AddOneshotTask(at time.Time, identifier string, task_source int, context interface{}, handler TaskHandler) error {
	if at.After(time.Now()) {
		return fmt.Errorf("can't add task later than now")
	}

	task := &Task{
		UserIdentifier:    identifier,
		Type:              TASK_TYPE_ONESHOT,
		TaskSource:        task_source,
		NextExecutionTime: at,
		Context:           context,
		LastExecutionTime: time.Unix(0, 0),
		CreateTime:        time.Now(),
		UpdateTime:        time.Now(),
	}

	if err := taskManager.saveTaskToDB(task); err != nil {
		return fmt.Errorf("save task to db error : %v", err)
	}

	if err := taskManager.internalAddTask(task); err != nil {
		return fmt.Errorf("add internal task error: %v", err)
	}

	return nil
}

func (taskManager *TaskManager) runTasks([]*Task) {
	//runTask not realy run task
	//instead, it will just move task from pending queue to running queue
	//
}

func (taskManager *TaskManager) Run() {
	for {
		now := time.Now()
		duration := now.Sub(taskManager.getNextWakeupTime())

		select {
		case <-time.After(duration):
			tasks := taskManager.getAvaliableTasks()
			go taskManager.runTasks(tasks)
		case <-taskManager.stop:
			return
		case <-taskManager.wake:
			continue
		}
	}
}

func (taskManager *TaskManager) Stop() {
	taskManager.stop <- true
}
