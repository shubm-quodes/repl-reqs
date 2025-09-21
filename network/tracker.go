package network

import (
	"container/list"
	"net/http"
	"sync"
)

// Request is a wrapper for a http.Request, adding a unique ID.
type Request struct {
	ID          string
	HttpRequest *http.Request
}

// RequestTracker manages the status of in-flight requests.
type RequestTracker struct {
	mu      sync.Mutex
	status  map[string]string // Maps request ID to its status
	updates chan Update
}

// NewRequestTracker creates a new instance of RequestTracker.
func NewRequestTracker() *RequestTracker {
	return &RequestTracker{
		status:  make(map[string]string),
		updates: make(chan Update),
	}
}

// AddRequest adds a request to the tracker.
func (t *RequestTracker) AddRequest(reqID string, status string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status[reqID] = status
}

// GetStatus returns the current status of a request.
func (t *RequestTracker) GetStatus(reqID string) (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	status, ok := t.status[reqID]
	return status, ok
}

// Update represents a status change for a request.
type Update struct {
	ReqID string
	Resp  *http.Response
	Err   error
}

// Status constants for the tracker.
const (
	StatusPending     = "pending"
	StatusProcessing  = "processing"
	StatusCompleted   = "completed"
	StatusFailed      = "failed"
)

// LRUList manages the order of requests using a combination of a linked list and a map.
type LRUList struct {
	mu    sync.Mutex
	ll    *list.List
	items map[string]*list.Element
}

// NewLRUList creates a new LRUList instance.
func NewLRUList() *LRUList {
	return &LRUList{
		ll:    list.New(),
		items: make(map[string]*list.Element),
	}
}

// AddOrTouch adds a new request to the list or moves an existing one to the front.
func (l *LRUList) AddOrTouch(req *Request) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if elem, ok := l.items[req.ID]; ok {
		// Move existing element to the front
		l.ll.MoveToFront(elem)
		return
	}

	// Add new element to the front
	elem := l.ll.PushFront(req)
	l.items[req.ID] = elem
}

// GetAt gets a request at a specific index and moves it to the front.
func (l *LRUList) GetAt(index int) (*Request, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if index < 0 || index >= l.ll.Len() {
		return nil, nil // Or an error
	}

	elem := l.ll.Front()
	for i := 0; i < index; i++ {
		elem = elem.Next()
	}

	// Move the accessed element to the front
	l.ll.MoveToFront(elem)
	return elem.Value.(*Request), nil
}

// GetAll returns all requests in the current LRU order.
func (l *LRUList) GetAll() []*Request {
	l.mu.Lock()
	defer l.mu.Unlock()

	requests := make([]*Request, 0, l.ll.Len())
	for elem := l.ll.Front(); elem != nil; elem = elem.Next() {
		requests = append(requests, elem.Value.(*Request))
	}
	return requests
}

// Size returns the number of requests in the list.
func (l *LRUList) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.ll.Len()
}

// Remove removes a request from the list.
func (l *LRUList) Remove(reqID string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if elem, ok := l.items[reqID]; ok {
		l.ll.Remove(elem)
		delete(l.items, reqID)
	}
}
