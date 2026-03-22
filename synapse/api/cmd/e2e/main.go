package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
)

var apiURL = "http://localhost:8096"
var apiKey = ""
var jwtToken = ""

func main() {
	// Optional: pass existing API key, or test will register a new user
	if len(os.Args) > 1 {
		apiKey = os.Args[1]
	}

	pass, fail := 0, 0
	test := func(name string, ok bool) {
		if ok {
			fmt.Printf("  ✓ %s\n", name)
			pass++
		} else {
			fmt.Printf("  ✗ %s\n", name)
			fail++
		}
	}

	fmt.Println("=== Router API E2E Tests ===")
	fmt.Printf("Target: %s\n\n", apiURL)

	// ── Health ──
	body, status := get("/health", "")
	test("GET /health → 200", status == 200 && contains(body, "ok"))

	// ── Auth: Register ──
	email := fmt.Sprintf("e2e-%d@test.star-ai.net", rand.Intn(999999))
	body, status = post("/auth/register", "", fmt.Sprintf(
		`{"email":"%s","password":"Test123!","name":"E2E User"}`, email))
	test("POST /auth/register → 201", status == 201)

	if status == 201 {
		jwtToken = jsonGet(body, "token")
		if apiKey == "" {
			apiKey = jsonGet(body, "api_key", "key")
		}
		test("Register returns JWT token", jwtToken != "")
		test("Register returns API key", contains(body, "sk-star-"))
		fmt.Printf("    API Key: %s\n", truncate(apiKey, 24)+"...")
	}

	// ── Auth: Duplicate register ──
	_, status = post("/auth/register", "", fmt.Sprintf(
		`{"email":"%s","password":"Test123!"}`, email))
	test("POST /auth/register (duplicate) → 409", status == 409)

	// ── Auth: Login ──
	body, status = post("/auth/login", "", fmt.Sprintf(
		`{"email":"%s","password":"Test123!"}`, email))
	test("POST /auth/login → 200", status == 200 && contains(body, "token"))

	// ── Auth: Bad password ──
	_, status = post("/auth/login", "", fmt.Sprintf(
		`{"email":"%s","password":"wrong"}`, email))
	test("POST /auth/login (bad password) → 401", status == 401)

	// ── Dashboard: Profile (JWT) ──
	body, status = getJWT("/dash/profile", jwtToken)
	test("GET /dash/profile (JWT) → 200", status == 200 && contains(body, email))

	// ── Dashboard: No JWT → 401 ──
	_, status = get("/dash/profile", "")
	test("GET /dash/profile (no JWT) → 401", status == 401)

	// ── Dashboard: Keys via JWT ──
	body, status = getJWT("/dash/keys", jwtToken)
	test("GET /dash/keys (JWT) → 200", status == 200 && contains(body, "keys"))

	// ── Dashboard: Balance via JWT ──
	body, status = getJWT("/dash/balance", jwtToken)
	test("GET /dash/balance (JWT) → 200", status == 200 && contains(body, "balance_cents"))

	// ── Payment: Packages ──
	body, status = getJWT("/dash/pay/packages", jwtToken)
	test("GET /dash/pay/packages → 200 + has packages", status == 200 && contains(body, "pkg_100"))

	// ── Payment: Create Alipay (no config → 503) ──
	body, status = postJWT("/dash/pay/alipay", jwtToken, `{"package_id":"pkg_10"}`)
	test("POST /dash/pay/alipay (unconfigured) → 503", status == 503)

	// ── Payment: Create WeChat (no config → 503) ──
	body, status = postJWT("/dash/pay/wechat", jwtToken, `{"package_id":"pkg_10"}`)
	test("POST /dash/pay/wechat (unconfigured) → 503", status == 503)

	// ── Payment: Invalid package ──
	body, status = postJWT("/dash/pay/alipay", jwtToken, `{"package_id":"pkg_invalid"}`)
	test("POST /dash/pay/alipay (bad pkg) → 400 or 503", status == 400 || status == 503)

	// ── Payment: Orders ──
	body, status = getJWT("/dash/pay/orders", jwtToken)
	test("GET /dash/pay/orders → 200", status == 200 && contains(body, "orders"))

	fmt.Println()

	// ── API Key auth rejection ──
	_, status = get("/v1/models", "sk-star-invalid")
	test("GET /v1/models (bad key) → 401", status == 401)

	// ── Models list ──
	body, status = get("/v1/models", apiKey)
	test("GET /v1/models → 200 + has models", status == 200 && contains(body, "openai/gpt-4o"))

	// ── Keys CRUD ──
	body, status = get("/v1/keys", apiKey)
	test("GET /v1/keys → 200", status == 200 && contains(body, "keys"))

	body, status = post("/v1/keys", apiKey, `{"name":"E2E Test Key"}`)
	test("POST /v1/keys → 201 + returns key", status == 201 && contains(body, "sk-star-"))

	// ── Balance / Usage ──
	body, status = get("/v1/balance", apiKey)
	test("GET /v1/balance → 200", status == 200 && contains(body, "balance_cents"))

	body, status = get("/v1/usage", apiKey)
	test("GET /v1/usage → 200", status == 200 && contains(body, "records"))

	// ── Chat routing ──
	body, status = post("/v1/chat/completions", apiKey, `{"model":"qwen/qwen-turbo","messages":[{"role":"user","content":"hi"}]}`)
	isOurAuthError := status == 401 && contains(body, "authentication_error")
	test("POST /v1/chat/completions (qwen) → routes domestic", !isOurAuthError)
	fmt.Printf("    Response: %d — %s\n", status, truncate(body, 120))

	body, status = post("/v1/chat/completions", apiKey, `{"model":"openai/gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	test("POST /v1/chat/completions (openai) → routes via proxy", status != 401)
	fmt.Printf("    Response: %d — %s\n", status, truncate(body, 120))

	_, status = post("/v1/chat/completions", apiKey, `{"model":"unknown/model","messages":[{"role":"user","content":"hi"}]}`)
	test("POST /v1/chat/completions (unknown) → 400", status == 400)

	// ── Rate limit headers ──
	resp := doReq("GET", "/v1/models", apiKey, "")
	test("Rate limit headers present", resp.Header.Get("X-RateLimit-Limit") != "")

	fmt.Printf("\n=== Results: %d passed, %d failed ===\n", pass, fail)
	if fail > 0 {
		os.Exit(1)
	}
}

func get(path, key string) (string, int) {
	resp := doReq("GET", path, key, "")
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b), resp.StatusCode
}

func getJWT(path, token string) (string, int) {
	return get(path, token)
}

func postJWT(path, token, body string) (string, int) {
	return post(path, token, body)
}

func post(path, key, body string) (string, int) {
	resp := doReq("POST", path, key, body)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b), resp.StatusCode
}

func doReq(method, path, key, body string) *http.Response {
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	req, _ := http.NewRequest(method, apiURL+path, reader)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("  ✗ HTTP error: %v\n", err)
		os.Exit(1)
	}
	return resp
}

func contains(s, substr string) bool {
	return len(s) > 0 && bytes.Contains([]byte(s), []byte(substr))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// jsonGet extracts a nested string value from JSON. Keys are path segments.
// e.g. jsonGet(body, "api_key", "key") → body["api_key"]["key"]
func jsonGet(body string, keys ...string) string {
	var data interface{}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return ""
	}
	current := data
	for _, k := range keys {
		m, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current = m[k]
	}
	s, _ := current.(string)
	return s
}
