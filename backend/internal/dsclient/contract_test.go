package dsclient

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func sampleFinancials() []MonthlyFinancials {
	months := make([]MonthlyFinancials, 6)
	for i := range months {
		months[i] = MonthlyFinancials{PeriodMonth: fmt.Sprintf("2026-%02d", i+4), RevenueKopecks: 9007199254740993}
	}
	return months
}

func TestScoringInputContract(t *testing.T) {
	input := ScoringInput{INN: "7707083893", AmountKopecks: 50000000, TermMonths: 12, Financials: sampleFinancials()}
	if err := input.Validate(); err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"inn", "amount_kopecks", "term_months", "financials", "declared_monthly_debt_payment_kopecks"} {
		if _, ok := object[key]; !ok {
			t.Errorf("missing key %s", key)
		}
	}
	if len(object) != 5 {
		t.Fatalf("unexpected wire fields: %s", data)
	}
	var decoded ScoringInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(input, decoded) {
		t.Fatal("int64 values or fields changed in round trip")
	}
	for _, mutate := range []func(*ScoringInput){
		func(x *ScoringInput) { x.AmountKopecks = 0 },
		func(x *ScoringInput) { x.TermMonths = 0 },
		func(x *ScoringInput) { x.INN = " " },
		func(x *ScoringInput) { x.DeclaredMonthlyDebtPaymentKopecks = -1 },
		func(x *ScoringInput) { x.Financials = x.Financials[:5] },
		func(x *ScoringInput) { x.Financials[0].PeriodMonth = "2026-04-01" },
		func(x *ScoringInput) { x.Financials[0].PeriodMonth = "2026-13" },
		func(x *ScoringInput) { x.Financials[0].PeriodMonth = "0000-01" },
		func(x *ScoringInput) { x.Financials[0].PeriodMonth = x.Financials[1].PeriodMonth },
		func(x *ScoringInput) { x.Financials[0].TaxOutflowKopecks = -1 },
	} {
		candidate := input
		candidate.Financials = append([]MonthlyFinancials(nil), input.Financials...)
		mutate(&candidate)
		if candidate.Validate() == nil {
			t.Errorf("accepted invalid input: %+v", candidate)
		}
	}
}

func TestExtractionRequiredFields(t *testing.T) {
	valid := FinancialExtractionResponse{INN: "7707083893", Financials: sampleFinancials()}
	data, _ := json.Marshal(valid)
	result, err := decodeFinancialExtraction(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.Financials[0].RevenueKopecks != 9007199254740993 {
		t.Fatal("int64 precision lost")
	}
	var object map[string]json.RawMessage
	_ = json.Unmarshal(data, &object)
	for _, key := range []string{"inn", "financials", "declared_monthly_debt_payment_kopecks"} {
		for _, null := range []bool{false, true} {
			c := make(map[string]json.RawMessage)
			for k, v := range object {
				c[k] = v
			}
			if null {
				c[key] = json.RawMessage("null")
			} else {
				delete(c, key)
			}
			bad, _ := json.Marshal(c)
			if _, err := decodeFinancialExtraction(bad); err == nil {
				t.Errorf("accepted missing/null %s", key)
			}
		}
	}
	var months []map[string]json.RawMessage
	_ = json.Unmarshal(object["financials"], &months)
	for _, key := range []string{"period_month", "revenue_kopecks", "operating_inflow_kopecks", "operating_outflow_kopecks", "tax_outflow_kopecks"} {
		original := months[0][key]
		delete(months[0], key)
		object["financials"], _ = json.Marshal(months)
		bad, _ := json.Marshal(object)
		if _, err := decodeFinancialExtraction(bad); err == nil {
			t.Errorf("accepted month missing %s", key)
		}
		months[0][key] = original
	}
	for _, bad := range []string{"null", "[]", "{}", `{"inn":"x","financials":[],"declared_monthly_debt_payment_kopecks":0}`} {
		if _, err := decodeFinancialExtraction([]byte(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}

func TestScoringResponseContract(t *testing.T) {
	for _, score := range []int{0, 39, 40, 65, 69, 70, 100} {
		data := fmt.Sprintf(`{"score":%d,"explanation":{"positive_factors":[],"negative_factors":[]},"model_version":"rules-v1"}`, score)
		result, err := decodeScoring([]byte(data))
		if err != nil || result.Score != score {
			t.Fatalf("valid score %d rejected: %v", score, err)
		}
	}
	for _, bad := range []string{
		`null`, `{}`, `{"score":65}`,
		`{"explanation":{"positive_factors":[],"negative_factors":[]},"model_version":"v1"}`,
		`{"score":null,"explanation":{"positive_factors":[],"negative_factors":[]},"model_version":"v1"}`,
		`{"score":-1,"explanation":{"positive_factors":[],"negative_factors":[]},"model_version":"v1"}`,
		`{"score":101,"explanation":{"positive_factors":[],"negative_factors":[]},"model_version":"v1"}`,
		`{"score":65.5,"explanation":{"positive_factors":[],"negative_factors":[]},"model_version":"v1"}`,
		`{"score":65,"explanation":{"positive_factors":[],"negative_factors":[]},"model_version":" "}`,
		`{"score":65,"explanation":null,"model_version":"v1"}`,
		`{"score":65,"explanation":{"negative_factors":[]},"model_version":"v1"}`,
		`{"score":65,"explanation":{"positive_factors":null,"negative_factors":[]},"model_version":"v1"}`,
		`{"score":65,"explanation":{"positive_factors":[null],"negative_factors":[]},"model_version":"v1"}`,
		`{"score":65,"explanation":{"positive_factors":[{}],"negative_factors":[]},"model_version":"v1"}`,
	} {
		if _, err := decodeScoring([]byte(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}
