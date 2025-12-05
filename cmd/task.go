package cmd

import (
	"sync"
	"time"
)

const (
	TaskStatusInitiated = "initiated"
)

type TaskStatus struct {
	ID        string
	Cmd       string
	Message   string
	Error     error
	Done      bool
	Result    any
	Output    string
	CreatedAt time.Time
}

type TaskUpdater interface {
	UpdateMessage(msg string)

	AppendOutput(output string)

	SetResult(result any)

	GetResult() any

	Fail(err error)

	Complete(result any)

	CompleteWithMessage(msg string, result any)
}

type Task struct {
	status     TaskStatus
	updateChan chan<- TaskStatus
	mu         sync.RWMutex
}

func NewTask(id, cmd string, updateChan chan<- TaskStatus) *Task {
	t := &Task{
		status: TaskStatus{
			ID:        id,
			Cmd:       cmd,
			CreatedAt: time.Now(),
		},
		updateChan: updateChan,
	}

	return t
}

func (t *Task) UpdateMessage(msg string) {
	t.mu.Lock()
	t.status.Message = msg
	t.mu.Unlock()
	t.sendUpdate()
}

func (t *Task) AppendOutput(output string) {
	t.mu.Lock()
	if t.status.Output == "" {
		t.status.Output = output
	} else {
		t.status.Output += "\n" + output
	}
	msg := t.status.Message
	t.mu.Unlock()

	if msg != "" {
		t.sendUpdate()
	}
}

func (t *Task) SetResult(result any) {
	t.mu.Lock()
	t.status.Result = result
	t.mu.Unlock()
	// Don't send update - result is set with Complete()
}

func (t *Task) Fail(err error) {
	t.mu.Lock()
	t.status.Error = err
	if t.status.Message == "" {
		t.status.Message = "Task failed"
	}
	t.mu.Unlock()
	t.sendUpdate()
}

func (t *Task) Complete(result any) {
	t.mu.Lock()
	t.status.Result = result
	t.status.Done = true
	if t.status.Message == "" {
		t.status.Message = "Task completed"
	}
	t.mu.Unlock()
	t.sendUpdate()
}

func (t *Task) CompleteWithMessage(msg string, result any) {
	t.mu.Lock()
	t.status.Message = msg
	t.status.Result = result
	t.status.Done = true
	t.mu.Unlock()
	t.sendUpdate()
}

func (t *Task) sendUpdate() {
	if t.updateChan != nil {
		t.mu.RLock()
		// Send a copy to avoid race conditions
		statusCopy := t.status
		t.mu.RUnlock()

		t.updateChan <- statusCopy
	}
}

func (t *Task) GetStatus() TaskStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status
}

func (t *Task) GetResult() any {
	return t.status.Result
}
