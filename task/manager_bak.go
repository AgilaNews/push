package task

import (
	"time"

	"gopkg.in/DATA-DOG/go-sqlmock.v1"
	"testing"
)

func NilHandler(identifier string, context interface{}) error {
	return nil
}

var (
	baseDate = time.Now().Add(time.Hour)
)

func TestDupTask(t *testing.T) {
	wdb, wmock, _ := sqlmock.New()
	rdb, _, _ := sqlmock.New()

	manager, _ := NewTaskManager(rdb, wdb)
	wmock.ExpectExec("INSERT INTO tb_task").WillReturnResult(sqlmock.NewResult(1, 1))
	taska := NewOneshotTask(baseDate, "a", TASK_SOURCE_PUSH, 0, 0, NilHandler, nil)

	if err := manager.AddTask(taska); err != nil {
		t.Errorf("add task should not fail: %v", err)
	}
	if err := manager.AddTask(taska); err == nil {
		t.Errorf("add task should fail : %v", err)
	}

}

func TestTaskPriority(t *testing.T) {
	wdb, wmock, _ := sqlmock.New()
	rdb, _, _ := sqlmock.New()

	manager, _ := NewTaskManager(rdb, wdb)

	now := time.Now().Add(time.Second)
	taska := NewOneshotTask(now, "a",
		TASK_SOURCE_PUSH, 0, 0, NilHandler, nil)
	taskb := NewOneshotTask(now.Add(time.Second), "b",
		TASK_SOURCE_PUSH, 0, 0, NilHandler, nil)

	wmock.ExpectExec("INSERT INTO tb_task")
	manager.AddTask(taska)
	wmock.ExpectExec("INSERT INTO tb_task")
	manager.AddTask(taskb)

	next := manager.getNextWakeupTime()
	if !next.Equal(baseDate) {
		t.Errorf("expect %v, get %v", next, now)
	}

	tasks := manager.popAvaliableTasks(now.Add(-time.Second))
	if len(tasks) != 0 {
		t.Errorf("shall not pop any task")
	}

	tasks = manager.popAvaliableTasks(baseDate)
	if len(tasks) != 1 {
		t.Errorf("we should pop one task, poped %d", len(tasks))
		return
	} else {
		if tasks[0].UserIdentifier != "a" {
			t.Errorf("should pop 'a', poped %v", tasks[0].UserIdentifier)
		}
	}

	manager.doneTask(tasks[0], STATUS_SUCC)

	next = manager.getNextWakeupTime()

	if !next.Equal(baseDate.Add(time.Second)) {
		t.Errorf("expect %v, get %v", baseDate.Add(time.Second), next)
	}
}

func testRunHandler(identifer string, context interface{}) error {
	c := context.(*struct{ Counter int })
	c.Counter++

	return nil
}

func TestRunTask(t *testing.T) {
	TestContext := struct {
		Counter int
	}{
		Counter: 0,
	}

	wdb, wmock, _ := sqlmock.New()
	rdb, _, _ := sqlmock.New()

	taskManager, _ := NewTaskManager(rdb, wdb)

	taska := NewOneshotTask(baseDate, "a",
		TASK_SOURCE_PUSH, 0, 0, NilHandler, nil)
	taskb := NewOneshotTask(baseDate.Add(time.Second), "b",
		TASK_SOURCE_PUSH, 0, 0, NilHandler, nil)

	wmock.ExpectExec("INSERT INTO tb_task")
	wmock.ExpectExec("INSERT INTO tb_task")
	taskManager.AddTask(taska)
	taskManager.AddTask(taskb)

	tasks := taskManager.popAvaliableTasks(baseDate.Add(time.Minute))

	taskManager.runTasks(tasks)

	if TestContext.Counter != 2 {
		t.Errorf("Counter %v not equal 2", TestContext.Counter)
	}
}

func testNormalRunnerHandler(identifier string, context interface{}) error {
	c := context.(*struct {
		Stop       chan bool
		ExceptTime time.Time
		T          *testing.T
	})

	now := time.Now()
	if !c.ExceptTime.After(now.Add(-time.Second)) && c.ExceptTime.Before(now.Add(time.Second)) {
		c.T.Errorf("time now equal expect %v!= now %v", c.ExceptTime, now)
	}

	c.Stop <- true
	return nil
}

func TestNormal(t *testing.T) {
	now := time.Now()

	TestContext := struct {
		Stop       chan bool  `json:"-"`
		ExceptTime time.Time  `json:"expect"`
		T          *testing.T `json:"-"`
	}{
		Stop:       make(chan bool),
		ExceptTime: now.Add(3 * time.Second),
		T:          t,
	}

	wdb, wmock, _ := sqlmock.New()
	rdb, _, _ := sqlmock.New()

	taskManager, _ := NewTaskManager(rdb, wdb)
	task := NewOneshotTask(TestContext.ExceptTime, "normal_task", TASK_SOURCE_PUSH, 0, 0, testNormalRunnerHandler, &TestContext)
	wmock.ExpectExec("INSERT INTO tb_task")

	if err := taskManager.AddTask(task); err != nil {
		t.Errorf("add task error : %v", err)
		return
	}
	go taskManager.Run()
	<-TestContext.Stop
	taskManager.Stop()
}
