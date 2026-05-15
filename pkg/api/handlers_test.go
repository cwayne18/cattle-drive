package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"rancherlabs/cattle-drive/pkg/cluster"
)

// ── sliceLogger ───────────────────────────────────────────────────────────────

func TestSliceLogger_SuccessEvent(t *testing.T) {
	l := &sliceLogger{}
	l.LogEvent(cluster.MigrateEvent{Kind: "project", Name: "my-project"})

	if len(l.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(l.entries))
	}
	e := l.entries[0]
	if e.Error {
		t.Error("success event should not be flagged as error")
	}
	if !strings.Contains(e.Message, "my-project") {
		t.Errorf("message %q should contain object name", e.Message)
	}
}

func TestSliceLogger_ErrorEvent(t *testing.T) {
	l := &sliceLogger{}
	l.LogEvent(cluster.MigrateEvent{
		Kind: "crtb",
		Name: "binding-1",
		Err:  errors.New("forbidden"),
	})

	if len(l.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(l.entries))
	}
	e := l.entries[0]
	if !e.Error {
		t.Error("error event should be flagged as error")
	}
	if !strings.Contains(e.Message, "forbidden") {
		t.Errorf("message %q should contain the error text", e.Message)
	}
	if !strings.Contains(e.Message, "binding-1") {
		t.Errorf("message %q should contain object name", e.Message)
	}
}

func TestSliceLogger_MultipleEvents(t *testing.T) {
	l := &sliceLogger{}
	l.LogEvent(cluster.MigrateEvent{Kind: "project", Name: "p1"})
	l.LogEvent(cluster.MigrateEvent{Kind: "prtb", Name: "prtb-1"})
	l.LogEvent(cluster.MigrateEvent{Kind: "project", Name: "p2", Err: errors.New("boom")})

	if len(l.entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(l.entries))
	}
	if l.entries[0].Error || l.entries[1].Error {
		t.Error("first two entries should not be errors")
	}
	if !l.entries[2].Error {
		t.Error("third entry should be an error")
	}
}

// ── clusterNotFoundError ─────────────────────────────────────────────────────

func TestClusterNotFoundError_SourceMissing(t *testing.T) {
	err := &clusterNotFoundError{source: "src", target: "tgt", sourceFound: false}
	want := "source cluster 'src' not found"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestClusterNotFoundError_TargetMissing(t *testing.T) {
	err := &clusterNotFoundError{source: "src", target: "tgt", sourceFound: true}
	want := "target cluster 'tgt' not found"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

// ── buildStatusResponse ───────────────────────────────────────────────────────

func newTestCluster(displayName string) *cluster.Cluster {
	return &cluster.Cluster{
		Obj: &v3.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "c-" + displayName},
			Spec:       v3.ClusterSpec{DisplayName: displayName},
		},
	}
}

func TestBuildStatusResponse_EmptyClusters(t *testing.T) {
	sc := newTestCluster("source")
	tc := newTestCluster("target")
	resp := buildStatusResponse(sc, tc)

	if resp.Source != "source" {
		t.Errorf("Source = %q, want %q", resp.Source, "source")
	}
	if resp.Target != "target" {
		t.Errorf("Target = %q, want %q", resp.Target, "target")
	}
	if resp.Projects == nil || len(resp.Projects) != 0 {
		t.Errorf("Projects should be non-nil empty slice, got %v", resp.Projects)
	}
	if resp.CRTBs == nil || len(resp.CRTBs) != 0 {
		t.Errorf("CRTBs should be non-nil empty slice, got %v", resp.CRTBs)
	}
	if resp.ClusterRepos == nil || len(resp.ClusterRepos) != 0 {
		t.Errorf("ClusterRepos should be non-nil empty slice, got %v", resp.ClusterRepos)
	}
}

func TestBuildStatusResponse_WithObjects(t *testing.T) {
	sc := newTestCluster("source")
	tc := newTestCluster("target")

	sc.ToMigrate = cluster.ToMigrate{
		Projects: []*cluster.Project{
			{
				Name:     "my-project",
				Migrated: false,
				Diff:     false,
				PRTBs: []*cluster.ProjectRoleTemplateBinding{
					{Name: "prtb-1", Migrated: true, Description: "admin"},
				},
				Namespaces: []*cluster.Namespace{
					{Name: "ns-1", Migrated: false, Diff: true},
				},
			},
		},
		CRTBs: []*cluster.ClusterRoleTemplateBinding{
			{Name: "crtb-1", Migrated: true, Description: "cluster-admin"},
		},
		ClusterRepos: []*cluster.ClusterRepo{
			{Name: "repo-1", Migrated: false, Diff: true},
		},
	}

	resp := buildStatusResponse(sc, tc)

	if len(resp.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(resp.Projects))
	}
	p := resp.Projects[0]
	if p.Name != "my-project" {
		t.Errorf("project name = %q", p.Name)
	}
	if p.Migrated {
		t.Error("project should not be migrated")
	}
	if len(p.PRTBs) != 1 || !p.PRTBs[0].Migrated {
		t.Errorf("expected migrated prtb, got %v", p.PRTBs)
	}
	if len(p.Namespaces) != 1 || !p.Namespaces[0].Diff {
		t.Errorf("expected diff namespace, got %v", p.Namespaces)
	}
	if len(resp.CRTBs) != 1 || resp.CRTBs[0].Name != "crtb-1" {
		t.Errorf("unexpected CRTBs: %v", resp.CRTBs)
	}
	if len(resp.ClusterRepos) != 1 || !resp.ClusterRepos[0].Diff {
		t.Errorf("unexpected ClusterRepos: %v", resp.ClusterRepos)
	}
}

