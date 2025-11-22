package network

import (
	"net/http"
	"testing"
)

type mockRequestTracker struct{}

func (m *mockRequestTracker) AddRequest(reqID, status string) {

}

func (m *mockRequestTracker) GetStatus(reqID string) (string, bool) {
	return "", false
}

func createTestRequest(t *testing.T, method, url string) *http.Request {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("Failed to create test request: %v", err)
	}
	return req
}

func TestCycleRequests(t *testing.T) {

}

func TestCycleRequestsWithEdgeCases(t *testing.T) {
	// tracker := &mockRequestTracker{}
	reqMgr := NewRequestManager(&RequestTracker{}, nil, nil)

	t.Run("No Requests", func(t *testing.T) {
		_, err := reqMgr.CycleRequests("empty-context")
		if err == nil {
			t.Errorf("Expected an error for an empty context, but got nil")
		}
	})

	t.Run("Single Request", func(t *testing.T) {
		context := "single-context"
		reqMgr.AddRequest(context, createTestRequest(t, http.MethodGet, "/single"))

		req, err := reqMgr.CycleRequests(context)
		if err != nil {
			t.Errorf("Got an error for a single request")
		}
		if req != nil {
			t.Errorf("Expected nil request for a single request, but got non-nil")
		}

		requests, _ := reqMgr.GetRequests(context)
		if len(requests) != 1 {
			t.Fatalf("Expected 1 request, got %d", len(requests))
		}
		if requests[0].HttpRequest.URL.Path != "/single" {
			t.Errorf("Expected request to remain the same, but it changed")
		}
	})

}

func TestCycleRequestsWithRoundRobin(t *testing.T) {
	context := "rr-context"
	reqMgr := NewRequestManager(&RequestTracker{}, nil, nil)

	// 1. Add Requests: They are added C, B, A to the LRU list,
	// where A is the most recently added/touched (MRU).
	reqMgr.AddRequest(context, createTestRequest(t, http.MethodGet, "/req-A"))
	reqMgr.AddRequest(context, createTestRequest(t, http.MethodGet, "/req-B"))
	reqMgr.AddRequest(context, createTestRequest(t, http.MethodGet, "/req-C"))

	// Assuming GetAll() returns [C, B, A] (LRU to MRU) initially:
	// - Cycle 1: Gets C (LRU). C is moved to MRU. New order: [B, A, C]. Returns C.
	// - Cycle 2: Gets B (LRU). B is moved to MRU. New order: [A, C, B]. Returns B.
	// - Cycle 3: Gets A (LRU). A is moved to MRU. New order: [C, B, A]. Returns A.
	// - Cycle 4: Gets C (LRU). Cycle restarts.

	expectedCyclePaths := []string{"/req-A", "/req-B", "/req-C", "/req-A", "/req-B", "/req-C"}

	for i, expectedPath := range expectedCyclePaths {
		req, err := reqMgr.CycleRequests(context)
		if err != nil {
			t.Fatalf("Cycle %d: Unexpected error cycling requests: %v", i+1, err)
		}
		if req == nil {
			t.Fatalf("Cycle %d: Expected a request, but got nil", i+1)
		}

		if req.HttpRequest.URL.Path != expectedPath {
			t.Errorf(
				"Cycle %d: Expected path %s, but got %s",
				i+1,
				expectedPath,
				req.HttpRequest.URL.Path,
			)
		}
	}
}
