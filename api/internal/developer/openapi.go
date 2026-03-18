package developer

import (
	"encoding/json"
	"time"
)

// ════════════════════════════════════════════════════════════════
//  OpenAPI 3.0 Spec Generator
// ════════════════════════════════════════════════════════════════

// OpenAPISpec represents an OpenAPI 3.0 specification.
type OpenAPISpec struct {
	OpenAPI    string                `json:"openapi"`
	Info       OpenAPIInfo           `json:"info"`
	Servers    []OpenAPIServer       `json:"servers"`
	Paths      map[string]PathItem   `json:"paths"`
	Components *Components           `json:"components,omitempty"`
	Tags       []Tag                 `json:"tags"`
}

type OpenAPIInfo struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Contact     *Contact `json:"contact,omitempty"`
	License     *License `json:"license,omitempty"`
}

type Contact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

type License struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

type OpenAPIServer struct {
	URL         string `json:"url"`
	Description string `json:"description"`
}

type Tag struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type PathItem map[string]Operation // method -> operation

type Operation struct {
	Tags        []string              `json:"tags"`
	Summary     string                `json:"summary"`
	Description string                `json:"description,omitempty"`
	OperationID string                `json:"operationId"`
	Parameters  []Parameter           `json:"parameters,omitempty"`
	RequestBody *RequestBody          `json:"requestBody,omitempty"`
	Responses   map[string]Response   `json:"responses"`
	Security    []map[string][]string `json:"security,omitempty"`
}

type Parameter struct {
	Name        string `json:"name"`
	In          string `json:"in"` // query, path, header
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Schema      Schema `json:"schema"`
}

type RequestBody struct {
	Description string               `json:"description,omitempty"`
	Required    bool                 `json:"required,omitempty"`
	Content     map[string]MediaType `json:"content"`
}

type MediaType struct {
	Schema Schema `json:"schema"`
}

type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

type Schema struct {
	Type       string            `json:"type,omitempty"`
	Format     string            `json:"format,omitempty"`
	Properties map[string]Schema `json:"properties,omitempty"`
	Items      *Schema           `json:"items,omitempty"`
	Ref        string            `json:"$ref,omitempty"`
	Example    interface{}       `json:"example,omitempty"`
	Enum       []string          `json:"enum,omitempty"`
}

type Components struct {
	Schemas         map[string]Schema `json:"schemas,omitempty"`
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty"`
}

type SecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
	Description  string `json:"description,omitempty"`
}

// GenerateSpec builds the complete OpenAPI 3.0 spec for StarClaw.
func GenerateSpec(serverURL string) *OpenAPISpec {
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	spec := &OpenAPISpec{
		OpenAPI: "3.0.3",
		Info: OpenAPIInfo{
			Title:       "StarClaw API",
			Description: "StarClaw — Open Source AI Agent Orchestration Platform. Complete API reference for building agents, managing workflows, and integrating AI capabilities.",
			Version:     time.Now().Format("2006.0102.1504"),
			Contact:     &Contact{Name: "StarClaw Team", URL: "https://star-ai.net", Email: "dev@star-ai.net"},
			License:     &License{Name: "Apache 2.0", URL: "https://www.apache.org/licenses/LICENSE-2.0"},
		},
		Servers: []OpenAPIServer{
			{URL: serverURL + "/api/v1", Description: "Current Server"},
		},
		Tags: []Tag{
			{Name: "Auth", Description: "Authentication and user management"},
			{Name: "Agents", Description: "AI Agent CRUD and execution"},
			{Name: "Chat", Description: "Chat completions with streaming"},
			{Name: "Workflows", Description: "Visual workflow engine"},
			{Name: "Knowledge", Description: "RAG knowledge base management"},
			{Name: "Tools", Description: "Tool and plugin management"},
			{Name: "Marketplace", Description: "Agent marketplace and economy"},
			{Name: "Observe", Description: "Observability: traces, alerts, logs"},
			{Name: "Webhooks", Description: "Event-driven webhook orchestration"},
			{Name: "P2P", Description: "Peer-to-peer networking and evolution"},
			{Name: "System", Description: "System configuration and admin"},
		},
		Paths: make(map[string]PathItem),
		Components: &Components{
			SecuritySchemes: map[string]SecurityScheme{
				"BearerAuth": {
					Type:         "http",
					Scheme:       "bearer",
					BearerFormat: "JWT",
					Description:  "JWT token from /auth/login or /auth/register",
				},
			},
			Schemas: buildSchemas(),
		},
	}

	// Register all endpoint groups
	registerAuthEndpoints(spec)
	registerAgentEndpoints(spec)
	registerChatEndpoints(spec)
	registerWorkflowEndpoints(spec)
	registerKnowledgeEndpoints(spec)
	registerMarketplaceEndpoints(spec)
	registerObserveEndpoints(spec)
	registerWebhookEndpoints(spec)

	return spec
}

