package root

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/output"
)

// securityPriceImportMaxRows mirrors Security::ManualPriceImport::MAX_ROWS.
const securityPriceImportMaxRows = 1000

type securityPriceRow struct {
	Date     string `json:"date"`
	Price    string `json:"price"`
	Currency string `json:"currency"`
}

// securityPriceRowError has the shape of one entry in Sure's 422 `errors`:
// Index is the 0-based row, nil for a batch-level problem.
type securityPriceRowError struct {
	Index   *int   `json:"index"`
	Message string `json:"message"`
}

type securityPriceRowErrors []securityPriceRowError

func (e securityPriceRowErrors) Error() string {
	msgs := make([]string, len(e))
	for i, re := range e {
		if re.Index == nil {
			msgs[i] = re.Message
		} else {
			msgs[i] = fmt.Sprintf("prices[%d]: %s", *re.Index, re.Message)
		}
	}
	return strings.Join(msgs, "; ")
}

func newSecurityPricesCreateCmd() *cobra.Command {
	var securityID, file string
	var apply bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Import closing prices for an untracked security (self-hosted only; default dry-run, --apply to execute)",
		Long: `Import daily closing prices for an offline security no provider tracks.

Sure accepts this on self-hosted instances only, for a security traded in an
account you can write to (or held in a linked one). Rows replace any price
already stored for the same date; all rows are stored or none are.

--file takes CSV with a header naming date, price and currency columns, or JSON:
an array of {"date","price","currency"} objects or an object with a "prices"
array. Use --file - to read standard input. Up to 1000 rows, ISO dates, unique
dates, price > 0 with at most 4 decimals, one currency per request.`,
		Run: func(cmd *cobra.Command, args []string) {
			payload, err := buildSecurityPricesCreatePayload(securityID, file, cmd.InOrStdin())
			if err != nil {
				var rowErrs securityPriceRowErrors
				if errors.As(err, &rowErrs) {
					output.Fail("validation_failed", "Prices were not sent", map[string]any{"errors": rowErrs})
					return
				}
				output.Fail("validation_failed", err.Error(), nil)
				return
			}
			dispatchWrite(apply, "POST", "/api/v1/security_prices", payload)
		},
	}
	cmd.Flags().StringVar(&securityID, "security-id", "", "security id (required)")
	cmd.Flags().StringVar(&file, "file", "", "CSV or JSON file of prices, - for stdin (required)")
	cmd.Flags().BoolVar(&apply, "apply", false, "execute the import (otherwise dry-run)")
	return cmd
}

func buildSecurityPricesCreatePayload(securityID, file string, stdin io.Reader) (map[string]any, error) {
	if securityID == "" {
		return nil, errors.New("security-id is required")
	}
	if file == "" {
		return nil, errors.New("file is required (use - for stdin)")
	}
	in := stdin
	if file != "-" {
		f, err := os.Open(file)
		if err != nil {
			return nil, fmt.Errorf("file not accessible: %w", err)
		}
		defer f.Close()
		in = f
	}
	rows, err := readSecurityPriceRows(in)
	if err != nil {
		return nil, err
	}
	if err := validateSecurityPriceRows(rows); err != nil {
		return nil, err
	}
	return map[string]any{"security_id": securityID, "prices": rows}, nil
}

// readSecurityPriceRows reads CSV or JSON, told apart by the first
// non-blank character: a CSV header cannot start with '[' or '{'. Values are
// kept as the exact text given, JSON numbers included.
func readSecurityPriceRows(r io.Reader) ([]securityPriceRow, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read prices: %w", err)
	}
	data = bytes.TrimSpace(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")))
	if len(data) == 0 {
		return nil, errors.New("prices input is empty")
	}
	if data[0] == '[' || data[0] == '{' {
		return readSecurityPriceRowsJSON(data)
	}
	return readSecurityPriceRowsCSV(data)
}

var securityPriceColumns = []string{"date", "price", "currency"}

