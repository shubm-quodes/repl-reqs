package network

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Tracker interface {
	AddRequest(reqID, status string)
	GetStatus(reqID string) (string, bool)
}

type RequestManager struct {
	tracker       Tracker
	client        *http.Client
	commonHeaders http.Header
	requests      map[string]*LRUList
	mu            sync.Mutex
}

func NewRequestManager(
	tracker Tracker,
	client *http.Client,
	commonHeaders http.Header,
) *RequestManager {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &RequestManager{
		tracker:       tracker,
		client:        client,
		commonHeaders: commonHeaders,
		requests:      make(map[string]*LRUList),
	}
}

func (rm *RequestManager) AddRequest(context string, req *http.Request) (string, error) {
	reqID := uuid.New().String()
	newRequest := &Request{
		ID:          reqID,
		HttpRequest: req,
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, ok := rm.requests[context]; !ok {
		rm.requests[context] = NewLRUList()
	}
	rm.requests[context].AddOrTouch(newRequest)

	return reqID, nil
}

func (rm *RequestManager) GetRequest(context string, index int) (*Request, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if lru, ok := rm.requests[context]; ok {
		return lru.GetAt(index)
	}
	return nil, nil
}

func (rm *RequestManager) GetRequests(context string) ([]*Request, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if lru, ok := rm.requests[context]; ok {
		return lru.GetAll(), nil
	}
	return nil, nil
}

func (rm *RequestManager) MakeRequest(reqID string, req *http.Request) error {
	for key, values := range rm.commonHeaders {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	rm.tracker.AddRequest(reqID, StatusProcessing)

	go func(reqID string, r *http.Request) {
		resp, err := rm.client.Do(r)
		if err != nil {
			rm.tracker.AddRequest(reqID, StatusFailed)
			return
		}
		defer resp.Body.Close()

		rm.tracker.AddRequest(reqID, StatusCompleted)
	}(reqID, req)

	return nil
}

func (rm *RequestManager) CycleRequests(context string) (*Request, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	lru, ok := rm.requests[context]
	if !ok || lru.Size() == 0 {
		return nil, fmt.Errorf("no requests found for context: %s", context)
	}

	requests := lru.GetAll()
	if len(requests) <= 1 {
		return nil, nil 
	}

	currentIndex := 0
	
	nextIndex := (currentIndex + 1) % len(requests)
	nextRequest := requests[nextIndex]

	lru.AddOrTouch(nextRequest)

	return nextRequest, nil
}
