package root

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestTradesList_Flags(t *testing.T) {
	cmd := newTradesCmd()

	list, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatalf("find list subcommand: %v", err)
	}

	// Verify expected flags exist
	expectedFlags := []string{"from", "to", "start-date", "end-date", "account", "account-id", "account-ids", "page", "per-page"}
	for _, name := range expectedFlags {
		if list.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q to exist", name)
		}
	}

	// Verify usage contains expected flags
	s := list.Flags().FlagUsages()
	if !strings.Contains(s, "from") {
		t.Fatalf("expected from in usage")
	}
	if !strings.Contains(s, "account-id") {
		t.Fatalf("expected account-id in usage")
	}
}

func TestTradesShow_Args(t *testing.T) {
	cmd := newTradesCmd()

	show, _, err := cmd.Find([]string{"show"})
	if err != nil {
		t.Fatalf("find show subcommand: %v", err)
	}

	// Verify it requires exactly 1 argument
	if show.Args == nil {
		t.Fatal("expected Args validator to be set")
	}
}

func TestBuildTradeCreatePayloadRequiresSecurityIdentifier(t *testing.T) {
	_, err := buildTradeCreatePayload(tradeCreateOpts{
		AccountID: "acc_123",
		Date:      "2026-05-01",
		Type:      "buy",
		Qty:       "1",
		Price:     "10",
	})
	if err == nil {
		t.Fatal("expected missing security identifier error")
	}
}

func TestTradesWrite_FlagsSureNoLongerAccepts(t *testing.T) {
	cmd := newTradesCmd()
	create := findSub(t, cmd, "create")
	update := findSub(t, cmd, "update")
	// Sure dropped category_id from both permit lists, so it would be ignored.
	for _, sub := range []*cobra.Command{create, update} {
		if sub.Flags().Lookup("category-id") != nil {
			t.Fatalf("%s must not expose --category-id", sub.Name())
		}
	}
	// Sure derives the create label from the type; the flag could only repeat it or 422.
	if create.Flags().Lookup("investment-activity-label") != nil {
		t.Fatal("create must not expose --investment-activity-label")
	}
	for _, name := range []string{"fee", "amount", "transfer-account-id"} {
		if create.Flags().Lookup(name) == nil {
			t.Fatalf("create must expose --%s", name)
		}
	}
}

func TestTradesTypeFlagHelpListsSupportedTypes(t *testing.T) {
	cmd := newTradesCmd()
	createUsage := findSub(t, cmd, "create").Flags().Lookup("type").Usage
	for _, typ := range []string{"buy", "sell", "sweep_in", "sweep_out", "reinvestment", "dividend", "interest", "fee", "deposit", "withdrawal"} {
		if !strings.Contains(createUsage, typ) {
			t.Fatalf("create --type usage %q missing %s", createUsage, typ)
		}
	}
	updateUsage := findSub(t, cmd, "update").Flags().Lookup("type").Usage
	for _, typ := range []string{"buy", "sell", "sweep_in", "sweep_out", "reinvestment"} {
		if !strings.Contains(updateUsage, typ) {
			t.Fatalf("update --type usage %q missing %s", updateUsage, typ)
		}
	}
	if strings.Contains(updateUsage, "dividend") {
		t.Fatalf("update --type usage %q must not offer dividend", updateUsage)
	}
}

