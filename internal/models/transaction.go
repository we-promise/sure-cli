package models

import "time"

// Transaction is a minimal, agent-friendly view of a Sure transaction.
// It is shared across packages (api, insights, commands) to avoid map[string]any plumbing.
type Transaction struct {
	ID             string
	EntryID        string
	Name           string
	Classification string // income|expense
	Kind           string // standard, funds_movement, cc_payment, loan_payment, one_time, investment_contribution, ...
	// Amount is signed for agents: income positive, expense negative.
	// See ResolveSignedAmount for how it is derived from the API fields.
	Amount float64
	// AmountDecimal is Amount as exact decimal text (same sign), e.g. "-12.3456".
	AmountDecimal string
	// AmountText is Sure's localized, rounded display string, e.g. "€12.35".
	AmountText   string
	Currency     string
	Date         time.Time
	AccountID    string
	AccountName  string
	MerchantID   string
	MerchantName string
	CategoryName string
	CategoryID   string
	ExternalID   string
	Source       string
	UserModified bool
	// Excluded rows are left out of Sure's budget and report totals.
	Excluded bool
	// SplitParent rows carry no amount of their own: their split children do.
	SplitParent bool
}

// reportExcludedKinds mirrors Sure's Transaction::BUDGET_EXCLUDED_KINDS:
// transfers between the family's own accounts, credit-card payments and
// one-off entries never count in budget or income-statement totals.
var reportExcludedKinds = map[string]bool{
	"funds_movement": true,
	"cc_payment":     true,
	"one_time":       true,
}

// Reportable reports whether the row itself may count toward spend/income
// totals: not excluded, not a split parent (its children carry the amount)
// and not a budget-excluded kind. Which accounts count is decided by the
// caller from the accounts list (see api.FetchTransactionsWindow).
//
// It approximates Sure's income statement (IncomeStatement::Totals): the
// transactions API exposes neither the pending flag nor the investment
// activity label, so pending rows and the Transfer, Sweep In, Sweep Out and
// Exchange rows Sure leaves out are counted here.
func (t Transaction) Reportable() bool {
	return !t.Excluded && !t.SplitParent && !reportExcludedKinds[t.Kind]
}

// ReportClassification is how Sure's income statement classifies the row
// (IncomeStatement::ScopedTransactionsQuery#classification_sql): loan
// payments are expenses whatever their sign, and so are investment
// contributions under Sure's default investment_contributions_as_spending.
// A family that turned that setting off reports contributions as invested,
// not spent; the API does not expose the setting, so the CLI assumes the
// default.
func (t Transaction) ReportClassification() string {
	switch t.Kind {
	case "loan_payment", "investment_contribution":
		return "expense"
	}
	return t.Classification
}
