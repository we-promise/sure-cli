package root

import (
	"testing"
)

func TestBuildImportPreflightPayload_RequiresType(t *testing.T) {
	_, err := buildImportPreflightPayload(importPreflightOpts{RawFileContent: "x"})
	if err == nil {
		t.Fatal("expected missing type to error")
	}
}

func TestBuildImportPreflightPayload_RequiresContentSource(t *testing.T) {
	_, err := buildImportPreflightPayload(importPreflightOpts{Type: "TransactionImport"})
	if err == nil {
		t.Fatal("expected missing file/raw_file_content to error")
	}
}

func TestBuildImportPreflightPayload_RejectsBothSources(t *testing.T) {
	_, err := buildImportPreflightPayload(importPreflightOpts{Type: "TransactionImport", File: "/tmp/x.csv", RawFileContent: "x"})
	if err == nil {
		t.Fatal("expected providing both file and raw content to error")
	}
}

func TestBuildImportPreflightPayload_RawContent(t *testing.T) {
	p, err := buildImportPreflightPayload(importPreflightOpts{
		Type:           "TransactionImport",
		RawFileContent: "Date,Name,Amount\n2026-01-01,Coffee,-3.50\n",
		DateColLabel:   "Date",
		AmountColLabel: "Amount",
		NameColLabel:   "Name",
		RowsToSkip:     "0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Fields["type"] != "TransactionImport" {
		t.Fatalf("type field: %q", p.Fields["type"])
	}
	if p.Fields["raw_file_content"] == "" {
		t.Fatal("raw_file_content should be set")
	}
	if p.Fields["date_col_label"] != "Date" || p.Fields["amount_col_label"] != "Amount" || p.Fields["name_col_label"] != "Name" {
		t.Fatalf("col labels not propagated: %#v", p.Fields)
	}
	if p.Fields["rows_to_skip"] != "0" {
		t.Fatalf("rows_to_skip not propagated: %q", p.Fields["rows_to_skip"])
	}
}

func TestImportsPreflightRegistered(t *testing.T) {
	got, _, err := New().Find([]string{"imports", "preflight"})
	if err != nil {
		t.Fatalf("imports preflight not registered: %v", err)
	}
	if got.Name() != "preflight" {
		t.Fatalf("resolved to %q, want preflight", got.Name())
	}
}
