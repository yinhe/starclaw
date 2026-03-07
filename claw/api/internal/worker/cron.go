package worker

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronExpr represents a parsed cron expression (5 fields: minute hour dom month dow)
type CronExpr struct {
	Minutes    []int // 0-59
	Hours      []int // 0-23
	DaysOfMonth []int // 1-31
	Months     []int // 1-12
	DaysOfWeek []int // 0-6 (0=Sunday)
}

// ParseCron parses a standard 5-field cron expression
// Supports: *, */N, N, N-M, N,M,O
func ParseCron(expr string) (*CronExpr, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron expression must have 5 fields, got %d", len(fields))
	}

	minutes, err := parseField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("invalid minute field: %v", err)
	}
	hours, err := parseField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("invalid hour field: %v", err)
	}
	dom, err := parseField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("invalid day-of-month field: %v", err)
	}
	months, err := parseField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("invalid month field: %v", err)
	}
	dow, err := parseField(fields[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("invalid day-of-week field: %v", err)
	}

	return &CronExpr{
		Minutes:     minutes,
		Hours:       hours,
		DaysOfMonth: dom,
		Months:      months,
		DaysOfWeek:  dow,
	}, nil
}

// Matches returns true if the given time matches this cron expression
func (c *CronExpr) Matches(t time.Time) bool {
	return contains(c.Minutes, t.Minute()) &&
		contains(c.Hours, t.Hour()) &&
		contains(c.DaysOfMonth, t.Day()) &&
		contains(c.Months, int(t.Month())) &&
		contains(c.DaysOfWeek, int(t.Weekday()))
}

// NextAfter returns the next time after `after` that matches the cron expression.
// Searches up to 366 days ahead.
func (c *CronExpr) NextAfter(after time.Time) time.Time {
	// Start from next minute (truncated to minute)
	t := after.Truncate(time.Minute).Add(time.Minute)
	end := after.Add(366 * 24 * time.Hour)

	for t.Before(end) {
		if c.Matches(t) {
			return t
		}
		// Skip efficiently: if month doesn't match, jump to next month
		if !contains(c.Months, int(t.Month())) {
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
			continue
		}
		// If day doesn't match, jump to next day
		if !contains(c.DaysOfMonth, t.Day()) || !contains(c.DaysOfWeek, int(t.Weekday())) {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
			continue
		}
		// If hour doesn't match, jump to next hour
		if !contains(c.Hours, t.Hour()) {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, t.Location())
			continue
		}
		// Otherwise advance by 1 minute
		t = t.Add(time.Minute)
	}
	return time.Time{} // no match found within 366 days
}

func parseField(field string, min, max int) ([]int, error) {
	if field == "*" {
		return makeRange(min, max), nil
	}

	// Handle */N (step)
	if strings.HasPrefix(field, "*/") {
		step, err := strconv.Atoi(field[2:])
		if err != nil || step <= 0 {
			return nil, fmt.Errorf("invalid step: %s", field)
		}
		var result []int
		for i := min; i <= max; i += step {
			result = append(result, i)
		}
		return result, nil
	}

	// Handle comma-separated values (which may contain ranges)
	var result []int
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			// Range: N-M
			bounds := strings.SplitN(part, "-", 2)
			lo, err1 := strconv.Atoi(bounds[0])
			hi, err2 := strconv.Atoi(bounds[1])
			if err1 != nil || err2 != nil || lo < min || hi > max || lo > hi {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			for i := lo; i <= hi; i++ {
				result = append(result, i)
			}
		} else {
			// Single value
			val, err := strconv.Atoi(part)
			if err != nil || val < min || val > max {
				return nil, fmt.Errorf("invalid value: %s", part)
			}
			result = append(result, val)
		}
	}
	return result, nil
}

func makeRange(min, max int) []int {
	result := make([]int, 0, max-min+1)
	for i := min; i <= max; i++ {
		result = append(result, i)
	}
	return result
}

func contains(vals []int, target int) bool {
	for _, v := range vals {
		if v == target {
			return true
		}
	}
	return false
}
