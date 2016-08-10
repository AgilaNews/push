package task

import (
	"container/heap"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/alecthomas/log4go"
)

const (
	TASK_TYPE_PERIODIC = 1
	TASK_TYPE_ONESHOT  = 2

	TASK_SOURCE_PUSH = 1

	STATUS_INIT    = 0
	STATUS_PENDING = 1
	STATUS_EXEC    = 2
	STATUS_SUCC    = 3
	STATUS_FAIL    = 4

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

	Retry         int
	RetryInterval int
	Timeout       int

	Handler TaskHandler
	Context interface{}
}

type TaskLog struct {
	TaskId int
	Status int
	Start  time.Time
	End    time.Time
}

type TaskManager struct {
	TaskMap struct {
		sync.RWMutex
		inner map[string]*Task
	}
	PendingQueue struct {
		sync.RWMutex
		inner PriorityQueue
	}

	stop chan bool
	wake chan bool

	Rdb *sql.DB
	Wdb *sql.DB
}

type TaskHandler func(identifier string, context interface{}) error
type PriorityQueue []*Task

func NewTaskManager(rdb, wdb *sql.DB) (*TaskManager, error) {
	m := &TaskManager{
		TaskMap: struct {
			sync.RWMutex
			inner map[string]*Task
		}{
			inner: make(map[string]*Task),
		},
		PendingQueue: struct {
			sync.RWMutex
			inner PriorityQueue
		}{
			inner: make(PriorityQueue, 0),
		},

		stop: make(chan bool),
		wake: make(chan bool),
		Rdb:  rdb,
		Wdb:  wdb,
	}

	heap.Init(&m.PendingQueue.inner)

	return m, nil
}

func (q *PriorityQueue) Swap(i, j int) {
	(*q)[i], (*q)[j] = (*q)[j], (*q)[i]
}

func (q *PriorityQueue) Len() int {
	return len(*q)
}

func (q *PriorityQueue) Less(i, j int) bool {
	return (*q)[i].NextExecutionTime.Before((*q)[j].NextExecutionTime)
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

	select {
	case taskManager.wake <- true:
	default:
	}
	return nil
}

func (taskManager *TaskManager) getNextWakeupTime() time.Time {
	taskManager.PendingQueue.RLock()
	defer taskManager.PendingQueue.RUnlock()

	if taskManager.PendingQueue.inner.Len() == 0 {
		return time.Now().Add(TICK)
	} else {
		return taskManager.PendingQueue.inner[0].NextExecutionTime
	}
}

func (taskManager *TaskManager) popAvaliableTasks(deadline time.Time) []*Task {
	taskManager.PendingQueue.Lock()
	defer taskManager.PendingQueue.Unlock()

	ret := make([]*Task, 0)

	for len(taskManager.PendingQueue.inner) > 0 {
		next := taskManager.PendingQueue.inner[0].NextExecutionTime
		if next.Before(deadline) || next.Equal(deadline) {
			p := heap.Pop(&taskManager.PendingQueue.inner)
			ret = append(ret, p.(*Task))
		} else {
			break
		}
	}

	return ret
}

func (*TaskManager) GetTaskLog(id int) (*TaskLog, error) {
	return nil, nil
}

func NewOneshotTask(at time.Time,
	identifier string,
	source, retry, retryInterval int,
	handler TaskHandler,
	context interface{}) *Task {
	return &Task{
		UserIdentifier:    identifier,
		Type:              TASK_TYPE_ONESHOT,
		TaskSource:        source,
		NextExecutionTime: at,
		Context:           context,
		Retry:             retry,
		RetryInterval:     retryInterval,
		Handler:           handler,
		LastExecutionTime: time.Unix(0, 0),
		CreateTime:        time.Now(),
		UpdateTime:        time.Now(),
	}
}

func (taskManager *TaskManager) AddTask(task *Task) error {
	now := time.Now()
	if task.NextExecutionTime.Before(now) {
		return fmt.Errorf("can't add task than now: %v < %v", task.NextExecutionTime, now)
	}

	if err := taskManager.internalAddTask(task); err != nil {
		return fmt.Errorf("add internal task error: %v", err)
	}

	if err := taskManager.saveTaskToDB(task); err != nil {
		return fmt.Errorf("save task to db error : %v", err)
	}

	taskManager.PendingQueue.Lock()
	defer taskManager.PendingQueue.Unlock()
	heap.Push(&taskManager.PendingQueue.inner, task)

	log4go.Info("new task %v added type:%v next execution time %s", task.UserIdentifier, task.Type, task.NextExecutionTime)

	fmt.Println(taskManager.PendingQueue.inner)
	return nil
}

