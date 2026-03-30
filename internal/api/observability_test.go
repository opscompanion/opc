package api

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/opscompanion/opc/internal/models"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestHTTPClientObservabilityEndpoints(t *testing.T) {
	client := NewHTTPClient(&models.Config{APIURL: "https://example.test", APIKey: "test"})
	client.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body string
		switch req.URL.Path {
		case "/logs/search":
			body = `{"data":[{"timestamp":"2026-03-30T12:00:00Z","trace_id":"trace1234567890","span_id":"span1234567890","severity_number":17,"severity_text":"ERROR","body":"timeout waiting for upstream","service_name":"api","resource_attributes":{"region":"us-east-1"},"log_attributes":{"request_id":"req_123"},"tenant_id":"tenant_1","api_key_type":"organization","api_key_id":"key_1","user_id":null}],"nextCursor":"cursor_1","hasMore":true}`
		case "/logs/tail":
			body = `{"data":[{"timestamp":1711800000,"trace_id":"trace_tail","span_id":"span_tail","severity_number":9,"severity_text":"INFO","body":"tail event","service_name":"worker","resource_attributes":null,"log_attributes":null,"tenant_id":"tenant_1","api_key_type":null,"api_key_id":null,"user_id":null}],"nextCursor":null,"hasMore":false}`
		case "/traces/search":
			body = `{"data":[{"timestamp":"2026-03-30T12:00:00Z","trace_id":"trace1234567890","span_id":"span1234567890","parent_span_id":"parent123","trace_state":"","span_name":"GET /health","span_kind":2,"service_name":"api","resource_attributes":{"region":"us-east-1"},"scope_name":"otel","scope_version":"1.0.0","span_attributes":{"http.method":"GET"},"duration":1530,"status_code":1,"status_message":"ok","events":[],"links":[],"tenant_id":"tenant_1","api_key_type":"organization","api_key_id":"key_1","user_id":null}],"nextCursor":"cursor_2","hasMore":false}`
		default:
			body = `{"error":"not found"}`
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewBufferString(body)),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
			Header:     make(http.Header),
		}, nil
	})

	logsResp, err := client.SearchLogs(models.ObservabilitySearchRequest{Query: "timeout", TimeRange: "1h"})
	if err != nil {
		t.Fatalf("SearchLogs: %v", err)
	}
	if len(logsResp.Data) != 1 || logsResp.NextCursor == nil || *logsResp.NextCursor != "cursor_1" || !logsResp.HasMore {
		t.Fatalf("SearchLogs() = %#v", logsResp)
	}

	tailResp, err := client.TailLogs(models.ObservabilitySearchRequest{Services: []string{"worker"}})
	if err != nil {
		t.Fatalf("TailLogs: %v", err)
	}
	if len(tailResp.Data) != 1 || tailResp.NextCursor != nil || tailResp.HasMore {
		t.Fatalf("TailLogs() = %#v", tailResp)
	}

	traceResp, err := client.SearchTraces(models.ObservabilitySearchRequest{Query: "GET /health"})
	if err != nil {
		t.Fatalf("SearchTraces: %v", err)
	}
	if len(traceResp.Data) != 1 || traceResp.Data[0].SpanName != "GET /health" {
		t.Fatalf("SearchTraces() = %#v", traceResp)
	}
}
