package syscmd

import (
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

type RequestStatus string

const (
	StatusProcessing RequestStatus = "processing"
	StatusCompleted  RequestStatus = "completed"
	StatusError      RequestStatus = "error"
)

type Done chan struct{}

type TrackerRequest struct {
	Request       *Request
	Status        RequestStatus
	StatusCode    int
	ResponseHeaders http.Header
	ResponseBody  []byte
	Done          Done // A channel to signal completion
	RequestTime   time.Duration // To store the request duration
}

type Update struct {
	reqId string
	resp  *http.Response
}

type RequestTracker struct {
  mu sync.Mutex
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
			log.Printf("Tracker could not find request with ID: %s", update.reqId)
			continue
		}

		trackerReq.ResponseHeaders = update.resp.Header
		trackerReq.StatusCode = update.resp.StatusCode

		if update.resp.Body != nil {
			body, err := io.ReadAll(update.resp.Body)
			if err != nil {
				log.Printf(
					"Error reading response body for request %s: %v",
					update.reqId,
					err,
				)
				trackerReq.Status = StatusError
				continue
			}
			trackerReq.ResponseBody = body
		}

		trackerReq.Status = StatusCompleted
		log.Printf("Tracker updated state for request ID: %s", update.reqId)
	}
}

func (rt *RequestTracker) AddRequest(trackerReq *TrackerRequest) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if trackerReq.Request.ID == "" {
		log.Printf("Error: Attempted to add a TrackerRequest with no ID.")
		return
	}

	rt.requests[trackerReq.Request.ID] = trackerReq
	log.Printf("Tracker added request with ID: %s", trackerReq.Request.ID)
}

func (rt *RequestTracker) MarkCompleted(id string, resp *http.Response) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	trackerReq, ok := rt.requests[id]
	if !ok {
		log.Printf("Failed to find request with ID: %s to mark as completed.", id)
		return
	}

	trackerReq.ResponseHeaders = resp.Header
	trackerReq.StatusCode = resp.StatusCode

	if resp.Body != nil {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Error reading response body for request %s: %v", id, err)
			trackerReq.Status = StatusError
			return
		}
		trackerReq.ResponseBody = body
	}

	trackerReq.Status = StatusCompleted
	log.Printf(
		"Completed request with ID: %s and status code: %d",
		id,
		trackerReq.StatusCode,
	)
}

// Example of how the worker would use this
func processRequest(tracker *RequestTracker, reqID string) {
	// // ... code to execute the HTTP request ...
	//
	// // Assuming you have the response
	// resp, err := http.DefaultClient.Do(req.Http * cmd.Request)
	// if err != nil {
	// 	// handle error
	// }
	// defer resp.Body.Close()
	//
	// req.Response = resp
	// req.ResponseHeaders = resp.Header
	// req.StatusCode = resp.StatusCode
	//
	// // Mark the request as completed in the tracker
	// tracker.MarkCompleted(reqID, req)
}
