package cmd

import (
	"time"

	"github.com/google/uuid"
)

const (
  TaskStatusInitiated = "initiated"
)

type taskStatus struct {
	id        string
	message   string
	error     error
	done      bool
	result    any
	output    string
	createdAt time.Time
}

func NewTaskStatus(message string) *taskStatus {
	return &taskStatus{
		id:        uuid.NewString(),
		message:   message,
		createdAt: time.Now(),
	}
}

func (t taskStatus) GetID() string {
	return t.id
}

func (t taskStatus) GetMessage() string {
	return t.message
}

func (t taskStatus) GetError() error {
	return t.error
}

func (t taskStatus) IsDone() bool {
	return t.done
}

func (t taskStatus) GetResult() any {
	return t.result
}

func (t taskStatus) GetCreatedAt() time.Time {
	return t.createdAt
}

func (t *taskStatus) GetOutput(out string) string {
  return t.output
}

func (t *taskStatus) SetMessage(message string) {
	t.message = message
}

func (t *taskStatus) SetError(err error) {
	t.error = err
}

func (t *taskStatus) SetDone(done bool) {
	t.done = done
}

func (t *taskStatus) SetResult(result any) {
	t.result = result
}

func (t *taskStatus) SetCreatedAt(createdAt time.Time) {
	t.createdAt = createdAt
}

func (t *taskStatus) SetOutput(out string) {
  t.output = out
}
