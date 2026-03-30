package api

import (
	"encoding/json"
	"testing"

	"github.com/opscompanion/opc/internal/models"
)

func TestDecodeUnauthorized(t *testing.T) {
	raw := json.RawMessage(`{"unauthorized":true,"requiredScopes":["a:b:read"]}`)
	got, ok := decodeUnauthorized(raw)
	if !ok {
		t.Fatal("decodeUnauthorized() should detect unauthorized payload")
	}
	if got == nil || !got.Unauthorized || len(got.RequiredScopes) != 1 {
		t.Fatalf("decodeUnauthorized() = %#v", got)
	}
}

func TestIsJSONNull(t *testing.T) {
	if !isJSONNull(json.RawMessage(" null ")) {
		t.Fatal("isJSONNull should accept trimmed null")
	}
	if isJSONNull(json.RawMessage(`{"x":1}`)) {
		t.Fatal("isJSONNull should reject objects")
	}
}

func TestEscapePath(t *testing.T) {
	got := escapePath("/runbooks/hello world/ops#1.md/")
	want := "runbooks/hello%20world/ops%231.md"
	if got != want {
		t.Fatalf("escapePath() = %q, want %q", got, want)
	}
}

func TestAPIErrorError(t *testing.T) {
	err := (&APIError{StatusCode: 403, Body: "forbidden"}).Error()
	if err != "API error 403: forbidden" {
		t.Fatalf("Error() = %q", err)
	}
}

func TestDecodeUnauthorizedRejectsRegularPayload(t *testing.T) {
	raw, err := json.Marshal(models.Integration{PublicID: "int_1"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got, ok := decodeUnauthorized(raw); ok || got != nil {
		t.Fatalf("decodeUnauthorized() = %#v, %v; want nil, false", got, ok)
	}
}
