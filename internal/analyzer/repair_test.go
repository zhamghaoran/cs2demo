package analyzer

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRepairTruncatedJSON(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			"complete json untouched",
			`{"a":1,"b":[2,3]}`,
			`{"a":1,"b":[2,3]}`,
		},
		{
			"truncated mid-string",
			`{"a": "hel`,
			`{"a": "hel"}`,
		},
		{
			"truncated mid-array",
			`{"a": [1, 2`,
			`{"a": [1, 2]}`,
		},
		{
			"truncated mid-nested",
			`{"a": {"b": [1`,
			`{"a": {"b": [1]}}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := repairTruncatedJSON(c.in)
			if got != c.want {
				t.Errorf("repair(%q)\n  got=  %q\n  want= %q", c.in, got, c.want)
			}
			if !strings.HasSuffix(c.want, "}") {
				return
			}
			var v map[string]any
			if err := json.Unmarshal([]byte(got), &v); err != nil {
				t.Errorf("repaired %q is not valid JSON: %v", got, err)
			}
		})
	}
}

func TestParseLLMReportWithTruncation(t *testing.T) {
	truncated := "```json\n" + `{
  "overall_score": 72,
  "verdict": "测试截断处理",
  "strengths": [{"title": "枪法", "detail": "稳"`
	r, err := parseLLMReport(truncated)
	if err != nil {
		t.Fatalf("parseLLMReport with truncation failed: %v", err)
	}
	if r.OverallScore != 72 {
		t.Errorf("expected score=72, got %d", r.OverallScore)
	}
	if r.Verdict != "测试截断处理" {
		t.Errorf("expected verdict, got %q", r.Verdict)
	}
}