func (taskManager *TaskManager) doneTask(task *Task, status int) {
	switch task.Type {
	case TASK_TYPE_ONESHOT:
		taskManager.TaskMap.Lock()
		delete(taskManager.TaskMap.inner, task.UserIdentifier)
		taskManager.TaskMap.Unlock()

		taskManager.updateTaskStatus(task, STATUS_SUCC)
	default:
		panic("not support task type yet")
	}
}

func (taskManager *TaskManager) runTasks(tasks []*Task) {
	var wg sync.WaitGroup

	for _, task := range tasks {
		wg.Add(1)

		go func() {
			defer wg.Done()

			b := task.Retry
			for {
				taskManager.updateTaskStatus(task, STATUS_EXEC)
				err := task.Handler(task.UserIdentifier, task.Context)
				if err != nil {
					if task.Retry > 0 {
						log4go.Global.Info("task %v-%v fails, retry (%v/%v)", task.Type, task.UserIdentifier, task.Retry, b)
						task.Retry--
						time.Sleep(time.Second * time.Duration(task.RetryInterval))
					} else {
						break
					}
				} else {
					taskManager.doneTask(task, STATUS_SUCC)
					return
				}
			}

			taskManager.doneTask(task, STATUS_FAIL)
		}()
	}

	wg.Wait()
}

func (taskManager *TaskManager) Run() {
	for {
		now := time.Now()
		next := taskManager.getNextWakeupTime()
		var duration time.Duration

		if now.After(next) {
			duration = time.Duration(0)
		} else {
			duration = next.Sub(now)
		}

		log4go.Global.Debug("wait for duration %v next:%v now:%v", duration, next, now)
		select {
		case <-taskManager.stop:
			log4go.Global.Info("taskmanager closed")
			return
		case <-time.After(duration):
			tasks := taskManager.popAvaliableTasks(now)
			log4go.Global.Debug("run tasks [%d]", len(tasks))
			if len(tasks) > 0 {
				go taskManager.runTasks(tasks)
			}
		case <-taskManager.wake:
			log4go.Global.Debug("taskmanager waked")
			continue
		}
	}
}

func (taskManager *TaskManager) Stop() {
	taskManager.stop <- true
}

func (*TaskManager) syncTask() error {
	return nil
}

func (taskManager *TaskManager) updateTaskStatus(task *Task, status int) error {
	log4go.Info("update task [%v] status [%v] ", task.UserIdentifier, status)

	if _, err := taskManager.Wdb.Exec("UPDATE tb_task SET status=?", status); err != nil {
		return fmt.Errorf("update taks error : %v", status)
	}

	return nil
}

func (taskManager *TaskManager) saveTaskLog(tasklog *TaskLog) {
	_, err := taskManager.Wdb.Exec("INSERT INTO tb_task_log(`task_id`, `status`, `start_time`, `end_time`)"+
		"VALUES(?,?,?,?)",
		tasklog.TaskId,
		tasklog.Status,
		int(tasklog.Start.Unix()),
		int(tasklog.End.Unix()),
	)

	if err != nil {
		log4go.Warn("insert task log fail %v", tasklog)
	}
}

func (taskManager *TaskManager) saveTaskToDB(task *Task) error {
	var c []byte
	var err error

	if task.Context != nil {
		c, err = json.Marshal(task.Context)
		if err != nil {
			return fmt.Errorf("illegal context format (must be json-able): %v", err)
		}
	}

	result, err := taskManager.Wdb.Exec("INSERT INTO tb_task (`identifier`, `type`, `task_type`,"+
		"`source`, `period`, `context`, `last_execution_time`, `next_execution_time`, "+
		"`create_time`, `update_time`) VALUES(?,?,?,?,?,?,?,?,?,?)",
		task.UserIdentifier,
		task.Type,
		task.TaskSource,
		task.Period,
		string(c),
		int(task.LastExecutionTime.Unix()),
		int(task.NextExecutionTime.Unix()),
		int(task.CreateTime.Unix()),
		int(task.UpdateTime.Unix()),
		STATUS_INIT,
	)

	if err != nil {
		return err
	}

	lid, _ := result.LastInsertId()
	task.Id = int(lid)
	log4go.Info("saved task %d to db", task.Id)

	return nil
}