func readSecurityPriceRowsCSV(data []byte) ([]securityPriceRow, error) {
	records, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse CSV: %w", err)
	}
	col := map[string]int{}
	for i, name := range records[0] {
		name = strings.ToLower(strings.TrimSpace(name))
		if !slices.Contains(securityPriceColumns, name) {
			return nil, fmt.Errorf("CSV column %q is not one of %s", name, strings.Join(securityPriceColumns, ", "))
		}
		if _, dup := col[name]; dup {
			return nil, fmt.Errorf("CSV column %q appears more than once", name)
		}
		col[name] = i
	}
	for _, name := range securityPriceColumns {
		if _, ok := col[name]; !ok {
			return nil, fmt.Errorf("CSV header must name the columns %s", strings.Join(securityPriceColumns, ", "))
		}
	}
	rows := make([]securityPriceRow, 0, len(records)-1)
	for _, rec := range records[1:] {
		cell := func(name string) string { return strings.TrimSpace(rec[col[name]]) }
		rows = append(rows, securityPriceRow{Date: cell("date"), Price: cell("price"), Currency: cell("currency")})
	}
	return rows, nil
}

func readSecurityPriceRowsJSON(data []byte) ([]securityPriceRow, error) {
	type rawRow struct {
		Date     json.RawMessage `json:"date"`
		Price    json.RawMessage `json:"price"`
		Currency json.RawMessage `json:"currency"`
	}
	var raw []rawRow
	var target any = &raw
	if data[0] == '{' {
		target = &struct {
			Prices *[]rawRow `json:"prices"`
		}{&raw}
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	if dec.More() {
		return nil, errors.New("parse JSON: unexpected data after the prices")
	}
	if raw == nil {
		return nil, errors.New(`JSON must be an array of prices or an object with a "prices" array`)
	}
	rows := make([]securityPriceRow, len(raw))
	for i, r := range raw {
		var errs [3]error
		rows[i].Date, errs[0] = jsonScalarText(r.Date)
		rows[i].Price, errs[1] = jsonScalarText(r.Price)
		rows[i].Currency, errs[2] = jsonScalarText(r.Currency)
		if err := errors.Join(errs[:]...); err != nil {
			return nil, fmt.Errorf("prices[%d]: %w", i, err)
		}
	}
	return rows, nil
}

// jsonScalarText returns a JSON string's value or a JSON number's literal
// text, so 12.3400 stays "12.3400".
func jsonScalarText(raw json.RawMessage) (string, error) {
	raw = bytes.TrimSpace(raw)
	switch {
	case len(raw) == 0 || string(raw) == "null":
		return "", nil
	case raw[0] == '"':
		var s string
		err := json.Unmarshal(raw, &s)
		return strings.TrimSpace(s), err
	case raw[0] == '-' || (raw[0] >= '0' && raw[0] <= '9'):
		return string(raw), nil
	default:
		return "", fmt.Errorf("value %s must be a string or a number", raw)
	}
}

var isoCalendarDateRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// validateSecurityPriceRows applies the row rules of
// Security::ManualPriceImport that need no server state. Not checked here:
// future dates (Sure's "today" is its own time zone), ISO 4217 membership,
// and which currency the security allows.
func validateSecurityPriceRows(rows []securityPriceRow) error {
	if len(rows) == 0 {
		return securityPriceRowErrors{{Message: "prices must contain at least one row"}}
	}
	if len(rows) > securityPriceImportMaxRows {
		return securityPriceRowErrors{{Message: fmt.Sprintf("a request can carry at most %d prices", securityPriceImportMaxRows)}}
	}

	var errs securityPriceRowErrors
	seenDates := map[string]bool{}
	batchCurrency := ""
	for i, row := range rows {
		currency := strings.ToUpper(strings.TrimSpace(row.Currency))
		message := securityPriceDateError(row.Date)
		if message == "" {
			message = securityPriceError(row.Price)
		}
		if message == "" && currency == "" {
			message = "currency is required"
		}
		if message == "" && seenDates[row.Date] {
			message = "date appears more than once in this batch"
		}
		if message == "" {
			seenDates[row.Date] = true
			if batchCurrency == "" {
				batchCurrency = currency
			} else if currency != batchCurrency {
				message = "every row in one request must use the same currency"
			}
		}
		if message != "" {
			index := i
			errs = append(errs, securityPriceRowError{Index: &index, Message: message})
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func securityPriceDateError(date string) string {
	if !isoCalendarDateRE.MatchString(date) {
		return "date must be an ISO 8601 date"
	}
	if _, err := time.Parse(time.DateOnly, date); err != nil {
		return "date must be an ISO 8601 date"
	}
	return ""
}

func securityPriceError(price string) string {
	switch {
	case !isDecimalText(price):
		return "price must be a number written as plain decimal text"
	case !isPositiveDecimalText(price):
		return "price must be greater than 0"
	case decimalScale(price) > 4:
		return "price must have at most 4 decimal places"
	case decimalIntegerDigits(price) > 15:
		return "price is too large"
	}
	return ""
}
