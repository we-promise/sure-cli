package output

import (
	"fmt"
	"os"

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
	if accounts, ok := m["accounts"].([]any); ok {
		tw := table.NewWriter()
		tw.SetOutputMirror(os.Stdout)
		tw.AppendHeader(table.Row{"id", "name", "type", "currency", "balance", "classification"})
		for _, a := range accounts {
			rowm, _ := a.(map[string]any)
			tw.AppendRow(table.Row{
				fmt.Sprint(rowm["id"]),
				fmt.Sprint(rowm["name"]),
				fmt.Sprint(rowm["account_type"]),
				fmt.Sprint(rowm["currency"]),
				fmt.Sprint(rowm["balance"]),
				fmt.Sprint(rowm["classification"]),
			})
		}
		tw.Render()
		return true
	}

	// Transactions list
	if txs, ok := m["transactions"].([]any); ok {
		tw := table.NewWriter()
		tw.SetOutputMirror(os.Stdout)
		tw.AppendHeader(table.Row{"id", "date", "name", "classification", "amount", "account"})
		for _, tx := range txs {
			rowm, _ := tx.(map[string]any)
			acct := ""
			if am, ok := rowm["account"].(map[string]any); ok {
				acct = fmt.Sprint(am["name"])
			}
			tw.AppendRow(table.Row{
				fmt.Sprint(rowm["id"]),
				fmt.Sprint(rowm["date"]),
				fmt.Sprint(rowm["name"]),
				fmt.Sprint(rowm["classification"]),
				fmt.Sprint(rowm["amount"]),
				acct,
			})
		}
		tw.Render()
		return true
	}

	// Holdings list (Sure api/v1/holdings/_holding)
	if holdings, ok := m["holdings"].([]any); ok {
		tw := table.NewWriter()
		tw.SetOutputMirror(os.Stdout)
		tw.AppendHeader(table.Row{"id", "date", "ticker", "security", "qty", "price", "amount", "avg_cost", "lots", "account"})
		for _, h := range holdings {
			rowm, _ := h.(map[string]any)
			tw.AppendRow(table.Row{
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
			})
		}
		tw.Render()
		return true
	}

	// Trades list (Sure api/v1/trades/_trade)
	if trades, ok := m["trades"].([]any); ok {
		tw := table.NewWriter()
		tw.SetOutputMirror(os.Stdout)
		tw.AppendHeader(table.Row{"id", "date", "ticker", "activity", "qty", "price", "amount", "lot_source", "account"})
		for _, tr := range trades {
			rowm, _ := tr.(map[string]any)
			tw.AppendRow(table.Row{
				cell(rowm["id"]),
				cell(rowm["date"]),
				nestedCell(rowm, "security", "ticker"),
				cell(rowm["investment_activity_label"]),
				cell(rowm["qty"]),
				cell(rowm["price"]),
				cell(rowm["amount"]),
				cell(rowm["lot_source_trade_id"]),
				nestedCell(rowm, "account", "name"),
			})
		}
		tw.Render()
		return true
	}

	return false
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
