package network

import (
	"bytes"
	"fmt"
	"io"
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
	tracker          *RequestTracker
	client           *http.Client
	commonHeaders    http.Header
	requests         map[string]*util.LRUList[string, *Request]
	drafts           map[string]*util.LRUList[string, *RequestDraft]
	lastReceivedResp *http.Response
	mu               sync.Mutex
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

func (rm *RequestManager) GetRequests(context string) ([]*Request, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if lru, ok := rm.requests[context]; ok {
		return lru.GetAll(), nil
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

func (rm *RequestManager) PeakTrackerRequest(context string) (*TrackerRequest, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	contextList, ok := rm.requests[context]
	if !ok {
		return nil, fmt.Errorf("context '%s' not found", context)
	}

	lastRequest, _ := contextList.GetAt(0)
	if lastRequest == nil {
		return nil, fmt.Errorf("no requests found in context '%s'", context)
	}
	tr, ok := rm.tracker.requests[lastRequest.ID]
	if !ok {
		return nil, fmt.Errorf("request id '%s' does not exist in tracker", lastRequest.ID)
	}
	return tr, nil
}

func (rm *RequestManager) MakeRequest(req *http.Request) (string, <-chan Update, error) {
	return rm.makeRequest("", req, false)
}

func (rm *RequestManager) MakeRequestWithContext(
	context string,
	req *http.Request,
) (string, <-chan Update, error) {
	return rm.makeRequest(context, req, true)
}

func (rm *RequestManager) makeRequest(
	context string,
	req *http.Request,
	trackInContext bool,
) (string, <-chan Update, error) {
	reqID := uuid.New().String()
	rm.copyCommonHeaders(req)

	trackerReq := rm.createTrackerRequest(reqID, req)
	rm.tracker.AddRequest(trackerReq)

	if trackInContext {
		// Discard old buffered response if it exists
		rm.discardOldBufferedResponse(context)
		rm.addToContext(context, trackerReq.Request)
	}

	updateChan := make(chan Update)
	go rm.executeRequest(trackerReq, req, reqID, updateChan, trackInContext)

	return reqID, updateChan, nil
}

func (rm *RequestManager) createTrackerRequest(reqID string, req *http.Request) *TrackerRequest {
	return &TrackerRequest{
		Request: &Request{
			ID:          reqID,
			HttpRequest: req,
		},
		Status: StatusProcessing,
		Done:   make(Done),
	}
}

func (rm *RequestManager) addToContext(context string, request *Request) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, ok := rm.requests[context]; !ok {
		rm.requests[context] = util.NewLRUList[string, *Request]()
	}
	rm.requests[context].AddOrTouch(request)
}

func (rm *RequestManager) discardOldBufferedResponse(context string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	contextList, ok := rm.requests[context]
	if !ok {
		return
	}

	lastRequest, _ := contextList.GetAt(0)
	if lastRequest == nil {
		return
	}

	// Get the tracker request and close its buffered body
	if trackerReq, ok := rm.tracker.requests[lastRequest.ID]; ok {
		if trackerReq.ResponseBody != nil {
			trackerReq.ResponseBody.Close()
			trackerReq.ResponseBody = nil
		}
	}
}

func (rm *RequestManager) executeRequest(
	trackerReq *TrackerRequest,
	req *http.Request,
	reqID string,
	updateChan chan Update,
	bufferBody bool,
) {
	defer close(trackerReq.Done)

	start := time.Now()
	resp, err := rm.client.Do(req)

	// Buffer response body ONLY for tracked context requests
	if bufferBody && err == nil && resp != nil && resp.Body != nil {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		if readErr != nil {
			err = fmt.Errorf("failed to buffer response body: %w", readErr)
		} else {
			trackerReq.ResponseBody = io.NopCloser(bytes.NewReader(bodyBytes))
			resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
	}

	trackerReq.Status = rm.determineStatus(err)
	trackerReq.RequestTime = time.Since(start)

	update := Update{
		reqId: reqID,
		resp:  resp,
		err:   err,
	}

	rm.lastReceivedResp = resp
	rm.tracker.updates <- update
	updateChan <- update
}

func (rm *RequestManager) bufferResponseBody(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		return fmt.Errorf("failed to read response body: %w", err)
	}
	resp.Body.Close()

	resp.Body = io.NopCloser(bytes.NewReader(body))
	return nil
}

func (rm *RequestManager) determineStatus(err error) RequestStatus {
	if err != nil {
		return StatusError
	}
	return StatusCompleted
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