// ToJSON serializes the spec to JSON.
func (s *OpenAPISpec) ToJSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

// ── Schema definitions ──

func buildSchemas() map[string]Schema {
	return map[string]Schema{
		"Agent": {
			Type: "object",
			Properties: map[string]Schema{
				"id":            {Type: "string", Format: "uuid"},
				"name":          {Type: "string", Example: "My Assistant"},
				"description":   {Type: "string"},
				"system_prompt": {Type: "string"},
				"model_name":    {Type: "string", Example: "gpt-4o"},
				"tools":         {Type: "array", Items: &Schema{Type: "string"}},
				"is_public":     {Type: "boolean"},
				"created_at":    {Type: "string", Format: "date-time"},
			},
		},
		"ChatRequest": {
			Type: "object",
			Properties: map[string]Schema{
				"agent_id":        {Type: "string", Format: "uuid"},
				"message":         {Type: "string", Example: "Hello, how can you help me?"},
				"conversation_id": {Type: "string", Format: "uuid"},
				"model":           {Type: "string", Example: "gpt-4o"},
				"stream":          {Type: "boolean", Example: true},
			},
		},
		"Workflow": {
			Type: "object",
			Properties: map[string]Schema{
				"id":          {Type: "string", Format: "uuid"},
				"name":        {Type: "string"},
				"description": {Type: "string"},
				"nodes":       {Type: "string"},
				"edges":       {Type: "string"},
				"created_at":  {Type: "string", Format: "date-time"},
			},
		},
		"KnowledgeBase": {
			Type: "object",
			Properties: map[string]Schema{
				"id":          {Type: "string", Format: "uuid"},
				"name":        {Type: "string"},
				"description": {Type: "string"},
				"doc_count":   {Type: "integer"},
				"created_at":  {Type: "string", Format: "date-time"},
			},
		},
		"Error": {
			Type: "object",
			Properties: map[string]Schema{
				"error": {Type: "string", Example: "unauthorized"},
			},
		},
	}
}

// Helper to create a protected operation.
func protectedOp(tags []string, summary, opID string) Operation {
	return Operation{
		Tags:        tags,
		Summary:     summary,
		OperationID: opID,
		Responses: map[string]Response{
			"200": {Description: "Success"},
			"401": {Description: "Unauthorized", Content: map[string]MediaType{"application/json": {Schema: Schema{Ref: "#/components/schemas/Error"}}}},
		},
		Security: []map[string][]string{{"BearerAuth": {}}},
	}
}

// ── Endpoint registrations ──

