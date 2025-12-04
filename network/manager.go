package network

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shubm-quodes/repl-reqs/util"
)

type Tracker interface {
	AddRequest(*TrackerRequest)
}

type RequestManager struct {
	tracker       *RequestTracker
	client        *http.Client
	commonHeaders http.Header
	requests      map[string]*util.LRUList[string, *Request]
	drafts        map[string]*util.LRUList[string, *RequestDraft]
	mu            sync.Mutex
}

func NewRequestManager(
	tracker *RequestTracker,
	client *http.Client,
	commonHeaders http.Header,
) *RequestManager {
	if client == nil {
		client = &http.Client{}
	}
	return &RequestManager{
		tracker:       tracker,
		client:        client,
		commonHeaders: commonHeaders,
		drafts:        make(map[string]*util.LRUList[string, *RequestDraft]),
		requests:      make(map[string]*util.LRUList[string, *Request]),
	}
}

func (rm *RequestManager) AddDraftRequest(context string, draftReq *RequestDraft) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, ok := rm.drafts[context]; !ok {
		rm.drafts[context] = util.NewLRUList[string, *RequestDraft]()
	}
	rm.drafts[context].AddOrTouch(draftReq)
}

func (rm *RequestManager) AddRequest(context string, req *http.Request) (string, error) {
	reqID := uuid.NewString()
	newRequest := &Request{
		ID:          reqID,
		HttpRequest: req,
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, ok := rm.requests[context]; !ok {
		rm.requests[context] = util.NewLRUList[string, *Request]()
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

func (rm *RequestManager) GetRequestDrafts(context string) []*RequestDraft {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if lru, ok := rm.drafts[context]; ok {
		return lru.GetAll()
	}
	return nil
}

func (rm *RequestManager) PeakRequestDraft(context string) *RequestDraft {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if lru, ok := rm.drafts[context]; ok {
		draft, _ := lru.GetAt(0)
		return draft
	}

	return nil
}

func (rm *RequestManager) GetRequests(context string) ([]*Request, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if lru, ok := rm.requests[context]; ok {
		return lru.GetAll(), nil
	}
	return nil, nil
}

func (rm *RequestManager) MakeRequest(req *http.Request) (string, <-chan Update, error) {
	reqID := uuid.New().String()
	done := make(Done)
	rm.copyCommonHeaders(req)

	trackerReq := &TrackerRequest{
		Request: &Request{
			ID:          reqID,
			HttpRequest: req,
		},
		Status: StatusProcessing,
		Done:   done,
	}

	rm.tracker.AddRequest(trackerReq)
	update := Update{
		reqId: reqID,
	}

	updateChan := make(chan Update)
	go func(rm *RequestManager, id string, r *http.Request) {
		defer close(done)

		start := time.Now()
		resp, err := rm.client.Do(r)
		if err != nil {
			update.err = err
			trackerReq.Status = StatusError
		}

		requestTime := time.Since(start)

		update.resp = resp
		rm.tracker.updates <- update
		updateChan <- update

		trackerReq.RequestTime = requestTime
	}(rm, reqID, req)

	return reqID, updateChan, nil
}

func (rm *RequestManager) copyCommonHeaders(req *http.Request) {
	for key, values := range rm.commonHeaders {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
}

func (rm *RequestManager) CycleRequests(context string) (*Request, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	lru, ok := rm.requests[context]
	if !ok || lru.Size() == 0 {
		return nil, fmt.Errorf("no requests found for context: %s", context)
	}

	requests := lru.GetAll()
	listSize := len(requests)

	if listSize <= 1 {
		return nil, nil
	}

	nextRequest := requests[listSize-1]

	lru.AddOrTouch(nextRequest)

	return nextRequest, nil
}
