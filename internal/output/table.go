package output

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/jedib0t/go-pretty/v6/table"
)

// PrintTable attempts to render a human-friendly table for known response shapes.
// Returns true if it rendered, false if unsupported.
func PrintTable(env Envelope) bool {
	m, ok := env.Data.(map[string]any)
	if !ok {
		return false
	}

	// Accounts list
	if accounts, ok := rows(m["accounts"]); ok {
		render(table.Row{"id", "name", "type", "currency", "balance", "classification"}, accounts, func(rowm map[string]any) table.Row {
			return table.Row{
				cell(rowm["id"]),
				cell(rowm["name"]),
				cell(rowm["account_type"]),
				cell(rowm["currency"]),
				cell(rowm["balance"]),
				cell(rowm["classification"]),
			}
		})
		return true
	}

	// Transactions list
	if txs, ok := rows(m["transactions"]); ok {
		render(table.Row{"id", "date", "name", "classification", "amount", "account"}, txs, func(rowm map[string]any) table.Row {
			acct := ""
			if am, ok := rowm["account"].(map[string]any); ok {
				acct = cell(am["name"])
			}
			return table.Row{
				cell(rowm["id"]),
				cell(rowm["date"]),
				cell(rowm["name"]),
				cell(rowm["classification"]),
				cell(rowm["amount"]),
				acct,
			}
		})
		return true
	}

	// Holdings list (Sure api/v1/holdings/_holding)
	if holdings, ok := rows(m["holdings"]); ok {
		render(table.Row{"id", "date", "ticker", "security", "qty", "price", "amount", "avg_cost", "lots", "account"}, holdings, func(rowm map[string]any) table.Row {
			return table.Row{
				cell(rowm["id"]),
				cell(rowm["date"]),
				nestedCell(rowm, "security", "ticker"),
				nestedCell(rowm, "security", "name"),
				cell(rowm["qty"]),
				cell(rowm["price"]),
				cell(rowm["amount"]),
				cell(rowm["avg_cost"]),
				lotsCell(rowm["lots"]),
				nestedCell(rowm, "account", "name"),
			}
		})
		return true
	}

	// Trades list (Sure api/v1/trades/_trade)
	if trades, ok := rows(m["trades"]); ok {
		render(table.Row{"id", "date", "ticker", "activity", "qty", "price", "amount", "lot_source", "account"}, trades, func(rowm map[string]any) table.Row {
			return table.Row{
				cell(rowm["id"]),
				cell(rowm["date"]),
				nestedCell(rowm, "security", "ticker"),
				cell(rowm["investment_activity_label"]),
				cell(rowm["qty"]),
				cell(rowm["price"]),
				cell(rowm["amount"]),
				cell(rowm["lot_source_trade_id"]),
				nestedCell(rowm, "account", "name"),
			}
		})
		return true
	}

	return false
}

// render prints one table row per item where envelopes go. Cells are Sure
// data, so they are made Printable first.
func render(header table.Row, items []map[string]any, row func(map[string]any) table.Row) {
	tw := table.NewWriter()
	tw.SetOutputMirror(envelopes())
	tw.AppendHeader(header)
	for _, item := range items {
		r := row(item)
		for i, c := range r {
			r[i] = Printable(fmt.Sprint(c))
		}
		tw.AppendRow(r)
	}
	tw.Render()
}

// Printable makes s safe to print on a terminal: tabs and line breaks become
// spaces and every other control character (C0, DEL, C1) is dropped, so text
// from Sure cannot carry escape sequences that move the cursor, recolour the
// screen or write the clipboard.
func Printable(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			return ' '
		case unicode.IsControl(r):
			return -1
		}
		return r
	}, s)
}

// rows reads a list of objects, decoded from one response ([]any) or
// gathered across pages ([]map[string]any).
func rows(v any) ([]map[string]any, bool) {
	switch list := v.(type) {
	case []map[string]any:
		return list, true
	case []any:
		out := make([]map[string]any, len(list))
		for i, item := range list {
			out[i], _ = item.(map[string]any)
		}
		return out, true
	}
	return nil, false
}

// cell renders a JSON value, leaving null blank instead of "<nil>".
func cell(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// nestedCell renders m[key][field] when m[key] is an object, else blank.
func nestedCell(m map[string]any, key, field string) string {
	if nested, ok := m[key].(map[string]any); ok {
		return cell(nested[field])
	}
	return ""
}

// lotsCell shows how many open lots a holding row carries. Sure sends lots
// only on a position's latest row and null elsewhere, which stays blank.
func lotsCell(v any) string {
	if lots, ok := v.([]any); ok {
		return fmt.Sprint(len(lots))
	}
	return ""
}
