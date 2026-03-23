package handler

import (
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"starclaw.net/synapse/api/internal/provider"
)

type ModelsHandler struct {
	registry *provider.Registry
}

func NewModelsHandler(reg *provider.Registry) *ModelsHandler {
	return &ModelsHandler{registry: reg}
}

type modelEntry struct {
	ID            string  `json:"id"`
	Object        string  `json:"object"`
	Created       int64   `json:"created"`
	OwnedBy       string  `json:"owned_by"`
	Type          string  `json:"type,omitempty"`
	ContextLength int     `json:"context_length,omitempty"`
	InputPrice    float64 `json:"input_price,omitempty"`
	OutputPrice   float64 `json:"output_price,omitempty"`
}

// ListModels returns all available models from the registry (OpenAI-compatible format)
func (h *ModelsHandler) ListModels(c *gin.Context) {
	now := time.Now().Unix()

	entries := h.registry.ListModels()
	models := make([]modelEntry, 0, len(entries))

	for _, e := range entries {
		models = append(models, modelEntry{
			ID:            e.Model.Name,
			Object:        "model",
			Created:       now,
			OwnedBy:       e.Slug,
			Type:          e.Model.Type,
			ContextLength: e.Model.ContextLength,
			InputPrice:    e.Model.InputPrice,
			OutputPrice:   e.Model.OutputPrice,
		})
	}

	// Sort by provider then model name for stable output
	sort.Slice(models, func(i, j int) bool {
		if models[i].OwnedBy != models[j].OwnedBy {
			return models[i].OwnedBy < models[j].OwnedBy
		}
		return models[i].ID < models[j].ID
	})

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   models,
	})
}
