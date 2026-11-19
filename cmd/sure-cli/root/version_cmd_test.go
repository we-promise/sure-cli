package root

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/we-promise/sure-cli/internal/config"
	errs "github.com/we-promise/sure-cli/internal/errors"
)

func TestVersionCommandShape(t *testing.T) {
	got, _, err := New().Find([]string{"version"})
	if err != nil {
		t.Fatalf("version not registered: %v", err)
	}
	if got.Flags().Lookup("server") == nil {
		t.Fatal("expected --server flag on version")
	}
}

func TestVersionCommand_LocalOutputUnchanged(t *testing.T) {
	cmd := newVersionCmd()
	cmd.SetArgs(nil)
	out := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if !strings.HasPrefix(out, "sure-cli "+version+"\n") || !strings.Contains(out, "commit: "+commit) || !strings.Contains(out, "built:  "+date) {
		t.Fatalf("unexpected local version output: %q", out)
	}
}

func TestFetchServerVersion_Success(t *testing.T) {
	var gotPath, gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotKey = r.URL.Path, r.Header.Get("X-Api-Key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"2026.11.2","commit_sha":"0123abc"}`))
	}))
	t.Cleanup(srv.Close)
	apiClientFor(t, srv.URL)

	res, ce := fetchServerVersion()
	if ce != nil {
		t.Fatalf("unexpected error: %+v", ce)
	}
	if gotPath != "/api/v1/version" || gotKey != "test-key" {
		t.Fatalf("path=%q key=%q", gotPath, gotKey)
	}
	m := res.(map[string]any)
	if m["version"] != "2026.11.2" || m["commit_sha"] != "0123abc" {
		t.Fatalf("server version = %#v", m)
	}
}

func TestFetchServerVersion_HTTPErrorIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	t.Cleanup(srv.Close)
	apiClientFor(t, srv.URL)

	res, ce := fetchServerVersion()
	if res != nil || ce == nil {
		t.Fatalf("expected error, got res=%#v ce=%#v", res, ce)
	}
	if ce.Code != errs.CodeAuthRequired || ce.Details["status"] != http.StatusUnauthorized {
		t.Fatalf("ce = %+v", ce)
	}
}

func TestFetchServerVersion_NotConfigured(t *testing.T) {
	viper.Reset()
	_ = config.Init("/tmp/sure-cli-test-does-not-exist.yaml")
	viper.Set("api_url", "")

	res, ce := fetchServerVersion()
	if res != nil || ce == nil || ce.Code != errs.CodeConfigMissing {
		t.Fatalf("expected config_missing, got res=%#v ce=%#v", res, ce)
	}
}

func TestVersionReport_ServerFailureKeepsLocalVersion(t *testing.T) {
	report := buildVersionReport(nil, errs.New(errs.CodeNetwork, "Connection refused"))
	b, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	_ = json.Unmarshal(b, &got)
	cli := got["cli"].(map[string]any)
	if cli["version"] != version || cli["commit"] != commit || cli["built"] != date {
		t.Fatalf("cli = %#v", cli)
	}
	if _, has := got["server"]; has {
		t.Fatalf("server should be omitted on failure: %#v", got)
	}
	se := got["server_error"].(map[string]any)
	if se["code"] != errs.CodeNetwork {
		t.Fatalf("server_error = %#v", se)
	}
}

func TestVersionReport_ServerSuccess(t *testing.T) {
	report := buildVersionReport(map[string]any{"version": "2026.11.2"}, nil)
	if report["server"] == nil {
		t.Fatalf("server missing: %#v", report)
	}
	if _, has := report["server_error"]; has {
		t.Fatalf("server_error should be omitted on success: %#v", report)
	}
}
