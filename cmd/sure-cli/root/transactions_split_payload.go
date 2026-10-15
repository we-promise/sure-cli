package root

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"regexp"
	"strings"
)

func readTransactionSplitInput(o transactionSplitOpts, stdin io.Reader) ([]byte, error) {
	switch {
	case o.Splits != "" && o.SplitsFile != "":
		return nil, errors.New("provide only one of splits or splits-file")
	case o.Splits != "":
		return []byte(o.Splits), nil
	case o.SplitsFile == "-":
		return io.ReadAll(stdin)
	case o.SplitsFile != "":
		return os.ReadFile(o.SplitsFile)
	default:
		return nil, errors.New("splits or splits-file is required")
	}
}

// Amounts stay as the text the caller wrote. The server stores four decimal
// places and requires an exact sum, so a float round trip would break it.
var transactionSplitAmountRe = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

var transactionSplitInputKeys = map[string]bool{
	"amount": true, "amount_decimal": true, "name": true,
	"category_id": true, "category": true, "excluded": true,
}

// buildTransactionSplitPayload accepts a bare array of children, or the object that
// "split show" prints (with or without its envelope).
func buildTransactionSplitPayload(raw []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var parsed any
	if err := dec.Decode(&parsed); err != nil {
		return nil, fmt.Errorf("splits is not valid JSON: %w", err)
	}

	if obj, ok := parsed.(map[string]any); ok {
		if data, ok := obj["data"].(map[string]any); ok {
			obj = data
		}
		parsed = obj["children"]
	}
	items, ok := parsed.([]any)
	if !ok || len(items) == 0 {
		return nil, errors.New("splits must be a non-empty JSON array")
	}

	splits := make([]map[string]any, 0, len(items))
	for i, item := range items {
		child, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("split %d must be an object", i+1)
		}
		split, err := buildTransactionSplitChild(child)
		if err != nil {
			return nil, fmt.Errorf("split %d: %w", i+1, err)
		}
		splits = append(splits, split)
	}

	return map[string]any{"split": map[string]any{"splits": splits}}, nil
}

func buildTransactionSplitChild(child map[string]any) (map[string]any, error) {
	// A child copied from "split show" is a whole transaction; its extra
	// fields are ignored. Anything else is checked so a typo is not dropped.
	if _, fromShow := child["id"]; !fromShow {
		for key := range child {
			if !transactionSplitInputKeys[key] {
				return nil, fmt.Errorf("unknown field %q", key)
			}
		}
	}

	if lost := transactionSplitLostFields(child); len(lost) > 0 {
		return nil, fmt.Errorf("has %s, which the split API does not keep on a child and would be lost; remove them from the input to proceed", strings.Join(lost, " and "))
	}

	amount, err := transactionSplitChildAmount(child)
	if err != nil {
		return nil, err
	}
	split := map[string]any{"amount": amount}

	if v, ok := child["name"]; ok && v != nil {
		name, ok := v.(string)
		if !ok {
			return nil, errors.New("name must be a string")
		}
		if name != "" {
			split["name"] = name
		}
	}

	categoryID, err := transactionSplitChildCategoryID(child)
	if err != nil {
		return nil, err
	}
	if categoryID != "" {
		split["category_id"] = categoryID
	}

	if v, ok := child["excluded"]; ok && v != nil {
		excluded, ok := v.(bool)
		if !ok {
			return nil, errors.New("excluded must be true or false")
		}
		split["excluded"] = excluded
	}

	return split, nil
}

// show's "amount" is display-formatted ("$60.00"); amount_decimal is exact.
// When both are plain numbers and differ, either could be the edit, so refuse
// rather than guess.
func transactionSplitChildAmount(child map[string]any) (string, error) {
	exactValue, hasDecimal := child["amount_decimal"]
	shownValue, hasAmount := child["amount"]
	if !hasDecimal && !hasAmount {
		return "", errors.New("amount is required")
	}

	value := exactValue
	if !hasDecimal {
		value = shownValue
	} else if hasAmount {
		shown, exact := transactionSplitAmountText(shownValue), transactionSplitAmountText(exactValue)
		if transactionSplitAmountRe.MatchString(shown) && transactionSplitAmountRe.MatchString(exact) && !transactionSplitSameDecimal(shown, exact) {
			return "", fmt.Errorf("amount (%s) and amount_decimal (%s) disagree; edit amount_decimal, or remove it to use amount", shown, exact)
		}
	}

	amount := transactionSplitAmountText(value)
	if !transactionSplitAmountRe.MatchString(amount) {
		return "", fmt.Errorf("amount must be a plain decimal such as \"12.50\", got %v", value)
	}
	return amount, nil
}

// An explicit category_id wins, null included: that clears the category.
// Otherwise a child copied from "split show" keeps its category.id.
func transactionSplitChildCategoryID(child map[string]any) (string, error) {
	value, hasCategoryID := child["category_id"]
	if category, ok := child["category"].(map[string]any); ok && !hasCategoryID {
		value = category["id"]
	}
	if value == nil {
		return "", nil
	}
	id, ok := value.(string)
	if !ok {
		return "", errors.New("category_id must be a string")
	}
	return id, nil
}

func transactionSplitAmountText(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	}
	return ""
}

func transactionSplitSameDecimal(a, b string) bool {
	x, okX := new(big.Rat).SetString(a)
	y, okY := new(big.Rat).SetString(b)
	return okX && okY && x.Cmp(y) == 0
}

func transactionSplitLostFields(child map[string]any) []string {
	var lost []string
	if tags, ok := child["tags"].([]any); ok && len(tags) > 0 {
		lost = append(lost, "tags")
	}
	if notes, ok := child["notes"].(string); ok && notes != "" {
		lost = append(lost, "notes")
	}
	return lost
}
