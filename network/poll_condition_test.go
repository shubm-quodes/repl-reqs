package network

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"
)

func mockResponse(
	status int,
	body string,
	contentType string,
	headers map[string]string,
) *http.Response {
	resp := &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
	resp.Header.Set("Content-Type", contentType)
	for k, v := range headers {
		resp.Header.Set(k, v)
	}
	return resp
}

func TestNewCondition(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{"Valid Status", "$status=200", "*network.StatusCondition", false},
		{
			"Valid Header",
			"$header.Content-Type=application/json",
			"*network.HeaderCondition",
			false,
		},
		{"Valid Body", "$body.user.id=123", "*network.BodyCondition", false},
		{"Invalid Format", "status=200", "", true},
		{"Unknown Type", "$footer=none", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewCondition(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCondition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && fmt.Sprintf("%T", got) != tt.want {
				t.Errorf("NewCondition() got type = %T, want %s", got, tt.want)
			}
		})
	}
}

func TestStatusCondition_Evaluate(t *testing.T) {
	cond := &StatusCondition{Expected: "200"}

	t.Run("Match", func(t *testing.T) {
		resp := mockResponse(200, "", "", nil)
		if !cond.Evaluate(resp) {
			t.Errorf("Expected status 200 to evaluate to true")
		}
	})

	t.Run("Mismatch", func(t *testing.T) {
		resp := mockResponse(404, "", "", nil)
		if cond.Evaluate(resp) {
			t.Errorf("Expected status 404 to evaluate to false")
		}
	})
}

func TestHeaderCondition_Evaluate(t *testing.T) {
	cond := &HeaderCondition{Key: ".X-Test-ID", Expected: "ABC-123"}

	t.Run("Match Header", func(t *testing.T) {
		resp := mockResponse(200, "", "", map[string]string{"X-Test-ID": "ABC-123"})
		if !cond.Evaluate(resp) {
			t.Error("Expected header match to be true")
		}
	})

	t.Run("Missing Header", func(t *testing.T) {
		resp := mockResponse(200, "", "", nil)
		if cond.Evaluate(resp) {
			t.Error("Expected missing header to be false")
		}
	})
}

func TestBodyCondition_Evaluate(t *testing.T) {
	cond := &BodyCondition{Path: "data.id", Expected: "500"}

	t.Run("JSON Match", func(t *testing.T) {
		jsonBody := `{"data": {"id": 500}}`
		resp := mockResponse(200, jsonBody, "application/json", nil)

		if !cond.Evaluate(resp) {
			t.Fatalf("Body condition failed")
		}
	})

	t.Run("Unsupported Content Type", func(t *testing.T) {
		resp := mockResponse(200, "plain text", "text/plain", nil)
		if cond.Evaluate(resp) {
			t.Error("Expected false for unsupported content type")
		}
	})
}
