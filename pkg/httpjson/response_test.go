package httpjson

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewSuccess(t *testing.T) {
	response := NewSuccess("created", struct {
		ID string `json:"id"`
	}{ID: "user-1"})

	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	want := `{"code":0,"message":"created","data":{"id":"user-1"}}`
	if string(encoded) != want {
		t.Errorf("response JSON = %s, want %s", encoded, want)
	}
}

func TestNewErrorConvertsErrorToString(t *testing.T) {
	response := NewError(1001, "invalid request", errors.New("email is required"))

	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	want := `{"code":1001,"message":"invalid request","error":"email is required"}`
	if string(encoded) != want {
		t.Errorf("response JSON = %s, want %s", encoded, want)
	}
}

func TestNewErrorPreservesStructuredDetail(t *testing.T) {
	response := NewError(1001, "invalid request", map[string]string{"field": "email"})

	detail, ok := response.Error.(map[string]string)
	if !ok || detail["field"] != "email" {
		t.Fatalf("Error = %#v, want structured detail", response.Error)
	}
}

func TestNewErrorNeverUsesSuccessCode(t *testing.T) {
	response := NewError(SuccessCode, "failed", "reason")
	if response.Code == SuccessCode {
		t.Fatal("error response used the success code")
	}
}

func TestResponderWritesSuccessEnvelope(t *testing.T) {
	responder := NewResponder(slog.New(slog.NewTextHandler(io.Discard, nil)))
	response := httptest.NewRecorder()

	responder.Success(
		t.Context(),
		response,
		http.StatusCreated,
		"created",
		map[string]string{"id": "user-1"},
	)

	if response.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	want := "{\"code\":0,\"message\":\"created\",\"data\":{\"id\":\"user-1\"}}\n"
	if response.Body.String() != want {
		t.Errorf("body = %q, want %q", response.Body.String(), want)
	}
}
