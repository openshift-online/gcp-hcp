package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestWriteError(t *testing.T) {
	tests := []struct {
		name       string
		code       int
		message    string
		wantStatus string
	}{
		{
			name:       "not found",
			code:       http.StatusNotFound,
			message:    "resource not found",
			wantStatus: "Failure",
		},
		{
			name:       "bad request",
			code:       http.StatusBadRequest,
			message:    "invalid input",
			wantStatus: "Failure",
		},
		{
			name:       "internal server error",
			code:       http.StatusInternalServerError,
			message:    "something went wrong",
			wantStatus: "Failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			writeError(rr, tt.code, tt.message)

			if rr.Code != tt.code {
				t.Errorf("status code = %d, want %d", rr.Code, tt.code)
			}

			ct := rr.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}

			var body map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to unmarshal response body: %v", err)
			}

			if body["status"] != tt.wantStatus {
				t.Errorf("status = %v, want %v", body["status"], tt.wantStatus)
			}
			if body["message"] != tt.message {
				t.Errorf("message = %v, want %v", body["message"], tt.message)
			}
			if body["kind"] != "Status" {
				t.Errorf("kind = %v, want Status", body["kind"])
			}
			if int(body["code"].(float64)) != tt.code {
				t.Errorf("code in body = %v, want %d", body["code"], tt.code)
			}
		})
	}
}

func TestSpecChanged(t *testing.T) {
	tests := []struct {
		name string
		old  map[string]interface{}
		new  map[string]interface{}
		want bool
	}{
		{
			name: "no change",
			old: map[string]interface{}{
				"spec": map[string]interface{}{"replicas": float64(3)},
			},
			new: map[string]interface{}{
				"spec": map[string]interface{}{"replicas": float64(3)},
			},
			want: false,
		},
		{
			name: "spec changed",
			old: map[string]interface{}{
				"spec": map[string]interface{}{"replicas": float64(3)},
			},
			new: map[string]interface{}{
				"spec": map[string]interface{}{"replicas": float64(5)},
			},
			want: true,
		},
		{
			name: "spec added",
			old:  map[string]interface{}{},
			new: map[string]interface{}{
				"spec": map[string]interface{}{"replicas": float64(1)},
			},
			want: true,
		},
		{
			name: "metadata change only",
			old: map[string]interface{}{
				"metadata": map[string]interface{}{"name": "old"},
				"spec":     map[string]interface{}{"replicas": float64(3)},
			},
			new: map[string]interface{}{
				"metadata": map[string]interface{}{"name": "new"},
				"spec":     map[string]interface{}{"replicas": float64(3)},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldObj := &unstructured.Unstructured{Object: tt.old}
			newObj := &unstructured.Unstructured{Object: tt.new}

			got := specChanged(oldObj, newObj)
			if got != tt.want {
				t.Errorf("specChanged() = %v, want %v", got, tt.want)
			}
		})
	}
}
