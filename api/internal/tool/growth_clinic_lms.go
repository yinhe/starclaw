package tool

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// ═══════════════════════════════════════════════════════════════
// LMS Reference Data (WHO + China National Standards)
// Simplified: interpolated from key age points
// ═══════════════════════════════════════════════════════════════

// getHeightLMS returns L, M, S for height-for-age.
// Source: WHO Child Growth Standards + 中国7岁以下儿童生长发育参照标准(2009)
func getHeightLMS(gender string, ageMonths float64) (l, m, s float64) {
	// Key reference points: {ageMonths, L, M, S}
	var table []lmsRow
	if gender == "male" {
		table = maleHeightLMS
	} else {
		table = femaleHeightLMS
	}
	return interpolateLMS(table, ageMonths)
}

func getWeightLMS(gender string, ageMonths float64) (l, m, s float64) {
	var table []lmsRow
	if gender == "male" {
		table = maleWeightLMS
	} else {
		table = femaleWeightLMS
	}
	return interpolateLMS(table, ageMonths)
}

func getBMI_LMS(gender string, ageMonths float64) (l, m, s float64) {
	var table []lmsRow
	if gender == "male" {
		table = maleBMI_LMS
	} else {
		table = femaleBMI_LMS
	}
	return interpolateLMS(table, ageMonths)
}

type lmsRow struct {
	age     float64 // months
	l, m, s float64
}

func interpolateLMS(table []lmsRow, age float64) (l, m, s float64) {
	if len(table) == 0 {
		return 1, 100, 0.04
	}
	if age <= table[0].age {
		return table[0].l, table[0].m, table[0].s
	}
	if age >= table[len(table)-1].age {
		return table[len(table)-1].l, table[len(table)-1].m, table[len(table)-1].s
	}
	for i := 1; i < len(table); i++ {
		if age <= table[i].age {
			t := (age - table[i-1].age) / (table[i].age - table[i-1].age)
			l = table[i-1].l + t*(table[i].l-table[i-1].l)
			m = table[i-1].m + t*(table[i].m-table[i-1].m)
			s = table[i-1].s + t*(table[i].s-table[i-1].s)
			return
		}
	}
	last := table[len(table)-1]
	return last.l, last.m, last.s
}

// WHO + China merged LMS tables (key age points, linearly interpolated)
// Male height-for-age
var maleHeightLMS = []lmsRow{
	{0, 1, 49.9, 0.0379}, {1, 1, 54.7, 0.0364}, {3, 1, 61.4, 0.0349},
	{6, 1, 67.6, 0.0338}, {9, 1, 72.0, 0.0333}, {12, 1, 75.7, 0.0330},
	{18, 1, 82.3, 0.0328}, {24, 1, 87.8, 0.0325}, {36, 1, 96.1, 0.0321},
	{48, 1, 103.3, 0.0319}, {60, 1, 110.0, 0.0318}, {72, 1, 116.0, 0.0418},
	{84, 1, 121.7, 0.0418}, {96, 1, 127.3, 0.0420}, {108, 1, 132.6, 0.0424},
	{120, 1, 137.8, 0.0430}, {132, 1, 143.1, 0.0440}, {144, 1, 149.1, 0.0450},
	{156, 1, 156.0, 0.0450}, {168, 1, 163.2, 0.0440}, {180, 1, 168.5, 0.0430},
	{192, 1, 171.0, 0.0420}, {204, 1, 172.1, 0.0410}, {216, 1, 172.1, 0.0400},
}

// Female height-for-age
var femaleHeightLMS = []lmsRow{
	{0, 1, 49.1, 0.0379}, {1, 1, 53.7, 0.0364}, {3, 1, 59.8, 0.0349},
	{6, 1, 65.7, 0.0338}, {9, 1, 70.1, 0.0333}, {12, 1, 74.0, 0.0330},
	{18, 1, 80.7, 0.0328}, {24, 1, 86.4, 0.0325}, {36, 1, 95.1, 0.0321},
	{48, 1, 102.7, 0.0319}, {60, 1, 109.4, 0.0318}, {72, 1, 115.1, 0.0418},
	{84, 1, 120.8, 0.0420}, {96, 1, 126.6, 0.0424}, {108, 1, 132.2, 0.0430},
	{120, 1, 137.8, 0.0438}, {132, 1, 143.8, 0.0446}, {144, 1, 149.8, 0.0448},
	{156, 1, 154.6, 0.0440}, {168, 1, 157.8, 0.0430}, {180, 1, 159.4, 0.0420},
	{192, 1, 160.0, 0.0410}, {204, 1, 160.1, 0.0400}, {216, 1, 160.1, 0.0390},
}

