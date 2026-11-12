package config

import (
	"os"
	"path/filepath"
	"testing"
)

func envFrom(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestDefaults(t *testing.T) {
	cfg, err := Load(envFrom(nil))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "auth user header", got: cfg.Auth.User, want: "X-Forwarded-User"},
		{name: "auth name header", got: cfg.Auth.Name, want: "X-Forwarded-Preferred-Username"},
		{name: "auth email header", got: cfg.Auth.Email, want: "X-Forwarded-Email"},
		{name: "auth groups header", got: cfg.Auth.Groups, want: "X-Forwarded-Groups"},
		{name: "log level", got: cfg.LogLevel, want: "info"},
		{name: "static dir", got: cfg.StaticDir, want: ""},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
	if cfg.Port != 8080 {
		t.Errorf("port = %d, want 8080", cfg.Port)
	}
}

func TestTrimming(t *testing.T) {
	cfg, err := Load(envFrom(map[string]string{
		"PORT":             " 9090 ",
		"LOG_LEVEL":        " DEBUG ",
		"AUTH_HEADER_USER": " X-User ",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 9090 || cfg.LogLevel != "debug" || cfg.Auth.User != "X-User" {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
	}{
		{name: "port not a number", values: map[string]string{"PORT": "abc"}},
		{name: "port out of range", values: map[string]string{"PORT": "70000"}},
		{name: "unknown log level", values: map[string]string{"LOG_LEVEL": "trace"}},
		{name: "static dir missing", values: map[string]string{"STATIC_DIR": "/does/not/exist"}},
		{name: "header with whitespace", values: map[string]string{"AUTH_HEADER_USER": "X User"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Load(envFrom(tc.values)); err == nil {
				t.Errorf("Load() = nil, want error")
			}
		})
	}
}

func TestStaticDirExists(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(envFrom(map[string]string{"STATIC_DIR": dir}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.StaticDir != dir {
		t.Errorf("static dir = %q, want %q", cfg.StaticDir, dir)
	}
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := Load(envFrom(map[string]string{"STATIC_DIR": file})); err == nil {
		t.Error("Load() with a file as STATIC_DIR = nil, want error")
	}
}