func TestBuildTradeCreatePayload_PerType(t *testing.T) {
	cases := []struct {
		name string
		opts tradeCreateOpts
		want map[string]any
	}{
		{
			name: "buy keeps exact decimal text and fee",
			opts: tradeCreateOpts{AccountID: "acc", Date: "2026-05-01", Type: "buy", Qty: "0.10000001", Price: "123.4500", Fee: "1.25", SecurityID: "sec"},
			want: map[string]any{"account_id": "acc", "date": "2026-05-01", "type": "buy", "qty": "0.10000001", "price": "123.4500", "fee": "1.25", "security_id": "sec"},
		},
		{
			name: "type is normalized like Sure does",
			opts: tradeCreateOpts{AccountID: "acc", Date: "2026-05-01", Type: " Sweep_Out ", Qty: "1", Price: "1", Ticker: "VMFXX"},
			want: map[string]any{"account_id": "acc", "date": "2026-05-01", "type": "sweep_out", "qty": "1", "price": "1", "ticker": "VMFXX"},
		},
		{
			name: "reinvestment with manual ticker and currency",
			opts: tradeCreateOpts{AccountID: "acc", Date: "2026-05-01", Type: "reinvestment", Qty: "2", Price: "0.5", ManualTicker: "PRIV", Currency: "EUR"},
			want: map[string]any{"account_id": "acc", "date": "2026-05-01", "type": "reinvestment", "qty": "2", "price": "0.5", "manual_ticker": "PRIV", "currency": "EUR"},
		},
		{
			name: "dividend",
			opts: tradeCreateOpts{AccountID: "acc", Date: "2026-05-01", Type: "dividend", Amount: "12.34", SecurityID: "sec"},
			want: map[string]any{"account_id": "acc", "date": "2026-05-01", "type": "dividend", "amount": "12.34", "security_id": "sec"},
		},
		{
			name: "interest without security",
			opts: tradeCreateOpts{AccountID: "acc", Date: "2026-05-01", Type: "interest", Amount: "3"},
			want: map[string]any{"account_id": "acc", "date": "2026-05-01", "type": "interest", "amount": "3"},
		},
		{
			name: "fee with ticker",
			opts: tradeCreateOpts{AccountID: "acc", Date: "2026-05-01", Type: "fee", Amount: "0.99", Ticker: "AAPL"},
			want: map[string]any{"account_id": "acc", "date": "2026-05-01", "type": "fee", "amount": "0.99", "ticker": "AAPL"},
		},
		{
			name: "deposit unlinked",
			opts: tradeCreateOpts{AccountID: "acc", Date: "2026-05-01", Type: "deposit", Amount: "1000"},
			want: map[string]any{"account_id": "acc", "date": "2026-05-01", "type": "deposit", "amount": "1000"},
		},
		{
			name: "withdrawal linked",
			opts: tradeCreateOpts{AccountID: "acc", Date: "2026-05-01", Type: "withdrawal", Amount: "250.00", TransferAccountID: "acc2"},
			want: map[string]any{"account_id": "acc", "date": "2026-05-01", "type": "withdrawal", "amount": "250.00", "transfer_account_id": "acc2"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := buildTradeCreatePayload(tc.opts)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !reflect.DeepEqual(payload, map[string]any{"trade": tc.want}) {
				t.Fatalf("payload = %#v\nwant trade = %#v", payload, tc.want)
			}
		})
	}
}

