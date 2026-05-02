package rakun

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"rakun/internal/config"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCollectCollectsGroupsInParallelAndPreservesOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var started atomic.Int32
	ready := make(chan struct{})
	var readyOnce sync.Once

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch {
		case r.URL.Path == "/api/v3/users/alpha" || r.URL.Path == "/api/v3/users/beta":
			if started.Add(1) == 2 {
				readyOnce.Do(func() {
					close(ready)
				})
			}

			select {
			case <-ready:
			case <-r.Context().Done():
				return
			}

			_, _ = fmt.Fprint(w, `{"type":"User"}`)
		case r.URL.Path == "/api/v3/user":
			_, _ = fmt.Fprint(w, `{"login":"viewer"}`)
		case (r.URL.Path == "/api/v3/users/alpha/repos" || r.URL.Path == "/api/v3/users/beta/repos") && r.URL.Query().Get("type") == "owner" && r.URL.Query().Get("sort") == "full_name" && r.URL.Query().Get("page") == "1":
			switch r.URL.Path {
			case "/api/v3/users/alpha/repos":
				_, _ = fmt.Fprint(w, `[{"name":"repo","full_name":"alpha/repo"}]`)
			case "/api/v3/users/beta/repos":
				_, _ = fmt.Fprint(w, `[{"name":"repo","full_name":"beta/repo"}]`)
			default:
				http.NotFound(w, r)
			}
		case (r.URL.Path == "/api/v3/users/alpha/repos" || r.URL.Path == "/api/v3/users/beta/repos") && r.URL.Query().Get("type") == "owner" && r.URL.Query().Get("sort") == "full_name" && r.URL.Query().Get("page") == "2":
			_, _ = fmt.Fprint(w, `[]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	originalTransport := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	defer func() {
		http.DefaultTransport = originalTransport
	}()

	app, err := New(t.TempDir(), 1)
	if err != nil {
		t.Fatalf("new rakun: %v", err)
	}

	tasks, err := app.Collect(ctx, []config.Group{
		{
			Domain: serverURL.Host,
			Type:   "github",
			Token:  config.Token{Value: "test-token", Set: true},
			Namespaces: map[string]*config.Namespace{
				"alpha": nil,
			},
		},
		{
			Domain: serverURL.Host,
			Type:   "github",
			Token:  config.Token{Value: "test-token", Set: true},
			Namespaces: map[string]*config.Namespace{
				"beta": nil,
			},
		},
	})
	if err != nil {
		t.Fatalf("collect tasks: %v", err)
	}

	got := make([]string, 0, len(tasks))
	for _, task := range tasks {
		syncTask, ok := task.(syncTask)
		if !ok {
			t.Fatalf("unexpected task type %T", task)
		}
		got = append(got, syncTask.spec.Remote)
	}

	expected := []string{
		"https://" + serverURL.Host + "/alpha/repo.git",
		"https://" + serverURL.Host + "/beta/repo.git",
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("unexpected tasks:\n%v", got)
	}

	if started.Load() != 2 {
		t.Fatalf("expected 2 groups to start discovery, got %d", started.Load())
	}
}