// Male weight-for-age
var maleWeightLMS = []lmsRow{
	{0, 0.35, 3.3, 0.121}, {1, 0.24, 4.5, 0.131}, {3, 0.13, 6.4, 0.131},
	{6, 0.01, 7.9, 0.124}, {9, -0.06, 9.2, 0.119}, {12, -0.1, 10.2, 0.116},
	{18, -0.13, 11.5, 0.113}, {24, -0.15, 12.7, 0.112}, {36, -0.15, 14.3, 0.112},
	{48, -0.13, 16.3, 0.113}, {60, -0.1, 18.3, 0.115}, {72, -0.07, 20.5, 0.120},
	{84, -0.04, 22.9, 0.126}, {96, 0, 25.6, 0.133}, {108, 0.04, 28.6, 0.139},
	{120, 0.08, 32.0, 0.145}, {132, 0.1, 35.6, 0.149}, {144, 0.1, 39.9, 0.150},
	{156, 0.1, 45.0, 0.148}, {168, 0.08, 50.5, 0.143}, {180, 0.06, 55.5, 0.138},
	{192, 0.04, 59.5, 0.133}, {204, 0.02, 62.5, 0.128}, {216, 0, 64.0, 0.125},
}

// Female weight-for-age
var femaleWeightLMS = []lmsRow{
	{0, 0.38, 3.2, 0.115}, {1, 0.28, 4.2, 0.127}, {3, 0.17, 5.8, 0.127},
	{6, 0.06, 7.3, 0.121}, {9, -0.01, 8.6, 0.117}, {12, -0.05, 9.5, 0.115},
	{18, -0.08, 11.0, 0.113}, {24, -0.1, 12.1, 0.113}, {36, -0.1, 14.0, 0.113},
	{48, -0.08, 16.1, 0.114}, {60, -0.05, 18.2, 0.117}, {72, -0.02, 20.2, 0.122},
	{84, 0.01, 22.4, 0.128}, {96, 0.04, 25.0, 0.134}, {108, 0.07, 28.0, 0.140},
	{120, 0.1, 31.5, 0.146}, {132, 0.12, 36.0, 0.150}, {144, 0.12, 40.5, 0.150},
	{156, 0.1, 45.0, 0.148}, {168, 0.08, 49.0, 0.143}, {180, 0.06, 52.0, 0.138},
	{192, 0.04, 53.5, 0.133}, {204, 0.02, 54.5, 0.128}, {216, 0, 55.0, 0.125},
}

// Male BMI-for-age
var maleBMI_LMS = []lmsRow{
	{0, 0.5, 13.4, 0.091}, {3, -0.2, 16.5, 0.083}, {6, -0.8, 17.3, 0.079},
	{12, -1.2, 17.2, 0.078}, {24, -1.5, 16.5, 0.079}, {36, -1.6, 15.9, 0.079},
	{48, -1.7, 15.5, 0.080}, {60, -1.8, 15.3, 0.081}, {72, -1.9, 15.3, 0.085},
	{84, -2.0, 15.5, 0.089}, {96, -2.0, 15.7, 0.094}, {108, -2.0, 16.0, 0.100},
	{120, -2.0, 16.4, 0.105}, {132, -1.9, 17.0, 0.110}, {144, -1.8, 17.6, 0.112},
	{156, -1.7, 18.4, 0.112}, {168, -1.5, 19.2, 0.110}, {180, -1.3, 20.0, 0.108},
	{192, -1.1, 20.7, 0.105}, {204, -0.9, 21.2, 0.102}, {216, -0.7, 21.5, 0.100},
}

// Female BMI-for-age
var femaleBMI_LMS = []lmsRow{
	{0, 0.5, 13.3, 0.092}, {3, -0.2, 16.2, 0.085}, {6, -0.7, 17.0, 0.080},
	{12, -1.0, 16.7, 0.079}, {24, -1.3, 16.0, 0.080}, {36, -1.4, 15.5, 0.080},
	{48, -1.5, 15.2, 0.081}, {60, -1.5, 15.0, 0.082}, {72, -1.6, 15.0, 0.087},
	{84, -1.7, 15.2, 0.092}, {96, -1.7, 15.5, 0.097}, {108, -1.7, 15.9, 0.103},
	{120, -1.6, 16.4, 0.108}, {132, -1.5, 17.1, 0.112}, {144, -1.4, 17.8, 0.113},
	{156, -1.2, 18.6, 0.112}, {168, -1.0, 19.3, 0.110}, {180, -0.8, 19.9, 0.107},
	{192, -0.6, 20.3, 0.104}, {204, -0.4, 20.6, 0.101}, {216, -0.3, 20.8, 0.100},
}

// ═══════════════════════════════════════════════════════════════
// Utility functions
// ═══════════════════════════════════════════════════════════════

func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	case string:
		var f float64
		fmt.Sscanf(n, "%f", &f)
		return f
	}
	return 0
}

func roundTo(v float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(v*pow) / pow
}

func jsonStr(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// suppress unused import warning for strings
var _ = strings.Contains
