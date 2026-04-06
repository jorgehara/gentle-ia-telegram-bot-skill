package config

import (
	"os"
	"testing"
	"time"
)

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST_ENV_VAR", "hello")
	defer os.Unsetenv("TEST_ENV_VAR")

	tests := []struct {
		name       string
		key        string
		defaultVal string
		want       string
	}{
		{"existing env var", "TEST_ENV_VAR", "default", "hello"},
		{"missing env var uses default", "NONEXISTENT_VAR_123", "fallback", "fallback"},
		{"empty env var uses default", "TEST_EMPTY_VAR", "default", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "empty env var uses default" {
				os.Setenv("TEST_EMPTY_VAR", "")
				defer os.Unsetenv("TEST_EMPTY_VAR")
			}
			got := getEnv(tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("getEnv() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	os.Setenv("TEST_INT_VAR", "42")
	defer os.Unsetenv("TEST_INT_VAR")

	tests := []struct {
		name       string
		key        string
		defaultVal int
		want       int
	}{
		{"valid int", "TEST_INT_VAR", 0, 42},
		{"missing var uses default", "NONEXISTENT_INT", 99, 99},
		{"invalid int uses default", "TEST_INT_VAR_BAD", 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "invalid int uses default" {
				os.Setenv("TEST_INT_VAR_BAD", "not-a-number")
				defer os.Unsetenv("TEST_INT_VAR_BAD")
			}
			got := getEnvInt(tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("getEnvInt() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name       string
		envVal     string
		defaultVal bool
		want       bool
	}{
		{"true string", "true", false, true},
		{"1 string", "1", false, true},
		{"yes string", "yes", false, true},
		{"false string", "false", true, false},
		{"missing uses default", "", true, true},
		{"random string returns false", "nope", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_BOOL_" + tt.name
			if tt.envVal != "" {
				os.Setenv(key, tt.envVal)
				defer os.Unsetenv(key)
			}
			got := getEnvBool(key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("getEnvBool(%q) = %v, want %v", tt.envVal, got, tt.want)
			}
		})
	}
}

func TestGetEnvDuration(t *testing.T) {
	os.Setenv("TEST_DUR_VAR", "5m")
	defer os.Unsetenv("TEST_DUR_VAR")

	tests := []struct {
		name       string
		key        string
		defaultVal time.Duration
		want       time.Duration
	}{
		{"valid duration", "TEST_DUR_VAR", 1 * time.Hour, 5 * time.Minute},
		{"missing uses default", "NONEXISTENT_DUR", 2 * time.Hour, 2 * time.Hour},
		{"invalid uses default", "TEST_DUR_BAD", 30 * time.Minute, 30 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "invalid uses default" {
				os.Setenv("TEST_DUR_BAD", "not-duration")
				defer os.Unsetenv("TEST_DUR_BAD")
			}
			got := getEnvDuration(tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("getEnvDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAllowedChat(t *testing.T) {
	tests := []struct {
		name        string
		allowedIDs  []int64
		chatID      int64
		wantAllowed bool
	}{
		{"empty list allows all", []int64{}, 12345, true},
		{"chat in whitelist", []int64{100, 200, 300}, 200, true},
		{"chat not in whitelist", []int64{100, 200, 300}, 999, false},
		{"single allowed", []int64{42}, 42, true},
		{"single not allowed", []int64{42}, 43, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{AllowedChatIDs: tt.allowedIDs}
			got := cfg.IsAllowedChat(tt.chatID)
			if got != tt.wantAllowed {
				t.Errorf("IsAllowedChat(%d) with %v = %v, want %v",
					tt.chatID, tt.allowedIDs, got, tt.wantAllowed)
			}
		})
	}
}

func TestGetEnvInt64Slice(t *testing.T) {
	tests := []struct {
		name       string
		envVal     string
		defaultVal []int64
		want       []int64
	}{
		{"single value", "12345", []int64{}, []int64{12345}},
		{"comma separated", "100,200,300", []int64{}, []int64{100, 200, 300}},
		{"empty uses default", "", []int64{1, 2}, []int64{1, 2}},
		{"ignores empty parts", "1,,2", []int64{}, []int64{1, 2}},
		{"skips invalid", "1,abc,3", []int64{}, []int64{1, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_INT64_" + tt.name
			if tt.envVal != "" {
				os.Setenv(key, tt.envVal)
				defer os.Unsetenv(key)
			}
			got := getEnvInt64Slice(key, tt.defaultVal)
			if len(got) != len(tt.want) {
				t.Errorf("getEnvInt64Slice() len = %d, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("getEnvInt64Slice()[%d] = %d, want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}
