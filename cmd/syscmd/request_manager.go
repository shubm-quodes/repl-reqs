package syscmd

import (
	"net/http"
	"time"

	"github.com/google/uuid"
)

var commonHeaders = make(KeyValPair)

type Request struct {
	ID          string
	Command     *ReqCmd
	HttpRequest *http.Request
}

type RequestManager struct {
	Tracker       *RequestTracker
	Client        *http.Client
	CommonHeaders http.Header
}

func NewRequestManager(tracker *RequestTracker, commonHeaders http.Header) *RequestManager {
	return &RequestManager{
		Tracker:       tracker,
		Client:        &http.Client{Timeout: 10 * time.Second},
		CommonHeaders: commonHeaders,
	}
}

func (rm *RequestManager) MakeRequest(req *http.Request) (string, <-chan Update, error) {
	reqID := uuid.New().String()
	done := make(Done)

	trackerReq := &TrackerRequest{
		Request: &Request{
			ID:          reqID,
			HttpRequest: req,
		},
		Status: StatusProcessing,
		Done:   done,
	}

	rm.Tracker.AddRequest(trackerReq)
	update := Update{
		reqId: reqID,
	}

	updateChan := make(chan Update)
	go func(rm *RequestManager, id string, r *http.Request) {
		defer close(done)

		start := time.Now()
		resp, err := rm.Client.Do(r)
		if err != nil {
			trackerReq.Status = StatusError
			return
		}

		requestTime := time.Since(start)

		update.resp = resp
		rm.Tracker.updates <- update
		updateChan <- update

		trackerReq.RequestTime = requestTime
	}(rm, reqID, req)

	return reqID, updateChan, nil
}
