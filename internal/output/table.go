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

	return false
}
