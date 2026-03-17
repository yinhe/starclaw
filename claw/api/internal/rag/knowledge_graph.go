package rag

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
//  Knowledge Graph Models
// ════════════════════════════════════════════════════════════════

// KGNode represents an entity in the knowledge graph.
type KGNode struct {
	ID              string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	KnowledgeBaseID string    `json:"knowledge_base_id" gorm:"type:varchar(36);index;not null"`
	DocumentID      string    `json:"document_id" gorm:"type:varchar(36);index"`
	Label           string    `json:"label" gorm:"type:varchar(200);not null;index"` // entity name
	EntityType      string    `json:"entity_type" gorm:"type:varchar(50);index"`     // person, org, concept, product, tech, location, event
	Description     string    `json:"description" gorm:"type:text"`
	Properties      string    `json:"properties" gorm:"type:json"` // arbitrary key-value pairs
	Embedding       []byte    `json:"-" gorm:"type:longblob"`      // vector embedding of label+description
	MentionCount    int       `json:"mention_count" gorm:"default:1"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (n *KGNode) BeforeCreate(tx *gorm.DB) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	if n.Properties == "" {
		n.Properties = "{}"
	}
	return nil
}

// KGEdge represents a relationship between two entities.
type KGEdge struct {
	ID              string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	KnowledgeBaseID string    `json:"knowledge_base_id" gorm:"type:varchar(36);index;not null"`
	SourceNodeID    string    `json:"source_node_id" gorm:"type:varchar(36);index;not null"`
	TargetNodeID    string    `json:"target_node_id" gorm:"type:varchar(36);index;not null"`
	Relation        string    `json:"relation" gorm:"type:varchar(100);index;not null"` // is_a, part_of, uses, related_to, created_by, etc.
	Weight          float64   `json:"weight" gorm:"default:1.0"`
	Properties      string    `json:"properties" gorm:"type:json"`
	DocumentID      string    `json:"document_id" gorm:"type:varchar(36);index"`
	CreatedAt       time.Time `json:"created_at"`
}

func (e *KGEdge) BeforeCreate(tx *gorm.DB) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	if e.Properties == "" {
		e.Properties = "{}"
	}
	return nil
}

// ════════════════════════════════════════════════════════════════
//  Entity Extraction (LLM-powered)
// ════════════════════════════════════════════════════════════════

// ExtractedEntity is the output of entity extraction.
type ExtractedEntity struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ExtractedRelation is the output of relation extraction.
type ExtractedRelation struct {
	Source   string  `json:"source"`
	Target   string  `json:"target"`
	Relation string  `json:"relation"`
	Weight   float64 `json:"weight"`
}

// ExtractionResult is the combined output of entity + relation extraction.
type ExtractionResult struct {
	Entities  []ExtractedEntity   `json:"entities"`
	Relations []ExtractedRelation `json:"relations"`
}

// EntityExtractor extracts entities and relations from text using LLM.
type EntityExtractor struct {
	db       *gorm.DB
	embedder EmbeddingProvider
}

// NewEntityExtractor creates a new extractor.
func NewEntityExtractor(db *gorm.DB, embedder EmbeddingProvider) *EntityExtractor {
	return &EntityExtractor{db: db, embedder: embedder}
}

// ExtractAndStore parses an ExtractionResult (from LLM) and stores KG nodes/edges.
func (ex *EntityExtractor) ExtractAndStore(ctx context.Context, kbID, docID string, result ExtractionResult) error {
	// Create or update nodes
	nodeMap := make(map[string]*KGNode)
	for _, e := range result.Entities {
		normalizedLabel := strings.TrimSpace(strings.ToLower(e.Name))
		if normalizedLabel == "" {
			continue
		}

		// Check if node already exists
		var existing KGNode
		err := ex.db.Where("knowledge_base_id = ? AND LOWER(label) = ?", kbID, normalizedLabel).First(&existing).Error
		if err == nil {
			// Update existing
			existing.MentionCount++
			if e.Description != "" && len(e.Description) > len(existing.Description) {
				existing.Description = e.Description
			}
			ex.db.Save(&existing)
			nodeMap[normalizedLabel] = &existing
			continue
		}

		node := KGNode{
			KnowledgeBaseID: kbID,
			DocumentID:      docID,
			Label:           e.Name,
			EntityType:      e.Type,
			Description:     e.Description,
		}
		ex.db.Create(&node)
		nodeMap[normalizedLabel] = &node
	}

	// Embed node labels+descriptions for graph search
	var toEmbed []string
	var toEmbedNodes []*KGNode
	for _, n := range nodeMap {
		text := n.Label
		if n.Description != "" {
			text += ": " + n.Description
		}
		toEmbed = append(toEmbed, text)
		toEmbedNodes = append(toEmbedNodes, n)
	}

	if len(toEmbed) > 0 {
		embeddings, err := ex.embedder.Embed(ctx, toEmbed)
		if err != nil {
			log.Printf("[KG] Failed to embed entities: %v", err)
		} else {
			for i, emb := range embeddings {
				if i < len(toEmbedNodes) {
					toEmbedNodes[i].Embedding = SerializeVector(emb)
					ex.db.Model(toEmbedNodes[i]).Update("embedding", toEmbedNodes[i].Embedding)
				}
			}
		}
	}

	// Create edges
	for _, r := range result.Relations {
		srcLabel := strings.TrimSpace(strings.ToLower(r.Source))
		tgtLabel := strings.TrimSpace(strings.ToLower(r.Target))

		srcNode := nodeMap[srcLabel]
		tgtNode := nodeMap[tgtLabel]
		if srcNode == nil || tgtNode == nil {
			continue
		}

		weight := r.Weight
		if weight <= 0 {
			weight = 1.0
		}

		// Check if edge already exists
		var existing KGEdge
		err := ex.db.Where("knowledge_base_id = ? AND source_node_id = ? AND target_node_id = ? AND relation = ?",
			kbID, srcNode.ID, tgtNode.ID, r.Relation).First(&existing).Error
		if err == nil {
			// Strengthen existing edge
			existing.Weight += 0.1
			ex.db.Save(&existing)
			continue
		}

		edge := KGEdge{
			KnowledgeBaseID: kbID,
			SourceNodeID:    srcNode.ID,
			TargetNodeID:    tgtNode.ID,
			Relation:        r.Relation,
			Weight:          weight,
			DocumentID:      docID,
		}
		ex.db.Create(&edge)
	}

	return nil
}

// ════════════════════════════════════════════════════════════════
//  Graph Traversal (multi-hop reasoning)
// ════════════════════════════════════════════════════════════════

// GraphResult represents a node + its context from graph traversal.
type GraphResult struct {
	Node  KGNode  `json:"node"`
	Score float64 `json:"score"` // combined relevance score
	Depth int     `json:"depth"` // hops from query
	Path  string  `json:"path"`  // traversal path description
}

// GraphTraversal performs multi-hop graph search.
type GraphTraversal struct {
	db       *gorm.DB
	embedder EmbeddingProvider
}

// NewGraphTraversal creates a graph traversal engine.
func NewGraphTraversal(db *gorm.DB, embedder EmbeddingProvider) *GraphTraversal {
	return &GraphTraversal{db: db, embedder: embedder}
}

// Search finds relevant nodes by semantic similarity + graph traversal.
func (g *GraphTraversal) Search(ctx context.Context, kbID, query string, maxHops, topK int) ([]GraphResult, error) {
	if maxHops <= 0 {
		maxHops = 2
	}
	if topK <= 0 {
		topK = 10
	}

	// Embed query
	embeddings, err := g.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		return nil, nil
	}
	queryVec := embeddings[0]

	// Find seed nodes by vector similarity
	var allNodes []KGNode
	g.db.Where("knowledge_base_id = ?", kbID).Find(&allNodes)

	type scoredNode struct {
		node  KGNode
		score float64
	}
	var seedCandidates []scoredNode

	for _, n := range allNodes {
		if len(n.Embedding) == 0 {
			continue
		}
		vec := DeserializeVector(n.Embedding)
		sim := float64(CosineSimilarity(queryVec, vec))
		if sim > 0.3 { // threshold
			seedCandidates = append(seedCandidates, scoredNode{node: n, score: sim})
		}
	}

	sort.Slice(seedCandidates, func(i, j int) bool {
		return seedCandidates[i].score > seedCandidates[j].score
	})

	// Take top seeds
	maxSeeds := 5
	if len(seedCandidates) > maxSeeds {
		seedCandidates = seedCandidates[:maxSeeds]
	}

	// BFS traversal from seed nodes
	visited := make(map[string]bool)
	var results []GraphResult

	for _, seed := range seedCandidates {
		g.traverse(seed.node, seed.score, 0, seed.node.Label, maxHops, visited, &results)
	}

	// Sort by score and take top-k
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// traverse performs BFS from a node, accumulating results.
func (g *GraphTraversal) traverse(node KGNode, score float64, depth int, path string, maxHops int, visited map[string]bool, results *[]GraphResult) {
	if depth > maxHops || visited[node.ID] {
		return
	}
	visited[node.ID] = true

	*results = append(*results, GraphResult{
		Node:  node,
		Score: score,
		Depth: depth,
		Path:  node.Label,
	})

	// Decay score with depth
	nextScore := score * 0.6

	// Find connected nodes
	var edges []KGEdge
	g.db.Where("(source_node_id = ? OR target_node_id = ?) AND knowledge_base_id = ?",
		node.ID, node.ID, node.KnowledgeBaseID).Find(&edges)

	for _, edge := range edges {
		neighborID := edge.TargetNodeID
		if neighborID == node.ID {
			neighborID = edge.SourceNodeID
		}
		if visited[neighborID] {
			continue
		}

		var neighbor KGNode
		if err := g.db.Where("id = ?", neighborID).First(&neighbor).Error; err != nil {
			continue
		}

		edgeScore := nextScore * edge.Weight
		neighborPath := path + " → " + edge.Relation + " → " + neighbor.Label
		g.traverse(neighbor, edgeScore, depth+1, neighborPath, maxHops, visited, results)
	}
}

// ════════════════════════════════════════════════════════════════
//  Hybrid Retriever (vector + BM25 + graph → RRF fusion)
// ════════════════════════════════════════════════════════════════

// HybridResult combines results from multiple retrieval strategies.
type HybridResult struct {
	Content    string  `json:"content"`
	Source     string  `json:"source"` // vector, bm25, graph
	Score      float64 `json:"score"`
	DocumentID string  `json:"document_id"`
	ChunkID    string  `json:"chunk_id,omitempty"`
	NodeID     string  `json:"node_id,omitempty"`
	EntityType string  `json:"entity_type,omitempty"`
}

// HybridRetriever combines vector search, BM25 text search, and graph traversal.
type HybridRetriever struct {
	db              *gorm.DB
	embedder        EmbeddingProvider
	vectorRetriever *Retriever
	graphTraversal  *GraphTraversal
}

// NewHybridRetriever creates a hybrid retriever.
func NewHybridRetriever(db *gorm.DB, embedder EmbeddingProvider) *HybridRetriever {
	return &HybridRetriever{
		db:              db,
		embedder:        embedder,
		vectorRetriever: NewRetriever(db, embedder),
		graphTraversal:  NewGraphTraversal(db, embedder),
	}
}

// Search performs hybrid retrieval: vector + BM25 + graph, fused with RRF.
func (h *HybridRetriever) Search(ctx context.Context, kbID, query string, topK int) ([]HybridResult, error) {
	if topK <= 0 {
		topK = 10
	}

	// 1. Vector search
	vectorResults, err := h.vectorRetriever.Search(ctx, kbID, query, topK*2)
	if err != nil {
		log.Printf("[Hybrid] Vector search error: %v", err)
	}

	// 2. BM25 text search
	bm25Results := h.bm25Search(kbID, query, topK*2)

	// 3. Knowledge graph traversal
	graphResults, err := h.graphTraversal.Search(ctx, kbID, query, 2, topK)
	if err != nil {
		log.Printf("[Hybrid] Graph search error: %v", err)
	}

	// 4. RRF fusion
	return h.rrfFuse(vectorResults, bm25Results, graphResults, topK), nil
}

// bm25Search performs keyword-based text search using SQL LIKE/MATCH.
func (h *HybridRetriever) bm25Search(kbID, query string, limit int) []SearchResult {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return nil
	}

	// Build LIKE conditions for each term
	var chunks []struct {
		ID         string
		DocumentID string
		Content    string
		ChunkIndex int
		MatchCount int
	}

	// Simple BM25-like scoring: count matching terms in each chunk
	q := h.db.Table("document_chunks").
		Select("id, document_id, content, chunk_index").
		Where("knowledge_base_id = ?", kbID)

	// Filter chunks that contain at least one term
	var orConditions []string
	var args []interface{}
	for _, term := range terms {
		orConditions = append(orConditions, "LOWER(content) LIKE ?")
		args = append(args, "%"+term+"%")
	}
	q = q.Where(strings.Join(orConditions, " OR "), args...)
	q.Limit(limit * 2).Find(&chunks)

	// Score by term frequency
	var results []SearchResult
	for _, c := range chunks {
		contentLower := strings.ToLower(c.Content)
		score := float32(0)
		for _, term := range terms {
			count := float32(strings.Count(contentLower, term))
			if count > 0 {
				// Simple TF-IDF-like score: tf * idf approximation
				tf := count / float32(len(strings.Fields(c.Content))+1)
				idf := float32(math.Log(10.0)) // simplified
				score += tf * idf
			}
		}
		if score > 0 {
			results = append(results, SearchResult{
				ChunkID:    c.ID,
				DocumentID: c.DocumentID,
				Content:    c.Content,
				Score:      score,
				ChunkIndex: c.ChunkIndex,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

// rrfFuse combines results using Reciprocal Rank Fusion.
// RRF(d) = Σ 1 / (k + rank_i(d)), where k = 60
func (h *HybridRetriever) rrfFuse(vectorResults []SearchResult, bm25Results []SearchResult, graphResults []GraphResult, topK int) []HybridResult {
	const k = 60.0
	scores := make(map[string]*HybridResult)

	// Helper to get or create a result entry
	getOrCreate := func(key, content, source, docID, chunkID, nodeID, entityType string) *HybridResult {
		if r, ok := scores[key]; ok {
			return r
		}
		r := &HybridResult{
			Content:    content,
			Source:     source,
			DocumentID: docID,
			ChunkID:    chunkID,
			NodeID:     nodeID,
			EntityType: entityType,
		}
		scores[key] = r
		return r
	}

	// Vector results
	for rank, r := range vectorResults {
		key := "chunk:" + r.ChunkID
		hr := getOrCreate(key, r.Content, "vector", r.DocumentID, r.ChunkID, "", "")
		hr.Score += 1.0 / (k + float64(rank+1))
		hr.Source = "vector"
	}

	// BM25 results
	for rank, r := range bm25Results {
		key := "chunk:" + r.ChunkID
		hr := getOrCreate(key, r.Content, "bm25", r.DocumentID, r.ChunkID, "", "")
		hr.Score += 1.0 / (k + float64(rank+1))
		if hr.Source == "vector" {
			hr.Source = "vector+bm25" // boosted by both
		}
	}

	// Graph results
	for rank, r := range graphResults {
		key := "node:" + r.Node.ID
		content := r.Node.Label
		if r.Node.Description != "" {
			content += ": " + r.Node.Description
		}
		hr := getOrCreate(key, content, "graph", r.Node.DocumentID, "", r.Node.ID, r.Node.EntityType)
		hr.Score += 1.0 / (k + float64(rank+1))
	}

	// Collect and sort
	var results []HybridResult
	for _, r := range scores {
		results = append(results, *r)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > topK {
		results = results[:topK]
	}
	return results
}

// ════════════════════════════════════════════════════════════════
//  KG Stats
// ════════════════════════════════════════════════════════════════

// KGStats returns statistics for a knowledge base's graph.
func KGStats(db *gorm.DB, kbID string) map[string]interface{} {
	var nodeCount, edgeCount int64
	db.Model(&KGNode{}).Where("knowledge_base_id = ?", kbID).Count(&nodeCount)
	db.Model(&KGEdge{}).Where("knowledge_base_id = ?", kbID).Count(&edgeCount)

	// Entity type distribution
	type typeCount struct {
		EntityType string
		Count      int64
	}
	var types []typeCount
	db.Model(&KGNode{}).Where("knowledge_base_id = ?", kbID).
		Select("entity_type, COUNT(*) as count").
		Group("entity_type").Find(&types)

	typeMap := make(map[string]int64)
	for _, t := range types {
		typeMap[t.EntityType] = t.Count
	}

	// Relation distribution
	type relCount struct {
		Relation string
		Count    int64
	}
	var rels []relCount
	db.Model(&KGEdge{}).Where("knowledge_base_id = ?", kbID).
		Select("relation, COUNT(*) as count").
		Group("relation").Find(&rels)

	relMap := make(map[string]int64)
	for _, r := range rels {
		relMap[r.Relation] = r.Count
	}

	return map[string]interface{}{
		"nodes":          nodeCount,
		"edges":          edgeCount,
		"entity_types":   typeMap,
		"relation_types": relMap,
	}
}

// BuildKGContext constructs context from graph results for LLM injection.
func BuildKGContext(results []GraphResult, maxChars int) string {
	if len(results) == 0 {
		return ""
	}
	if maxChars <= 0 {
		maxChars = 2000
	}

	var sb strings.Builder
	sb.WriteString("Knowledge Graph Context:\n")

	for _, r := range results {
		entry := "- " + r.Node.Label
		if r.Node.EntityType != "" {
			entry += " (" + r.Node.EntityType + ")"
		}
		if r.Node.Description != "" {
			entry += ": " + r.Node.Description
		}
		entry += "\n"

		if sb.Len()+len(entry) > maxChars {
			break
		}
		sb.WriteString(entry)
	}

	return sb.String()
}

// GetEntityExtractionPrompt returns the system prompt for LLM-based entity extraction.
func GetEntityExtractionPrompt() string {
	return `Extract entities and relationships from the following text. Return a JSON object with:
{
  "entities": [{"name": "...", "type": "person|org|concept|product|tech|location|event", "description": "brief description"}],
  "relations": [{"source": "entity_name", "target": "entity_name", "relation": "is_a|part_of|uses|related_to|created_by|depends_on|competes_with", "weight": 1.0}]
}
Only include clearly stated facts. Do not infer relationships that aren't explicit in the text. Keep entity names concise.`
}

// ParseExtractionResult parses LLM output into ExtractionResult.
func ParseExtractionResult(llmOutput string) (ExtractionResult, error) {
	// Find JSON in the output (LLM may wrap it in markdown code blocks)
	output := llmOutput
	if idx := strings.Index(output, "{"); idx >= 0 {
		output = output[idx:]
	}
	if idx := strings.LastIndex(output, "}"); idx >= 0 {
		output = output[:idx+1]
	}

	var result ExtractionResult
	err := json.Unmarshal([]byte(output), &result)
	return result, err
}
