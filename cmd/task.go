package cmd

import (
	"time"
)

const (
	TaskStatusInitiated = "initiated"
)

type TaskStatus struct {
	id        string
	cmd       string
	message   string
	error     error
	done      bool
	result    any
	output    string
	createdAt time.Time
}

func (t TaskStatus) GetID() string {
	return t.id
}

func (t TaskStatus) GetMessage() string {
	return t.message
}

func (t TaskStatus) GetError() error {
	return t.error
}

func (t TaskStatus) IsDone() bool {
	return t.done
}

func (t TaskStatus) GetResult() any {
	return t.result
}

func (t TaskStatus) GetCreatedAt() time.Time {
	return t.createdAt
}

func (t *TaskStatus) GetOutput(out string) string {
	return t.output
}

func (t *TaskStatus) SetMessage(message string) {
	t.message = message
}

func (t *TaskStatus) SetError(err error) {
	t.error = err
}

func (t *TaskStatus) SetDone(done bool) {
	t.done = done
}

func (t *TaskStatus) SetResult(result any) {
	t.result = result
}

func (t *TaskStatus) SetCreatedAt(createdAt time.Time) {
	t.createdAt = createdAt
}

func (t *TaskStatus) SetOutput(out string) {
	t.output = out
}
