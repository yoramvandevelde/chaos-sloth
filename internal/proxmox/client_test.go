package proxmox

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	client := New(srv.URL, "user@pam!test", "test-secret", false)
	return srv, client
}

func TestHibernateVM(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api2/json/nodes/pve1/qemu/101/status/suspend" {
			t.Errorf("path = %s, want /api2/json/nodes/pve1/qemu/101/status/suspend", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "todisk=1") {
			t.Errorf("body = %q, want todisk=1", string(body))
		}
		w.WriteHeader(200)
	})
	defer srv.Close()

	if err := client.HibernateVM(context.Background(), "pve1", 101); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPauseVM(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/pve1/qemu/101/status/suspend" {
			t.Errorf("path = %s, want /api2/json/nodes/pve1/qemu/101/status/suspend", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if len(body) > 0 {
			t.Errorf("pause should have empty body, got %q", string(body))
		}
		w.WriteHeader(200)
	})
	defer srv.Close()

	if err := client.PauseVM(context.Background(), "pve1", 101); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResumeVM(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/pve1/qemu/101/status/resume" {
			t.Errorf("path = %s, want /api2/json/nodes/pve1/qemu/101/status/resume", r.URL.Path)
		}
		w.WriteHeader(200)
	})
	defer srv.Close()

	if err := client.ResumeVM(context.Background(), "pve1", 101); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStopVM(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/pve2/qemu/201/status/stop" {
			t.Errorf("path = %s, want /api2/json/nodes/pve2/qemu/201/status/stop", r.URL.Path)
		}
		w.WriteHeader(200)
	})
	defer srv.Close()

	if err := client.StopVM(context.Background(), "pve2", 201); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartVM(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/pve2/qemu/201/status/start" {
			t.Errorf("path = %s, want /api2/json/nodes/pve2/qemu/201/status/start", r.URL.Path)
		}
		w.WriteHeader(200)
	})
	defer srv.Close()

	if err := client.StartVM(context.Background(), "pve2", 201); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResetVM(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/pve1/qemu/101/status/reset" {
			t.Errorf("path = %s, want /api2/json/nodes/pve1/qemu/101/status/reset", r.URL.Path)
		}
		w.WriteHeader(200)
	})
	defer srv.Close()

	if err := client.ResetVM(context.Background(), "pve1", 101); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPost_AuthHeader(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		expected := "PVEAPIToken=user@pam!test=test-secret"
		if auth != expected {
			t.Errorf("Authorization = %q, want %q", auth, expected)
		}
		w.WriteHeader(200)
	})
	defer srv.Close()

	if err := client.ResumeVM(context.Background(), "pve1", 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPost_ErrorStatus(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"errors":{"node":"not found"}}`)) //nolint:errcheck
	})
	defer srv.Close()

	err := client.ResumeVM(context.Background(), "pve1", 1)
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %q, should contain status 500", err.Error())
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, should contain response body", err.Error())
	}
}

func TestPost_404Error(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"data":null}`)) //nolint:errcheck
	})
	defer srv.Close()

	err := client.StopVM(context.Background(), "pve1", 999)
	if err == nil {
		t.Fatal("expected error for 404 status, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error = %q, should contain 404", err.Error())
	}
}

func TestPost_ContentHeader(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if len(body) > 0 {
			ct := r.Header.Get("Content-Type")
			if ct != "application/x-www-form-urlencoded" {
				t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", ct)
			}
		}
		w.WriteHeader(200)
	})
	defer srv.Close()

	// HibernateVM sends body with todisk=1
	if err := client.HibernateVM(context.Background(), "pve1", 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPost_NoContentHeaderWithoutBody(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			ct := r.Header.Get("Content-Type")
			if ct != "" {
				t.Errorf("Content-Type = %q, should be empty for no body", ct)
			}
		}
		w.WriteHeader(200)
	})
	defer srv.Close()

	// PauseVM sends no body
	if err := client.PauseVM(context.Background(), "pve1", 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNew_TrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	// Client should handle trailing slash in URL
	client := New(srv.URL+"/", "id", "secret", false)
	if err := client.ResumeVM(context.Background(), "pve1", 1); err != nil {
		t.Fatalf("unexpected error with trailing slash URL: %v", err)
	}
}

func TestPost_ContextCancelled(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := client.ResumeVM(ctx, "pve1", 1)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}
