package root

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	errs "github.com/we-promise/sure-cli/internal/errors"
)

func TestInsightsCommandShape(t *testing.T) {
	cmd := newInsightsCmd()

	list := findSub(t, cmd, "list")
	if list.Args == nil || list.Args(list, []string{"extra"}) == nil {
		t.Fatal("insights list takes no arguments")
	}
	// GET /api/v1/insights accepts no filters or paging; neither does the CLI.
	if list.Flags().HasFlags() {
		t.Fatalf("insights list must not expose flags the server ignores")
	}

	ack := findSub(t, cmd, "ack")
	if ack.Args == nil || ack.Args(ack, nil) == nil {
		t.Fatal("insights ack requires an insight id")
	}
	if ack.Flags().Lookup("apply") == nil {
		t.Fatal("insights ack must be dry-run by default with --apply to execute")
	}
}

// insightsServer records the request and answers with status and body.
func insightsServer(t *testing.T, status int, body string, gotMethod, gotPath *string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*gotMethod, *gotPath = r.Method, r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	apiClientFor(t, srv.URL)
}

func TestFetchInsights_ReadsServerFeed(t *testing.T) {
	var method, path string
	insightsServer(t, http.StatusOK, `{"insights":[{"id":"i1","type":"bank_fees","title":"Fees","body":"b","priority":"high","status":"active","generated_at":"2026-11-01T00:00:00Z"}]}`, &method, &path)

	res, status, ce := fetchInsights()
	if ce != nil {
		t.Fatalf("unexpected error: %+v", ce)
	}
	if method != http.MethodGet || path != "/api/v1/insights" || status != http.StatusOK {
		t.Fatalf("request = %s %s (status %d)", method, path, status)
	}
	feed := res.(map[string]any)["insights"].([]any)
	if len(feed) != 1 || feed[0].(map[string]any)["type"] != "bank_fees" {
		t.Fatalf("feed = %#v", feed)
	}
}

func TestFetchInsights_PreviewFeaturesDisabledIsActionable(t *testing.T) {
	var method, path string
	insightsServer(t, http.StatusForbidden, `{"error":"feature_disabled","message":"Preview features are not enabled for this user"}`, &method, &path)

	res, _, ce := fetchInsights()
	if res != nil || ce == nil {
		t.Fatalf("expected an error, got res=%#v", res)
	}
	if ce.Code != errs.CodeForbidden || ce.Details["api_error"] != "feature_disabled" {
		t.Fatalf("ce = %+v", ce)
	}
	if !strings.Contains(ce.Message, "Settings > Preferences") || !strings.Contains(ce.Message, "Preview features are not enabled") {
		t.Fatalf("message must keep Sure's reason and say how to enable preview features: %q", ce.Message)
	}
}

func TestFetchInsights_OtherForbiddenUnchanged(t *testing.T) {
	var method, path string
	insightsServer(t, http.StatusForbidden, `{"error":"insufficient_scope","message":"This action requires the 'read' scope"}`, &method, &path)

	_, _, ce := fetchInsights()
	if ce == nil || ce.Code != errs.CodeAuthInvalid || strings.Contains(ce.Message, "Preferences") {
		t.Fatalf("ce = %+v", ce)
	}
}

func TestAcknowledgeInsight_PatchesAcknowledgeEndpoint(t *testing.T) {
	var method, path string
	insightsServer(t, http.StatusNoContent, ``, &method, &path)

	status, ce := acknowledgeInsight("8f6c0f5e-1111-4222-8333-944445555666")
	if ce != nil {
		t.Fatalf("unexpected error: %+v", ce)
	}
	if method != http.MethodPatch || path != "/api/v1/insights/8f6c0f5e-1111-4222-8333-944445555666/acknowledge" || status != http.StatusNoContent {
		t.Fatalf("request = %s %s (status %d)", method, path, status)
	}
}

func TestAcknowledgeInsight_EscapesID(t *testing.T) {
	var method, path string
	insightsServer(t, http.StatusNotFound, `{"error":"not_found","message":"Record not found"}`, &method, &path)

	_, ce := acknowledgeInsight("../accounts")
	if path != "/api/v1/insights/..%2Faccounts/acknowledge" {
		t.Fatalf("path = %q, id must be path-escaped", path)
	}
	if ce == nil || ce.Code != errs.CodeNotFound {
		t.Fatalf("ce = %+v", ce)
	}
}

func TestAcknowledgeInsight_PreviewFeaturesDisabledIsActionable(t *testing.T) {
	var method, path string
	insightsServer(t, http.StatusForbidden, `{"error":"feature_disabled","message":"Preview features are not enabled for this user"}`, &method, &path)

	_, ce := acknowledgeInsight("i1")
	if ce == nil || !strings.Contains(ce.Message, "Settings > Preferences") {
		t.Fatalf("ce = %+v", ce)
	}
}

const insightID = "8f6c0f5e-1111-4222-8333-944445555666"

// runInsights runs `insights args...` and returns the envelope's data.
func runInsights(t *testing.T, args ...string) map[string]any {
	t.Helper()
	cmd := newInsightsCmd()
	cmd.SetArgs(args)
	out := captureStdout(t, func() {
		format = "json"
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("unmarshal: %v\nout=%q", err, out)
	}
	return env.Data
}

func TestInsightsAck_DryRunByDefault(t *testing.T) {
	data := runInsights(t, "ack", insightID)
	req, _ := data["request"].(map[string]any)
	if data["dry_run"] != true || req["method"] != "PATCH" || req["path"] != "/api/v1/insights/"+insightID+"/acknowledge" {
		t.Fatalf("dry run = %#v", data)
	}
	if _, hasBody := req["body"]; hasBody {
		t.Fatalf("acknowledge takes no body, got %#v", req)
	}
}

func TestInsightsAck_ApplyReportsOnlyWhatSureConfirms(t *testing.T) {
	var method, path string
	// Sure answers 204 also when the insight was already acknowledged or expired.
	insightsServer(t, http.StatusNoContent, ``, &method, &path)

	data := runInsights(t, "ack", insightID, "--apply")
	if method != http.MethodPatch || data["id"] != insightID || data["in_feed"] != false {
		t.Fatalf("request %s, data = %#v", method, data)
	}
	if _, claimed := data["acknowledged"]; claimed {
		t.Fatalf("a 204 does not prove this call acknowledged the insight: %#v", data)
	}
}

func TestValidateInsightID(t *testing.T) {
	for _, id := range []string{insightID, strings.ToUpper(insightID)} {
		if err := validateInsightID(id); err != nil {
			t.Fatalf("%s rejected: %v", id, err)
		}
	}
	for _, id := range []string{"i1", "../accounts", insightID + "0", insightID[1:], "8f6c0f5e11114222833394444555566600000"} {
		if validateInsightID(id) == nil {
			t.Fatalf("%q must be rejected", id)
		}
	}
}
