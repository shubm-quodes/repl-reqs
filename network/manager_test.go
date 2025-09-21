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
	testCases := []struct {
		name               string
		initialOrder       []string // The order requests are added
		cycles             int
		expectedFinalOrder []string
	}{
		{
			name:               "First cycle",
			initialOrder:       []string{"req1", "req2", "req3"},
			cycles:             1,
			expectedFinalOrder: []string{"req2", "req1", "req3"},
		},
		{
			name:               "Second cycle",
			initialOrder:       []string{"req1", "req2", "req3"},
			cycles:             2,
			expectedFinalOrder: []string{"req3", "req2", "req1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tracker := &mockRequestTracker{}
			rm := NewRequestManager(tracker, nil, nil)
			context := "test-context"

			for _, name := range tc.initialOrder {
				rm.AddRequest(context, createTestRequest(t, http.MethodGet, "/" + name))
			}

			var lastCycledRequest *Request
			var err error
			for i := 0; i < tc.cycles; i++ {
				lastCycledRequest, err = rm.CycleRequests(context)
				if err != nil {
					t.Fatalf("Unexpected error during cycle: %v", err)
				}
			}

			expectedReturnedRequestName := tc.expectedFinalOrder[0]
			if lastCycledRequest.HttpRequest.URL.Path != "/" + expectedReturnedRequestName {
				t.Errorf("Expected returned request to be %s, but got %s", expectedReturnedRequestName, lastCycledRequest.HttpRequest.URL.Path)
			}

			requests, _ := rm.GetRequests(context)
			if len(requests) != len(tc.expectedFinalOrder) {
				t.Fatalf("Expected %d requests, got %d", len(tc.expectedFinalOrder), len(requests))
			}

			for i, expectedName := range tc.expectedFinalOrder {
				if requests[i].HttpRequest.URL.Path != "/" + expectedName {
					t.Errorf("Expected position %d to be %s, but got %s", i, expectedName, requests[i].HttpRequest.URL.Path)
				}
			}
		})
	}
}

func TestCycleRequestsWithEdgeCases(t *testing.T) {
	tracker := &mockRequestTracker{}
	rm := NewRequestManager(tracker, nil, nil)

	
	t.Run("No Requests", func(t *testing.T) {
		_, err := rm.CycleRequests("empty-context")
		if err == nil {
			t.Errorf("Expected an error for an empty context, but got nil")
		}
	})

	
	t.Run("Single Request", func(t *testing.T) {
		context := "single-context"
		rm.AddRequest(context, createTestRequest(t, http.MethodGet, "/single"))
		
		req, err := rm.CycleRequests(context)
		if err == nil {
			t.Errorf("Expected an error for a single request, but got nil")
		}
		if req != nil {
			t.Errorf("Expected nil request for a single request, but got non-nil")
		}

		requests, _ := rm.GetRequests(context)
		if len(requests) != 1 {
			t.Fatalf("Expected 1 request, got %d", len(requests))
		}
		if requests[0].HttpRequest.URL.Path != "/single" {
			t.Errorf("Expected request to remain the same, but it changed")
		}
	})
}
