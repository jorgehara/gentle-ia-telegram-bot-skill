package opencode

import (
	"testing"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{"shorter than max", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"longer than max", "hello world", 5, "hello..."},
		{"empty string", "", 5, ""},
		{"max zero", "hello", 0, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		name string
		a, b int
		want int
	}{
		{"a less than b", 3, 5, 3},
		{"a greater than b", 10, 2, 2},
		{"equal", 7, 7, 7},
		{"negative", -5, 3, -5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := min(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestNewOpencodeClient(t *testing.T) {
	client := NewOpencodeClient("http://localhost:4096/", "user", "pass")

	if client == nil {
		t.Fatal("NewOpencodeClient() returned nil")
	}
	if client.baseURL != "http://localhost:4096" {
		t.Errorf("baseURL = %q, want %q", client.baseURL, "http://localhost:4096")
	}
	if client.username != "user" {
		t.Errorf("username = %q, want %q", client.username, "user")
	}
	if client.password != "pass" {
		t.Errorf("password = %q, want %q", client.password, "pass")
	}
	if client.sessions == nil {
		t.Error("sessions map not initialized")
	}
	if client.httpClient == nil {
		t.Error("httpClient not initialized")
	}
}

func TestNewOpencodeClientTrimsTrailingSlash(t *testing.T) {
	client := NewOpencodeClient("http://example.com/", "u", "p")
	if client.baseURL != "http://example.com" {
		t.Errorf("baseURL = %q, want %q", client.baseURL, "http://example.com")
	}
}

func TestClearSession(t *testing.T) {
	client := NewOpencodeClient("http://localhost:4096", "u", "p")

	// Simulate existing session
	client.sessions[12345] = "session-abc"
	client.ClearSession(12345)

	if _, exists := client.sessions[12345]; exists {
		t.Error("session should have been cleared")
	}

	// Clearing non-existent session should not panic
	client.ClearSession(99999)
}
