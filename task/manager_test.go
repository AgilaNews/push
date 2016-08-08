package task

import (
	"time"

	"testing"
)

func GeneralHandler(task *Task) error {
	return nil
}

func TestDupTask(t *testing.T) {

}

func TestTaskPriority(t *testing.T) {
	a, _ := time.Parse("1970-01-01 18:00:00", "2016-08-09 18:00:00")
	b := a.Add(time.Second)

	manager, _ := NewTaskManager()
	manager.AddOneshotTask(a, "123", TASK_SOURCE_PUSH, "", GeneralHandler)
	manager.AddOneshotTask(b, "123", TASK_SOURCE_PUSH, "", GeneralHandler)

	next := manager.getNextWakeupTime()
	if !next.Equal(a) {
		t.Errorf("expect %v, get %v", a, next)
	}
}