func registerAuthEndpoints(spec *OpenAPISpec) {
	spec.Paths["/auth/register"] = PathItem{
		"post": {
			Tags: []string{"Auth"}, Summary: "Register a new user", OperationID: "register",
			RequestBody: &RequestBody{Required: true, Content: map[string]MediaType{"application/json": {Schema: Schema{
				Type: "object", Properties: map[string]Schema{
					"username": {Type: "string"}, "email": {Type: "string", Format: "email"}, "password": {Type: "string", Format: "password"},
				},
			}}}},
			Responses: map[string]Response{"200": {Description: "JWT token returned"}},
		},
	}
	spec.Paths["/auth/login"] = PathItem{
		"post": {
			Tags: []string{"Auth"}, Summary: "Login with credentials", OperationID: "login",
			RequestBody: &RequestBody{Required: true, Content: map[string]MediaType{"application/json": {Schema: Schema{
				Type: "object", Properties: map[string]Schema{
					"email": {Type: "string", Format: "email"}, "password": {Type: "string", Format: "password"},
				},
			}}}},
			Responses: map[string]Response{"200": {Description: "JWT token + user info"}},
		},
	}
}

func registerAgentEndpoints(spec *OpenAPISpec) {
	listOp := protectedOp([]string{"Agents"}, "List all agents", "listAgents")
	listOp.Parameters = []Parameter{{Name: "page", In: "query", Schema: Schema{Type: "integer"}}}
	spec.Paths["/agents"] = PathItem{
		"get":  listOp,
		"post": protectedOp([]string{"Agents"}, "Create a new agent", "createAgent"),
	}
	spec.Paths["/agents/{id}"] = PathItem{
		"get":    protectedOp([]string{"Agents"}, "Get agent by ID", "getAgent"),
		"put":    protectedOp([]string{"Agents"}, "Update agent", "updateAgent"),
		"delete": protectedOp([]string{"Agents"}, "Delete agent", "deleteAgent"),
	}
	spec.Paths["/agents/{id}/clone"] = PathItem{"post": protectedOp([]string{"Agents"}, "Clone an agent", "cloneAgent")}
	spec.Paths["/agents/{id}/export"] = PathItem{"get": protectedOp([]string{"Agents"}, "Export agent as JSON", "exportAgent")}
	spec.Paths["/agents/import"] = PathItem{"post": protectedOp([]string{"Agents"}, "Import agent from JSON", "importAgent")}
}

func registerChatEndpoints(spec *OpenAPISpec) {
	chatOp := protectedOp([]string{"Chat"}, "Send chat message (SSE streaming)", "chatCompletions")
	chatOp.RequestBody = &RequestBody{Required: true, Content: map[string]MediaType{"application/json": {Schema: Schema{Ref: "#/components/schemas/ChatRequest"}}}}
	spec.Paths["/chat/completions"] = PathItem{"post": chatOp}
	spec.Paths["/conversations"] = PathItem{"get": protectedOp([]string{"Chat"}, "List conversations", "listConversations")}
	spec.Paths["/conversations/{id}/messages"] = PathItem{"get": protectedOp([]string{"Chat"}, "Get conversation messages", "getMessages")}
}

func registerWorkflowEndpoints(spec *OpenAPISpec) {
	spec.Paths["/workflows"] = PathItem{
		"get":  protectedOp([]string{"Workflows"}, "List workflows", "listWorkflows"),
		"post": protectedOp([]string{"Workflows"}, "Create workflow", "createWorkflow"),
	}
	spec.Paths["/workflows/{id}"] = PathItem{
		"get":    protectedOp([]string{"Workflows"}, "Get workflow", "getWorkflow"),
		"put":    protectedOp([]string{"Workflows"}, "Update workflow", "updateWorkflow"),
		"delete": protectedOp([]string{"Workflows"}, "Delete workflow", "deleteWorkflow"),
	}
	spec.Paths["/workflows/{id}/run"] = PathItem{"post": protectedOp([]string{"Workflows"}, "Execute workflow", "runWorkflow")}
}

