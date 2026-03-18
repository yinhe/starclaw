package webhook

import (
	"testing"
)

func TestCompareValues_NumericGt(t *testing.T) {
	if !compareValues(0.5, "gt", 0.1) {
		t.Error("0.5 > 0.1 should be true")
	}
	if compareValues(0.1, "gt", 0.5) {
		t.Error("0.1 > 0.5 should be false")
	}
}

func TestCompareValues_NumericGte(t *testing.T) {
	if !compareValues(0.5, "gte", 0.5) {
		t.Error("0.5 >= 0.5 should be true")
	}
	if !compareValues(1.0, "gte", 0.5) {
		t.Error("1.0 >= 0.5 should be true")
	}
	if compareValues(0.1, "gte", 0.5) {
		t.Error("0.1 >= 0.5 should be false")
	}
}

func TestCompareValues_NumericLt(t *testing.T) {
	if !compareValues(0.1, "lt", 0.5) {
		t.Error("0.1 < 0.5 should be true")
	}
	if compareValues(0.5, "lt", 0.1) {
		t.Error("0.5 < 0.1 should be false")
	}
}

func TestCompareValues_NumericLte(t *testing.T) {
	if !compareValues(0.5, "lte", 0.5) {
		t.Error("0.5 <= 0.5 should be true")
	}
	if compareValues(1.0, "lte", 0.5) {
		t.Error("1.0 <= 0.5 should be false")
	}
}

func TestCompareValues_NumericEq(t *testing.T) {
	if !compareValues(42.0, "eq", 42.0) {
		t.Error("42 == 42 should be true")
	}
	if compareValues(42.0, "eq", 43.0) {
		t.Error("42 == 43 should be false")
	}
}

func TestCompareValues_NumericNeq(t *testing.T) {
	if !compareValues(42.0, "neq", 43.0) {
		t.Error("42 != 43 should be true")
	}
	if compareValues(42.0, "neq", 42.0) {
		t.Error("42 != 42 should be false")
	}
}

func TestCompareValues_IntTypes(t *testing.T) {
	// int
	if !compareValues(10, "gt", 5) {
		t.Error("int 10 > 5 should be true")
	}
	// int64
	if !compareValues(int64(100), "lt", int64(200)) {
		t.Error("int64 100 < 200 should be true")
	}
	// float32
	if !compareValues(float32(1.5), "eq", float32(1.5)) {
		t.Error("float32 1.5 == 1.5 should be true")
	}
}

func TestCompareValues_StringEq(t *testing.T) {
	if !compareValues("hello", "eq", "hello") {
		t.Error(`"hello" == "hello" should be true`)
	}
	if compareValues("hello", "eq", "world") {
		t.Error(`"hello" == "world" should be false`)
	}
}

func TestCompareValues_StringNeq(t *testing.T) {
	if !compareValues("hello", "neq", "world") {
		t.Error(`"hello" != "world" should be true`)
	}
}

func TestCompareValues_StringContains(t *testing.T) {
	if !compareValues("hello world", "contains", "world") {
		t.Error(`"hello world" contains "world" should be true`)
	}
	if compareValues("hello", "contains", "xyz") {
		t.Error(`"hello" contains "xyz" should be false`)
	}
}

func TestCompareValues_StringNotContains(t *testing.T) {
	if !compareValues("hello", "not_contains", "xyz") {
		t.Error(`"hello" not_contains "xyz" should be true`)
	}
	if compareValues("hello world", "not_contains", "world") {
		t.Error(`"hello world" not_contains "world" should be false`)
	}
}

func TestCompareValues_StringStartsWith(t *testing.T) {
	if !compareValues("hello world", "starts_with", "hello") {
		t.Error(`"hello world" starts_with "hello" should be true`)
	}
	if compareValues("hello", "starts_with", "world") {
		t.Error(`"hello" starts_with "world" should be false`)
	}
}

func TestCompareValues_StringEndsWith(t *testing.T) {
	if !compareValues("hello world", "ends_with", "world") {
		t.Error(`"hello world" ends_with "world" should be true`)
	}
	if compareValues("hello", "ends_with", "world") {
		t.Error(`"hello" ends_with "world" should be false`)
	}
}

