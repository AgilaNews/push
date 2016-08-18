package task

import (
	"container/heap"
	"fmt"
	"sync"
	"time"

	"github.com/alecthomas/log4go"
	"github.com/jinzhu/gorm"
)

const (
	TASK_TYPE_PERIODIC = TaskType(1)
	TASK_TYPE_ONESHOT  = TaskType(2)

	TASK_SOURCE_PUSH = TaskSource(0)

	STATUS_INIT    = TaskStatus(0) //init but may editing
	STATUS_PENDING = TaskStatus(1) //added to pending Queue
	STATUS_EXEC    = TaskStatus(2)
	STATUS_SUCC    = TaskStatus(3)
	STATUS_FAIL    = TaskStatus(4)
	STATUS_CANCEL  = TaskStatus(5)

	TICK = time.Minute
)

type TaskSource int
type TaskStatus int
type TaskType int

type TaskHandler interface {
	DoTask(identifier string, context interface{}) error
	Sync(uid string) (interface{}, error)
}

type TaskKey struct {
	Source TaskSource
	Uid    string
}

type Task struct {
	ID         uint      `gorm:"column:id;primary_key"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
	CanceledAt time.Time `gorm:"column:canceled_at"`

	UserIdentifier    string     `gorm:"column:uid;type:varchar(32);not null;index"`
	Type              TaskType   `gorm:"column:type;type:tinyint(4)"`
	Source            TaskSource `gorm:"column:source;type:tinyint(4)"`
	Period            int        `gorm:"column:period;type:int(11)"`
	LastExecutionTime time.Time  `gorm:"column:last_execution_time"`
	NextExecutionTime time.Time  `gorm:"column:next_execution_time"`
	Status            TaskStatus `gorm:"column:status;type:tinyint(4);index"`

	Click     int     `gorm:"column:click"`
	Reach     int     `gorm:"column:reach"`
	ClickRate float32 `gorm:"column:click_rate"`

	Retry         int `gorm:"-"`
	RetryInterval int `gorm:"-"`
	Timeout       int `gorm:"-"`

	Handler TaskHandler `gorm:"-" json:"-"`
	Context interface{} `gorm:"-"`
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
		inner map[TaskKey]*Task
	}
	PendingQueue struct {
		sync.RWMutex
		inner PriorityQueue
	}

	stop chan bool
	wake chan bool

	wdb *gorm.DB
	rdb *gorm.DB

	handlers map[TaskSource]TaskHandler
}

type PriorityQueue []*Task

var (
	GlobalTaskManager *TaskManager
)

func (Task) TableName() string {
	return "tb_task"
}

func (t *Task) Equal(other *Task) bool {
	return t.UserIdentifier == other.UserIdentifier && t.Source == other.Source
}

func NewTaskManager(rdb, wdb *gorm.DB) (*TaskManager, error) {
	m := &TaskManager{
		TaskMap: struct {
			sync.RWMutex
			inner map[TaskKey]*Task
		}{
			inner: make(map[TaskKey]*Task),
		},
		PendingQueue: struct {
			sync.RWMutex
			inner PriorityQueue
		}{
			inner: make(PriorityQueue, 0),
		},

		stop: make(chan bool),
		wake: make(chan bool),
		wdb:  wdb,
		rdb:  rdb,

		handlers: make(map[TaskSource]TaskHandler),
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

func (taskManager *TaskManager) RegisterTaskSourceHandler(source TaskSource, handler TaskHandler) {
	taskManager.handlers[source] = handler
}

func (taskManager *TaskManager) internalRemoveTask(task *Task) error {
	var ok bool
	key := TaskKey{
		Source: task.Source,
		Uid:    task.UserIdentifier,
	}

	taskManager.TaskMap.RLock()
	_, ok = taskManager.TaskMap.inner[key]
	taskManager.TaskMap.RUnlock()

	if !ok {
		return fmt.Errorf("task not exists: %v", key)
	}

	taskManager.TaskMap.Lock()
	_, ok = taskManager.TaskMap.inner[key]
	if !ok {
		taskManager.TaskMap.Unlock()
		return fmt.Errorf("tasks not exists: %v", key)
	}

	delete(taskManager.TaskMap.inner, key)
	taskManager.TaskMap.Unlock()

	return nil
}

func (taskManager *TaskManager) internalAddTask(task *Task) error {
	var ok bool
	key := TaskKey{
		Source: task.Source,
		Uid:    task.UserIdentifier,
	}

	taskManager.TaskMap.RLock()
	_, ok = taskManager.TaskMap.inner[key]
	taskManager.TaskMap.RUnlock()

	if ok {
		return fmt.Errorf("task exists")
	}

	taskManager.TaskMap.Lock()
	_, ok = taskManager.TaskMap.inner[key]
	if ok {
		taskManager.TaskMap.Unlock()
		return fmt.Errorf("tasks exists")
	}

	taskManager.TaskMap.inner[key] = task
	taskManager.TaskMap.Unlock()

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

func (taskManager *TaskManager) NewOneshotTask(at time.Time,
	identifier string,
	source TaskSource,
	retry, retryInterval int,
	context interface{}) *Task {
	if _, ok := taskManager.handlers[source]; !ok {
		panic("please register your type first")
	}

	return &Task{
		UserIdentifier:    identifier,
		Type:              TASK_TYPE_ONESHOT,
		Source:            source,
		NextExecutionTime: at,
		Context:           context,
		Retry:             retry,
		RetryInterval:     retryInterval,
		LastExecutionTime: time.Time{},
		Handler:           taskManager.handlers[source],
	}
}

func (taskManager *TaskManager) addTaskToPendingQueue(task *Task) {
	taskManager.updateTaskStatus(task, STATUS_PENDING)
	taskManager.PendingQueue.Lock()
	defer taskManager.PendingQueue.Unlock()
	heap.Push(&taskManager.PendingQueue.inner, task)
	select {
	case taskManager.wake <- true:
	default:
	}
}

func (taskManager *TaskManager) CancelTask(uid string, source TaskSource) error {
	task := &Task{}

	if err := taskManager.rdb.Where("uid = ? and source = ?", uid, source).First(task).Error; err != nil {
		return err
	}

	if err := taskManager.saveCancelTask(task); err != nil {
		return err
	}

	taskManager.PendingQueue.Lock()
	for idx, iter := range taskManager.PendingQueue.inner {
		if task.Equal(iter) {
			//remove element
			taskManager.PendingQueue.inner = append(taskManager.PendingQueue.inner[:idx], taskManager.PendingQueue.inner[idx+1:]...)
			break
		}
	}
	taskManager.PendingQueue.Unlock()

	if err := taskManager.internalRemoveTask(task); err != nil {
		return err
	}

	select {
	case taskManager.wake <- true:
	default:
	}
	return nil
}

func (taskManager *TaskManager) GetTasks(pn, ps int) ([]*Task, int) {
	taskManager.PendingQueue.RLock()
	defer taskManager.PendingQueue.RUnlock()

	var tmp []*Task

	offset := pn * ps
	if offset < len(taskManager.PendingQueue.inner) {
		if offset+pn >= len(taskManager.PendingQueue.inner) {
			tmp = taskManager.PendingQueue.inner[offset:]
		} else {
			tmp = taskManager.PendingQueue.inner[offset : offset+pn]
		}
	}

	ret := make([]*Task, len(tmp))

	for idx, t := range tmp {
		task := *t
		ret[idx] = &task
	}

	return ret, len(taskManager.PendingQueue.inner)/pn + 1
}

func (taskManager *TaskManager) AddAndScheduleTask(task *Task) error {
	now := time.Now()
	if task.NextExecutionTime.Before(now) {
		return fmt.Errorf("can't add task than now: %v < %v", task.NextExecutionTime, now)
	}

	task.Status = STATUS_INIT
	if err := taskManager.saveTaskToDB(task); err != nil {
		return fmt.Errorf("save task to db error : %v", err)
	}

	if err := taskManager.internalAddTask(task); err != nil {
		return fmt.Errorf("add internal task error: %v", err)
	}

	log4go.Info("new task %v added type:%v next execaution time %s", task.UserIdentifier, task.Type, task.NextExecutionTime)
	taskManager.addTaskToPendingQueue(task)
	return nil
}

func (taskManager *TaskManager) doneTask(task *Task, status TaskStatus) {
	key := TaskKey{
		Uid:    task.UserIdentifier,
		Source: task.Source,
	}

	switch task.Type {
	case TASK_TYPE_ONESHOT:
		switch status {
		case STATUS_SUCC:
			taskManager.saveSuccessTask(task)
			fallthrough
		case STATUS_FAIL:
			taskManager.updateTaskStatus(task, STATUS_FAIL)
			taskManager.TaskMap.Lock()
			delete(taskManager.TaskMap.inner, key)
			taskManager.TaskMap.Unlock()
		}
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
				err := task.Handler.DoTask(task.UserIdentifier, task.Context)
				if err != nil {
					if task.Retry > 0 {
						log4go.Global.Info("task %v-%v fails, retry (%v/%v)", task.Type, task.UserIdentifier, task.Retry, b)
						task.Retry--
						time.Sleep(time.Second * time.Duration(task.RetryInterval))
					} else {
						break
					}
				} else {
					taskManager.saveSuccessTask(task)
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
			if len(tasks) > 0 {
				log4go.Global.Debug("run tasks [%d]", len(tasks))
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

func (taskManager *TaskManager) SyncTask() error {
	tasks := []*Task{}
	if err := taskManager.rdb.Where("status in (?)", []TaskStatus{STATUS_PENDING, STATUS_EXEC, STATUS_INIT}).Find(&tasks).Error; err != nil {
		return err
	}

	for _, task := range tasks {
		var context interface{}
		var err error

		if _, ok := taskManager.handlers[task.Source]; !ok {
			log4go.Warn("unknown task source :%v", task.Source)
			continue
		} else {
			task.Handler = taskManager.handlers[task.Source]
			context, err = task.Handler.Sync(task.UserIdentifier)

			if err != nil {
				log4go.Warn("task context sync error: %v", err)
				continue
			} else {
				task.Context = context
			}
		}

		now := time.Now()
		if task.NextExecutionTime.Before(now) {
			log4go.Warn("next execution time is to early, just set it to failure")
			taskManager.updateTaskStatus(task, STATUS_FAIL)
		} else {

			taskManager.addTaskToPendingQueue(task)
			log4go.Warn("schedule task : [%v]", task.UserIdentifier)
		}
	}

	return nil
}

func (taskManager *TaskManager) updateTaskStatus(task *Task, status TaskStatus) error {
	if err := taskManager.wdb.Model(task).Update("status", status).Error; err != nil {
		return fmt.Errorf("update taks error : %v", status)
	}

	log4go.Info("update task [%v] status [%v] ", task.UserIdentifier, status)

	return nil
}

func (taskManager *TaskManager) saveSuccessTask(task *Task) error {
	log4go.Info("update task [%v] status SUCCESS", task.UserIdentifier)

	task.LastExecutionTime = time.Now()
	if err := taskManager.wdb.Model(task).Update(
		map[string]interface{}{
			"status":              STATUS_SUCC,
			"last_execution_time": task.LastExecutionTime}).Error; err != nil {
		return fmt.Errorf("update delivery time and status error")
	}
	task.Status = STATUS_SUCC

	return nil
}

func (taskManager *TaskManager) saveCancelTask(task *Task) error {
	log4go.Info("update task [%v] status canceld", task.UserIdentifier)

	task.CanceledAt = time.Now()

	if err := taskManager.wdb.Model(task).Update(
		map[string]interface{}{
			"status":      STATUS_CANCEL,
			"canceled_at": task.CanceledAt}).Error; err != nil {
		return fmt.Errorf("update canceld time and status error")
	}

	task.Status = STATUS_CANCEL

	return nil
}

func (taskManager *TaskManager) saveTaskLog(tasklog *TaskLog) {
	panic("error")
}

func (taskManager *TaskManager) saveTaskToDB(task *Task) error {
	var err error

	if err = taskManager.wdb.Create(task).Error; err != nil {
		return err
	}
	log4go.Info("saved task %d to db", task.ID)

	return nil
}