func TestBuildStatusResponse_NilSlicesMarshallAsArrays(t *testing.T) {
	// When there are no items, the JSON output must use [] not null.
	sc := newTestCluster("source")
	tc := newTestCluster("target")
	resp := buildStatusResponse(sc, tc)

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	s := string(b)
	for _, field := range []string{`"projects":[]`, `"clusterRoleBindings":[]`, `"catalogRepos":[]`} {
		if !strings.Contains(s, field) {
			t.Errorf("expected %q in JSON output: %s", field, s)
		}
	}
}

// ── corsMiddleware ────────────────────────────────────────────────────────────

func TestCORSMiddleware_OptionsReturns204(t *testing.T) {
	handler := corsMiddleware([]string{"https://rancher.example.com"}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodOptions, "/api/clusters", nil)
	req.Header.Set("Origin", "https://rancher.example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "https://rancher.example.com" {
		t.Error("missing CORS origin header")
	}
	if w.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Error("missing CORS allow-headers header")
	}
}

func TestCORSMiddleware_PassesNonOptions(t *testing.T) {
	called := false
	handler := corsMiddleware([]string{"https://rancher.example.com"}, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/api/clusters", nil)
	req.Header.Set("Origin", "https://rancher.example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	if !called {
		t.Error("inner handler was not called")
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "https://rancher.example.com" {
		t.Error("missing CORS origin header on POST response")
	}
}

func TestCORSMiddleware_DefaultSameOriginModeOmitsCORSHeaders(t *testing.T) {
	handler := corsMiddleware(nil, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodOptions, "/api/clusters", nil)
	req.Header.Set("Origin", "https://dashboard.example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no CORS origin header, got %q", got)
	}
}

// ── bearerAuth ────────────────────────────────────────────────────────────────

func TestBearerAuth_MissingHeader(t *testing.T) {
	handler := bearerAuth("secret", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/api/clusters", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	assertJSONError(t, w)
}

func TestBearerAuth_WrongToken(t *testing.T) {
	handler := bearerAuth("correct", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/api/clusters", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestBearerAuth_CorrectToken(t *testing.T) {
	called := false
	handler := bearerAuth("secret", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/api/clusters", nil)
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	handler(w, req)

	if !called {
		t.Error("inner handler should have been called with the correct token")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestBearerAuth_OptionsPassesThrough(t *testing.T) {
	// Pre-flight requests must not be blocked by auth so CORS works from browsers.
	called := false
	handler := bearerAuth("secret", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodOptions, "/api/clusters", nil)
	// no Authorization header
	w := httptest.NewRecorder()
	handler(w, req)

	if !called {
		t.Error("inner handler should be called for OPTIONS even without token")
	}
}

func TestBearerAuth_BearerPrefixRequired(t *testing.T) {
	// Providing the token without the "Bearer " prefix must be rejected.
	handler := bearerAuth("secret", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/api/clusters", nil)
	req.Header.Set("Authorization", "secret") // missing "Bearer " prefix
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// ── HTTP handler method enforcement ──────────────────────────────────────────

func TestHandleClusters_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/clusters", nil)
	w := httptest.NewRecorder()
	handleClusters(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
	assertJSONError(t, w)
}

func TestHandleStatus_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	handleStatus(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
	assertJSONError(t, w)
}

func TestHandleMigrate_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/migrate", nil)
	w := httptest.NewRecorder()
	handleMigrate(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
	assertJSONError(t, w)
}

// ── HTTP handler input validation ─────────────────────────────────────────────

func TestHandleClusters_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/clusters", strings.NewReader("not-json"))
	w := httptest.NewRecorder()
	handleClusters(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	assertJSONError(t, w)
}

func TestHandleClusters_MissingKubeconfig(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/clusters",
		strings.NewReader(`{"kubeconfig":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handleClusters(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	assertJSONError(t, w)
}

func TestHandleStatus_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/status", strings.NewReader("not-json"))
	w := httptest.NewRecorder()
	handleStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleStatus_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"missing kubeconfig", `{"source":"a","target":"b"}`},
		{"missing source", `{"kubeconfig":"/kc","target":"b"}`},
		{"missing target", `{"kubeconfig":"/kc","source":"a"}`},
		{"empty object", `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/status", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handleStatus(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("%s: expected 400, got %d", tc.name, w.Code)
			}
			assertJSONError(t, w)
		})
	}
}

func TestHandleMigrate_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/migrate", strings.NewReader("not-json"))
	w := httptest.NewRecorder()
	handleMigrate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleMigrate_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"missing kubeconfig", `{"source":"a","target":"b"}`},
		{"missing source", `{"kubeconfig":"/kc","target":"b"}`},
		{"missing target", `{"kubeconfig":"/kc","source":"a"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/migrate", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handleMigrate(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("%s: expected 400, got %d", tc.name, w.Code)
			}
			assertJSONError(t, w)
		})
	}
}

func TestResolveKubeconfig(t *testing.T) {
	t.Run("request value wins", func(t *testing.T) {
		got, err := resolveKubeconfig("/req", "/default")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "/req" {
			t.Fatalf("expected /req, got %q", got)
		}
	})

	t.Run("falls back to default", func(t *testing.T) {
		got, err := resolveKubeconfig("", "/default")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "/default" {
			t.Fatalf("expected /default, got %q", got)
		}
	})

	t.Run("errors when both empty", func(t *testing.T) {
		_, err := resolveKubeconfig("", "")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestIsInClusterConfigValue(t *testing.T) {
	cases := map[string]bool{
		"incluster":       true,
		"in-cluster":      true,
		" INCLUSTER ":     true,
		"/tmp/kubeconfig": false,
		"":                false,
	}

	for input, want := range cases {
		if got := isInClusterConfigValue(input); got != want {
			t.Fatalf("isInClusterConfigValue(%q) = %v, want %v", input, got, want)
		}
	}
}

// ── NewServer route smoke test ────────────────────────────────────────────────

func TestNewServer_RoutesRegistered(t *testing.T) {
	srv := NewServer(ServerOptions{})

	routes := []string{"/api/clusters", "/api/status", "/api/migrate", "/healthz"}
	for _, path := range routes {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		// 404 means the route wasn't registered; anything else (405, 400, 200)
		// means it was found and the handler ran.
		if w.Code == http.StatusNotFound {
			t.Errorf("route %s returned 404 – not registered", path)
		}
	}
}

func TestNewServer_HealthzNoAuth(t *testing.T) {
	// Even when a token is set, /healthz must be reachable without auth.
	srv := NewServer(ServerOptions{APIToken: "secret"})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected /healthz to be 200 without token, got %d", w.Code)
	}
}

func TestNewServer_AuthEnforced(t *testing.T) {
	srv := NewServer(ServerOptions{APIToken: "secret"})

	req := httptest.NewRequest(http.MethodPost, "/api/clusters",
		strings.NewReader(`{"kubeconfig":"/kc"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", w.Code)
	}
}

func TestNewServer_AuthAllowsCorrectToken(t *testing.T) {
	srv := NewServer(ServerOptions{APIToken: "secret"})

	req := httptest.NewRequest(http.MethodPost, "/api/clusters",
		strings.NewReader(`{"kubeconfig":"/nonexistent"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// With the correct token the handler runs; because the kubeconfig path
	// doesn't exist we get a 400 (not a 401 or 403).
	if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
		t.Errorf("expected auth to pass, got %d", w.Code)
	}
}

func TestNewServer_CORSOptionsNoAuth(t *testing.T) {
	// Pre-flight OPTIONS to an API endpoint must succeed without auth even
	// when a token is configured.
	srv := NewServer(ServerOptions{
		APIToken:       "secret",
		AllowedOrigins: []string{"https://rancher.example.com"},
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/clusters", nil)
	req.Header.Set("Origin", "https://rancher.example.com")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS pre-flight, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "https://rancher.example.com" {
		t.Error("missing CORS origin header on OPTIONS response")
	}
}

func TestNewServer_DefaultKubeconfigUsed(t *testing.T) {
	srv := NewServer(ServerOptions{DefaultKubeconfig: "/nonexistent"})

	req := httptest.NewRequest(http.MethodPost, "/api/clusters", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if !strings.Contains(resp.Error, "failed to load kubeconfig") {
		t.Fatalf("expected kubeconfig load error, got %q", resp.Error)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// assertJSONError checks that the response body is a valid JSON object with
// a non-empty "error" field.
func assertJSONError(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Errorf("response body is not valid JSON: %v", err)
		return
	}
	if resp.Error == "" {
		t.Errorf("expected non-empty error field in response: %+v", resp)
	}
}
