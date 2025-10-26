package network

import (
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/shubm-quodes/repl-reqs/log"
)

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
	StatusError      = "errored"
)

type RequestStatus string

type Done chan struct{}

type TrackerRequest struct {
	Request         *Request
	Status          RequestStatus
	StatusCode      int
	ResponseHeaders http.Header
	ResponseBody    io.ReadCloser
	Done            Done
	RequestTime     time.Duration
}

// Request is a wrapper for a http.Request, adding a unique ID.
type Request struct {
	ID          string
	HttpRequest *http.Request
}

// RequestTracker manages the status of in-flight requests.
type RequestTracker struct {
	mu       sync.Mutex
	requests map[string]*TrackerRequest
	updates  chan Update
}

func NewRequestTracker() *RequestTracker {
	rt := &RequestTracker{
		requests: make(map[string]*TrackerRequest),
		updates:  make(chan Update),
	}
	go rt.startListener()
	return rt
}

func (rt *RequestTracker) startListener() {
	for update := range rt.updates {
		trackerReq, ok := rt.requests[update.reqId]
		if !ok {
			log.Debug("Tracker could not find request with ID: %s", update.reqId)
			continue
		}

		if update.resp != nil {
			trackerReq.ResponseHeaders = update.resp.Header
			trackerReq.StatusCode = update.resp.StatusCode
		}

		if update.resp != nil && update.resp.Body != nil {
			trackerReq.ResponseBody = update.resp.Body
		}

		trackerReq.Status = StatusCompleted
		log.Debug("Tracker updated state for request ID: %s", update.reqId)
	}
}

func (rt *RequestTracker) AddRequest(trackerReq *TrackerRequest) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if trackerReq.Request.ID == "" {
		log.Debug("Error: Attempted to add a TrackerRequest with no ID.")
		return
	}

	rt.requests[trackerReq.Request.ID] = trackerReq
	log.Debug("Tracker added request with ID: %s", trackerReq.Request.ID)
}

func (r *Request) GetKey() string {
	return r.ID
}

