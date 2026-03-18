package agent

import (
	"encoding/json"
	"testing"
	"time"
)

func TestGoalStatus_Constants(t *testing.T) {
	statuses := []GoalStatus{GoalPending, GoalActive, GoalCompleted, GoalFailed, GoalCancelled}
	expected := []string{"pending", "active", "completed", "failed", "cancelled"}

	for i, s := range statuses {
		if string(s) != expected[i] {
			t.Errorf("expected %q, got %q", expected[i], s)
		}
	}
}

func TestGetGoalDecompositionPrompt(t *testing.T) {
	prompt := GetGoalDecompositionPrompt()
	if prompt == "" {
		t.Error("expected non-empty prompt")
	}
	// Should mention key action types
	for _, keyword := range []string{"think", "tool_call", "sub_goal", "decide", "report"} {
		if !containsStr(prompt, keyword) {
			t.Errorf("prompt should mention %q", keyword)
		}
	}
}

func TestParseDecomposedPlan_Valid(t *testing.T) {
	input := `{
		"reasoning": "Break down the research task",
		"steps": [
			{"action": "think", "description": "Analyze the question"},
			{"action": "tool_call", "description": "Search the web", "tool": "web_search"},
			{"action": "report", "description": "Summarize findings"}
		]
	}`

	plan, err := ParseDecomposedPlan(input)
	if err != nil {
		t.Fatalf("ParseDecomposedPlan: %v", err)
	}
	if plan.Reasoning != "Break down the research task" {
		t.Errorf("unexpected reasoning: %s", plan.Reasoning)
	}
	if len(plan.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Action != "think" {
		t.Errorf("step 0 action: %s", plan.Steps[0].Action)
	}
	if plan.Steps[1].Tool != "web_search" {
		t.Errorf("step 1 tool: %s", plan.Steps[1].Tool)
	}
}

func TestParseDecomposedPlan_WithPreamble(t *testing.T) {
	// LLM often adds text before JSON
	input := `Here is my plan:
	
	{"reasoning":"test","steps":[{"action":"think","description":"step 1"}]}`

	plan, err := ParseDecomposedPlan(input)
	if err != nil {
		t.Fatalf("ParseDecomposedPlan with preamble: %v", err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}
}

func TestParseDecomposedPlan_Invalid(t *testing.T) {
	_, err := ParseDecomposedPlan("not json at all")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseDecomposedPlan_DependsOn(t *testing.T) {
	input := `{"reasoning":"x","steps":[
		{"action":"think","description":"a"},
		{"action":"tool_call","description":"b","depends_on":[0]},
		{"action":"report","description":"c","depends_on":[0,1]}
	]}`

	plan, err := ParseDecomposedPlan(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps[2].DependsOn) != 2 {
		t.Errorf("step 2 should have 2 deps, got %d", len(plan.Steps[2].DependsOn))
	}
	if plan.Steps[2].DependsOn[0] != 0 || plan.Steps[2].DependsOn[1] != 1 {
		t.Errorf("unexpected deps: %v", plan.Steps[2].DependsOn)
	}
}

func TestScheduleEvaluator_PastTime(t *testing.T) {
	past := time.Now().Add(-time.Hour).Format(time.RFC3339)
	configJSON, _ := json.Marshal(map[string]string{"at": past})
	goal := Goal{TriggerConfig: string(configJSON)}

	if !scheduleEvaluator(goal) {
		t.Error("past schedule should trigger")
	}
}

func TestScheduleEvaluator_FutureTime(t *testing.T) {
	future := time.Now().Add(time.Hour).Format(time.RFC3339)
	configJSON, _ := json.Marshal(map[string]string{"at": future})
	goal := Goal{TriggerConfig: string(configJSON)}

	if scheduleEvaluator(goal) {
		t.Error("future schedule should NOT trigger")
	}
}

func TestScheduleEvaluator_EmptyConfig(t *testing.T) {
	goal := Goal{TriggerConfig: ""}
	if scheduleEvaluator(goal) {
		t.Error("empty config should NOT trigger")
	}
}

func TestScheduleEvaluator_InvalidJSON(t *testing.T) {
	goal := Goal{TriggerConfig: "not json"}
	if scheduleEvaluator(goal) {
		t.Error("invalid JSON should NOT trigger")
	}
}

func TestScheduleEvaluator_InvalidTimestamp(t *testing.T) {
	goal := Goal{TriggerConfig: `{"at":"not-a-time"}`}
	if scheduleEvaluator(goal) {
		t.Error("invalid timestamp should NOT trigger")
	}
}

func TestFindJSONStart(t *testing.T) {
	cases := []struct {
		input    string
		expected int
	}{
		{`{"a":1}`, 0},
		{`prefix {"a":1}`, 7},
		{`no json here`, -1},
		{``, -1},
	}
	for _, tc := range cases {
		got := findJSONStart(tc.input)
		if got != tc.expected {
			t.Errorf("findJSONStart(%q) = %d, want %d", tc.input, got, tc.expected)
		}
	}
}

func TestFindJSONEnd(t *testing.T) {
	cases := []struct {
		input    string
		expected int
	}{
		{`{"a":1}`, 6},
		{`{"a":{"b":2}}`, 12},
		{`{"a":1} extra`, 6},
		{`no braces`, -1},
		{`{unclosed`, -1},
	}
	for _, tc := range cases {
		got := findJSONEnd(tc.input)
		if got != tc.expected {
			t.Errorf("findJSONEnd(%q) = %d, want %d", tc.input, got, tc.expected)
		}
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsLoop(s, substr))
}

func containsLoop(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