func TestCompareValues_UnknownOperator(t *testing.T) {
	if compareValues("a", "unknown_op", "b") {
		t.Error("unknown operator should return false")
	}
}

func TestToFloat64(t *testing.T) {
	cases := []struct {
		input    interface{}
		expected float64
		ok       bool
	}{
		{float64(1.5), 1.5, true},
		{float32(2.5), 2.5, true},
		{int(42), 42, true},
		{int64(100), 100, true},
		{"not a number", 0, false},
		{nil, 0, false},
		{true, 0, false},
	}

	for _, tc := range cases {
		got, ok := toFloat64(tc.input)
		if ok != tc.ok {
			t.Errorf("toFloat64(%v): expected ok=%v, got ok=%v", tc.input, tc.ok, ok)
		}
		if ok && got != tc.expected {
			t.Errorf("toFloat64(%v) = %f, want %f", tc.input, got, tc.expected)
		}
	}
}

func TestEvaluateCondition_EmptyCondition(t *testing.T) {
	e := NewEngine(nil)

	event := Event{Type: "test", Data: map[string]interface{}{"x": 1.0}}

	// Empty conditions should match
	for _, cond := range []string{"", "{}", "null"} {
		if !e.evaluateCondition(cond, event) {
			t.Errorf("empty condition %q should match", cond)
		}
	}
}

func TestEvaluateCondition_FieldMatch(t *testing.T) {
	e := NewEngine(nil)

	event := Event{
		Type: "agent.error",
		Data: map[string]interface{}{
			"error_rate": 0.5,
			"agent_name": "test-agent",
		},
	}

	// Numeric: error_rate > 0.1
	if !e.evaluateCondition(`{"field":"error_rate","operator":"gt","value":0.1}`, event) {
		t.Error("error_rate 0.5 > 0.1 should match")
	}

	// Numeric: error_rate < 0.1
	if e.evaluateCondition(`{"field":"error_rate","operator":"lt","value":0.1}`, event) {
		t.Error("error_rate 0.5 < 0.1 should NOT match")
	}

	// String: agent_name contains "test"
	if !e.evaluateCondition(`{"field":"agent_name","operator":"contains","value":"test"}`, event) {
		t.Error("agent_name contains 'test' should match")
	}
}

func TestEvaluateCondition_MissingField(t *testing.T) {
	e := NewEngine(nil)

	event := Event{Type: "test", Data: map[string]interface{}{"x": 1.0}}

	// Field not in event data → no match
	if e.evaluateCondition(`{"field":"missing_field","operator":"gt","value":0}`, event) {
		t.Error("missing field should NOT match")
	}
}

func TestEvaluateCondition_MalformedJSON(t *testing.T) {
	e := NewEngine(nil)
	event := Event{Type: "test", Data: map[string]interface{}{}}

	// Malformed JSON → fail open (match)
	if !e.evaluateCondition(`{bad json`, event) {
		t.Error("malformed condition should match (fail open)")
	}
}

func TestEvent_Struct(t *testing.T) {
	e := Event{
		Type:   "agent.error",
		Source: "test",
		Data:   map[string]interface{}{"error_rate": 0.5},
	}
	if e.Type != "agent.error" {
		t.Errorf("expected agent.error, got %s", e.Type)
	}
	if e.Data["error_rate"] != 0.5 {
		t.Error("expected error_rate=0.5")
	}
}

func TestEngine_New(t *testing.T) {
	e := NewEngine(nil)
	if e == nil {
		t.Fatal("expected non-nil engine")
	}
	if e.httpC == nil {
		t.Error("expected http client")
	}
	if e.eventCh == nil {
		t.Error("expected event channel")
	}
}

func TestEngine_Emit_NoBlock(t *testing.T) {
	e := NewEngine(nil)
	// Should not block even without Start()
	e.Emit(Event{Type: "test.event", Data: map[string]interface{}{"x": 1}})
	// Channel should have the event
	select {
	case ev := <-e.eventCh:
		if ev.Type != "test.event" {
			t.Errorf("expected test.event, got %s", ev.Type)
		}
	default:
		t.Error("expected event in channel")
	}
}