func TestBuildTradeCreatePayload_Rejects(t *testing.T) {
	base := func(typ string) tradeCreateOpts {
		return tradeCreateOpts{AccountID: "acc", Date: "2026-05-01", Type: typ}
	}
	with := func(o tradeCreateOpts, f func(*tradeCreateOpts)) tradeCreateOpts { f(&o); return o }
	buy := with(base("buy"), func(o *tradeCreateOpts) { o.Qty, o.Price, o.SecurityID = "1", "10", "sec" })

	cases := []struct {
		name    string
		opts    tradeCreateOpts
		wantErr string
	}{
		{"missing account", with(buy, func(o *tradeCreateOpts) { o.AccountID = "" }), "account-id is required"},
		{"missing date", with(buy, func(o *tradeCreateOpts) { o.Date = "" }), "date is required"},
		{"US-style date", with(buy, func(o *tradeCreateOpts) { o.Date = "05/01/2026" }), "invalid date"},
		{"unknown type", base("split"), "type must be one of"},
		{"missing type", base(""), "type must be one of"},
		{"buy without security", with(buy, func(o *tradeCreateOpts) { o.SecurityID = "" }), "one of security-id, ticker, or manual-ticker is required"},
		{"buy with two security identifiers", with(buy, func(o *tradeCreateOpts) { o.Ticker = "AAPL" }), "only one of security-id, ticker, or manual-ticker"},
		{"buy without qty", with(buy, func(o *tradeCreateOpts) { o.Qty = "" }), "qty is required"},
		{"buy without price", with(buy, func(o *tradeCreateOpts) { o.Price = "" }), "price is required"},
		{"buy zero qty", with(buy, func(o *tradeCreateOpts) { o.Qty = "0.000" }), "qty must be a positive decimal"},
		{"sell negative price", with(buy, func(o *tradeCreateOpts) { o.Type, o.Price = "sell", "-1" }), "price must be a positive decimal"},
		{"buy non-numeric qty", with(buy, func(o *tradeCreateOpts) { o.Qty = "1abc" }), "qty must be a positive decimal"},
		{"buy non-numeric fee", with(buy, func(o *tradeCreateOpts) { o.Fee = "abc" }), "fee must be a decimal number of 0 or more"},
		{"buy negative fee", with(buy, func(o *tradeCreateOpts) { o.Fee = "-1" }), "fee must be a decimal number of 0 or more"},
		{"buy fee beyond 4 decimals", with(buy, func(o *tradeCreateOpts) { o.Fee = "0.12345" }), "fee has more than 4 decimal places"},
		{"buy qty beyond 18 decimals", with(buy, func(o *tradeCreateOpts) { o.Qty = "0.0000000000000000001" }), "qty has more than 18 decimal places"},
		{"buy price beyond 10 decimals", with(buy, func(o *tradeCreateOpts) { o.Price = "0.00000000001" }), "price has more than 10 decimal places"},
		{"buy price too large", with(buy, func(o *tradeCreateOpts) { o.Price = "1234567890" }), "price has more than 9 digits before the decimal point"},
		{"buy trailing point", with(buy, func(o *tradeCreateOpts) { o.Qty = "3." }), "qty must be a positive decimal"},
		{"dividend amount beyond 4 decimals", with(base("dividend"), func(o *tradeCreateOpts) { o.SecurityID, o.Amount = "sec", "1.00001" }), "amount has more than 4 decimal places"},
		{"buy with amount", with(buy, func(o *tradeCreateOpts) { o.Amount = "10" }), "amount does not apply to buy"},
		{"buy with transfer account", with(buy, func(o *tradeCreateOpts) { o.TransferAccountID = "acc2" }), "transfer-account-id does not apply to buy"},
		{"dividend without security", with(base("dividend"), func(o *tradeCreateOpts) { o.Amount = "1" }), "one of security-id, ticker, or manual-ticker is required"},
		{"dividend without amount", with(base("dividend"), func(o *tradeCreateOpts) { o.SecurityID = "sec" }), "amount is required"},
		{"dividend zero amount", with(base("dividend"), func(o *tradeCreateOpts) { o.SecurityID, o.Amount = "sec", "0" }), "amount must be a positive decimal"},
		{"dividend with qty", with(base("dividend"), func(o *tradeCreateOpts) { o.SecurityID, o.Amount, o.Qty = "sec", "1", "2" }), "qty does not apply to dividend"},
		{"interest without amount", base("interest"), "amount is required"},
		{"fee with fee", with(base("fee"), func(o *tradeCreateOpts) { o.Amount, o.Fee = "1", "1" }), "fee does not apply to fee"},
		{"interest with price", with(base("interest"), func(o *tradeCreateOpts) { o.Amount, o.Price = "1", "1" }), "price does not apply to interest"},
		{"interest with transfer account", with(base("interest"), func(o *tradeCreateOpts) { o.Amount, o.TransferAccountID = "1", "x" }), "transfer-account-id does not apply to interest"},
		{"deposit without amount", base("deposit"), "amount is required"},
		{"withdrawal negative amount", with(base("withdrawal"), func(o *tradeCreateOpts) { o.Amount = "-5" }), "amount must be a positive decimal"},
		{"deposit with security", with(base("deposit"), func(o *tradeCreateOpts) { o.Amount, o.Ticker = "1", "AAPL" }), "ticker does not apply to deposit"},
		{"deposit with qty", with(base("deposit"), func(o *tradeCreateOpts) { o.Amount, o.Qty = "1", "1" }), "qty does not apply to deposit"},
		{"transfer deposit with currency", with(base("deposit"), func(o *tradeCreateOpts) { o.Amount, o.TransferAccountID, o.Currency = "1", "acc2", "USD" }), "currency does not apply to a deposit with transfer-account-id"},
		{"transfer withdrawal with currency", with(base("withdrawal"), func(o *tradeCreateOpts) { o.Amount, o.TransferAccountID, o.Currency = "1", "acc2", "USD" }), "currency does not apply to a withdrawal with transfer-account-id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildTradeCreatePayload(tc.opts)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestBuildTradeUpdatePayload_TypeAndLabel(t *testing.T) {
	for typ, label := range map[string]string{"buy": "Buy", "sell": "Sell", "sweep_in": "Sweep In", "sweep_out": "Sweep Out", "reinvestment": "Reinvestment"} {
		payload, err := buildTradeUpdatePayload(tradeUpdateOpts{Type: typ, InvestmentActivityLabel: label, Qty: "1"})
		if err != nil {
			t.Fatalf("%s/%s: unexpected err: %v", typ, label, err)
		}
		trade := payload["trade"].(map[string]any)
		if trade["type"] != typ || trade["investment_activity_label"] != label {
			t.Fatalf("unexpected trade payload: %#v", trade)
		}
	}

	if _, err := buildTradeUpdatePayload(tradeUpdateOpts{Type: "sell", InvestmentActivityLabel: "Buy"}); err == nil ||
		!strings.Contains(err.Error(), `investment-activity-label must be "Sell" for type sell`) {
		t.Fatalf("expected label/type mismatch error, got %v", err)
	}
	if _, err := buildTradeUpdatePayload(tradeUpdateOpts{Type: "dividend"}); err == nil ||
		!strings.Contains(err.Error(), "type must be one of buy, sell, sweep_in, sweep_out, reinvestment") {
		t.Fatalf("expected unsupported update type error, got %v", err)
	}

	// Without --type Sure checks the label against its own list; pass it through.
	payload, err := buildTradeUpdatePayload(tradeUpdateOpts{InvestmentActivityLabel: "Contribution"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if payload["trade"].(map[string]any)["investment_activity_label"] != "Contribution" {
		t.Fatalf("label not passed through: %#v", payload)
	}
}

func TestBuildTradeUpdatePayload(t *testing.T) {
	payload, err := buildTradeUpdatePayload(tradeUpdateOpts{Qty: "2", Price: "11.50", Type: "sell"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	trade := payload["trade"].(map[string]any)
	if trade["qty"] != "2" || trade["price"] != "11.50" || trade["type"] != "sell" {
		t.Fatalf("unexpected trade payload: %#v", trade)
	}
}

func TestBuildTradeUpdatePayload_Rejects(t *testing.T) {
	// Sure's TradesController#update reads type and nature only to sign qty
	// and price, recomputes amount from them, and casts qty/price with to_d.
	cases := []struct {
		name    string
		opts    tradeUpdateOpts
		wantErr string
	}{
		{"type without qty or price", tradeUpdateOpts{Type: "sell"}, "type and nature only apply with qty or price"},
		{"nature without qty or price", tradeUpdateOpts{Nature: "inflow", Name: "x"}, "type and nature only apply with qty or price"},
		{"type and nature", tradeUpdateOpts{Type: "buy", Nature: "inflow", Qty: "1"}, "use either type or nature, not both"},
		{"unknown nature", tradeUpdateOpts{Nature: "income", Qty: "1"}, "nature must be inflow or outflow"},
		{"amount with qty", tradeUpdateOpts{Amount: "10", Qty: "1"}, "amount cannot be combined with qty or price"},
		{"amount with price", tradeUpdateOpts{Amount: "10", Price: "1"}, "amount cannot be combined with qty or price"},
		{"non-numeric qty", tradeUpdateOpts{Qty: "abc"}, "qty must be a positive decimal"},
		{"zero price", tradeUpdateOpts{Price: "0"}, "price must be a positive decimal"},
		{"non-numeric amount", tradeUpdateOpts{Amount: "1,5"}, "amount must be a decimal number"},
		{"amount beyond 4 decimals", tradeUpdateOpts{Amount: "1.00001"}, "amount has more than 4 decimal places"},
		{"US-style date", tradeUpdateOpts{Date: "05/01/2026"}, "invalid date"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildTradeUpdatePayload(tc.opts)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}

	// The entry amount is signed (positive = outflow), so a negative one is valid.
	payload, err := buildTradeUpdatePayload(tradeUpdateOpts{Amount: "-25.50"})
	if err != nil || payload["trade"].(map[string]any)["amount"] != "-25.50" {
		t.Fatalf("signed amount: payload=%v err=%v", payload, err)
	}
	payload, err = buildTradeUpdatePayload(tradeUpdateOpts{Nature: "inflow", Price: "12"})
	if err != nil || payload["trade"].(map[string]any)["nature"] != "inflow" {
		t.Fatalf("nature with price: payload=%v err=%v", payload, err)
	}
}