func registerKnowledgeEndpoints(spec *OpenAPISpec) {
	spec.Paths["/knowledge-bases"] = PathItem{
		"get":  protectedOp([]string{"Knowledge"}, "List knowledge bases", "listKBs"),
		"post": protectedOp([]string{"Knowledge"}, "Create knowledge base", "createKB"),
	}
	spec.Paths["/knowledge-bases/{id}"] = PathItem{
		"get":    protectedOp([]string{"Knowledge"}, "Get knowledge base", "getKB"),
		"delete": protectedOp([]string{"Knowledge"}, "Delete knowledge base", "deleteKB"),
	}
	spec.Paths["/knowledge-bases/{id}/documents"] = PathItem{"post": protectedOp([]string{"Knowledge"}, "Upload document", "uploadDocument")}
	spec.Paths["/knowledge-bases/{id}/search"] = PathItem{"post": protectedOp([]string{"Knowledge"}, "Search knowledge base (RAG)", "searchKB")}
}

func registerMarketplaceEndpoints(spec *OpenAPISpec) {
	spec.Paths["/marketplace/listings"] = PathItem{"get": protectedOp([]string{"Marketplace"}, "Browse published agents", "listMarketplace")}
	spec.Paths["/marketplace/listings/{id}"] = PathItem{"get": protectedOp([]string{"Marketplace"}, "Get listing detail", "getMarketplaceListing")}
	spec.Paths["/marketplace/listings/{id}/purchase"] = PathItem{"post": protectedOp([]string{"Marketplace"}, "Purchase an agent", "purchaseAgent")}
	spec.Paths["/marketplace/listings/{id}/rate"] = PathItem{"post": protectedOp([]string{"Marketplace"}, "Rate a purchased agent", "rateAgent")}
	spec.Paths["/marketplace/trending"] = PathItem{"get": protectedOp([]string{"Marketplace"}, "Trending agents", "trendingAgents")}
	spec.Paths["/marketplace/creator/register"] = PathItem{"post": protectedOp([]string{"Marketplace"}, "Register as creator", "registerCreator")}
	spec.Paths["/marketplace/creator/dashboard"] = PathItem{"get": protectedOp([]string{"Marketplace"}, "Creator revenue dashboard", "creatorDashboard")}
	spec.Paths["/marketplace/creator/listings"] = PathItem{
		"get":  protectedOp([]string{"Marketplace"}, "My listings", "myListings"),
		"post": protectedOp([]string{"Marketplace"}, "Create listing", "createListing"),
	}
}

func registerObserveEndpoints(spec *OpenAPISpec) {
	spec.Paths["/observe/stats"] = PathItem{"get": protectedOp([]string{"Observe"}, "Observability overview", "observeStats")}
	spec.Paths["/observe/traces/{trace_id}"] = PathItem{"get": protectedOp([]string{"Observe"}, "Get trace spans", "getTrace")}
	spec.Paths["/observe/spans"] = PathItem{"get": protectedOp([]string{"Observe"}, "Query spans", "querySpans")}
	spec.Paths["/observe/logs"] = PathItem{"get": protectedOp([]string{"Observe"}, "Query structured logs", "queryLogs")}
	spec.Paths["/observe/alerts/rules"] = PathItem{
		"get":  protectedOp([]string{"Observe"}, "List alert rules", "listAlertRules"),
		"post": protectedOp([]string{"Observe"}, "Create alert rule", "createAlertRule"),
	}
}

func registerWebhookEndpoints(spec *OpenAPISpec) {
	spec.Paths["/webhooks/rules"] = PathItem{
		"get":  protectedOp([]string{"Webhooks"}, "List event rules", "listWebhookRules"),
		"post": protectedOp([]string{"Webhooks"}, "Create event rule", "createWebhookRule"),
	}
	spec.Paths["/webhooks/logs"] = PathItem{"get": protectedOp([]string{"Webhooks"}, "List event logs", "listWebhookLogs")}
	spec.Paths["/webhooks/stats"] = PathItem{"get": protectedOp([]string{"Webhooks"}, "Webhook stats", "webhookStats")}
	spec.Paths["/webhooks/event-types"] = PathItem{"get": protectedOp([]string{"Webhooks"}, "List supported event types", "eventTypes")}
	spec.Paths["/webhooks/test"] = PathItem{"post": protectedOp([]string{"Webhooks"}, "Fire test event", "testWebhook")}
}
