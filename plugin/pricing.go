package plugin

import (
	"fmt"
	"strings"
)

// ModelPricing contains pricing information for a model
type ModelPricing struct {
	Name                 string
	InputPriceShort      float64 // per 1M tokens (context <= 200K)
	InputPriceLong       float64 // per 1M tokens (context > 200K)
	OutputPriceShort     float64 // per 1M tokens
	OutputPriceLong      float64 // per 1M tokens
	LongContextThreshold int     // tokens, 0 means no long context pricing
}

// PricingTable contains pricing for all supported models
var PricingTable = map[string]ModelPricing{
	// Gemini 3.0 Series (Preview)
	"gemini-3-pro-preview": {
		Name:             "Gemini 3 Pro",
		InputPriceShort:  4.00,
		InputPriceLong:   4.00,
		OutputPriceShort: 12.00,
		OutputPriceLong:  12.00,
	},
	"gemini-3-flash-preview": {
		Name:             "Gemini 3 Flash",
		InputPriceShort:  0.50,
		InputPriceLong:   0.50,
		OutputPriceShort: 3.00,
		OutputPriceLong:  3.00,
	},

	// Gemini 2.5 Series (Production)
	"gemini-2.5-pro": {
		Name:                 "Gemini 2.5 Pro",
		InputPriceShort:      1.25,
		InputPriceLong:       2.50,
		OutputPriceShort:     10.00,
		OutputPriceLong:      15.00,
		LongContextThreshold: 200000,
	},
	"gemini-2.5-flash": {
		Name:             "Gemini 2.5 Flash",
		InputPriceShort:  0.30,
		InputPriceLong:   0.30,
		OutputPriceShort: 2.50,
		OutputPriceLong:  2.50,
	},
	"gemini-2.5-flash-lite": {
		Name:             "Gemini 2.5 Flash-Lite",
		InputPriceShort:  0.10,
		InputPriceLong:   0.10,
		OutputPriceShort: 0.40,
		OutputPriceLong:  0.40,
	},

	// Gemini 2.0 Series
	"gemini-2.0-flash": {
		Name:             "Gemini 2.0 Flash",
		InputPriceShort:  0.15,
		InputPriceLong:   0.15,
		OutputPriceShort: 0.60,
		OutputPriceLong:  0.60,
	},
	"gemini-2.0-flash-lite": {
		Name:             "Gemini 2.0 Flash-Lite",
		InputPriceShort:  0.075,
		InputPriceLong:   0.075,
		OutputPriceShort: 0.30,
		OutputPriceLong:  0.30,
	},

	// Gemini 1.5 Series (legacy)
	"gemini-1.5-pro": {
		Name:             "Gemini 1.5 Pro",
		InputPriceShort:  1.25,
		InputPriceLong:   1.25,
		OutputPriceShort: 5.00,
		OutputPriceLong:  5.00,
	},
	"gemini-1.5-flash": {
		Name:             "Gemini 1.5 Flash",
		InputPriceShort:  0.075,
		InputPriceLong:   0.075,
		OutputPriceShort: 0.30,
		OutputPriceLong:  0.30,
	},
}

// UsageStats holds token usage statistics
type UsageStats struct {
	Model          string
	InputTokens    int
	OutputTokens   int
	ThoughtsTokens int // Thinking tokens for reasoning models (billed as output)
	TotalTokens    int
	EstimatedInput int // local estimate before API call
	InputCost      float64
	OutputCost     float64
	ThoughtsCost   float64 // Cost for thinking tokens
	TotalCost      float64
	IsLongContext  bool
}

// CostCalculator calculates API costs based on token usage
type CostCalculator struct {
	model   string
	pricing ModelPricing
}

// NewCostCalculator creates a new cost calculator for a model
func NewCostCalculator(model string) *CostCalculator {
	// Try to find exact match first
	pricing, ok := PricingTable[model]
	if !ok {
		// Try partial match
		for key, p := range PricingTable {
			if strings.Contains(strings.ToLower(model), strings.ToLower(key)) {
				pricing = p
				ok = true
				break
			}
		}
	}

	// Default pricing if model not found
	if !ok {
		pricing = ModelPricing{
			Name:             model,
			InputPriceShort:  1.00,
			OutputPriceShort: 5.00,
			InputPriceLong:   1.00,
			OutputPriceLong:  5.00,
		}
	}

	return &CostCalculator{
		model:   model,
		pricing: pricing,
	}
}

// EstimateTokens estimates token count from text
// Rule of thumb: ~4 characters per token for English, ~2 for Chinese
func (c *CostCalculator) EstimateTokens(text string) int {
	// Count characters
	charCount := len(text)

	// Rough estimate: average 3 chars per token (mix of code/text)
	// This is a conservative estimate
	return charCount / 3
}

// CalculateCost calculates the cost for given token usage
func (c *CostCalculator) CalculateCost(inputTokens, outputTokens, thoughtsTokens int) *UsageStats {
	stats := &UsageStats{
		Model:          c.pricing.Name,
		InputTokens:    inputTokens,
		OutputTokens:   outputTokens,
		ThoughtsTokens: thoughtsTokens,
		TotalTokens:    inputTokens + outputTokens + thoughtsTokens,
	}

	// Check if this is long context
	if c.pricing.LongContextThreshold > 0 && inputTokens > c.pricing.LongContextThreshold {
		stats.IsLongContext = true
		stats.InputCost = float64(inputTokens) / 1_000_000 * c.pricing.InputPriceLong
		stats.OutputCost = float64(outputTokens) / 1_000_000 * c.pricing.OutputPriceLong
		stats.ThoughtsCost = float64(thoughtsTokens) / 1_000_000 * c.pricing.OutputPriceLong
	} else {
		stats.InputCost = float64(inputTokens) / 1_000_000 * c.pricing.InputPriceShort
		stats.OutputCost = float64(outputTokens) / 1_000_000 * c.pricing.OutputPriceShort
		stats.ThoughtsCost = float64(thoughtsTokens) / 1_000_000 * c.pricing.OutputPriceShort
	}

	stats.TotalCost = stats.InputCost + stats.OutputCost + stats.ThoughtsCost
	return stats
}

// FormatCostSummary formats the usage stats as a readable string
func (stats *UsageStats) FormatCostSummary() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("+--------------------------------------------------------------+\n")
	sb.WriteString("|                    Token Usage Statistics                     |\n")
	sb.WriteString("+--------------------------------------------------------------+\n")
	sb.WriteString(fmt.Sprintf("|  Model: %-53s |\n", stats.Model))

	if stats.EstimatedInput > 0 {
		sb.WriteString(fmt.Sprintf("|  Estimated Input: %-43d |\n", stats.EstimatedInput))
	}

	sb.WriteString(fmt.Sprintf("|  Input Tokens: %-45d |\n", stats.InputTokens))
	sb.WriteString(fmt.Sprintf("|  Output Tokens: %-44d |\n", stats.OutputTokens))
	if stats.ThoughtsTokens > 0 {
		sb.WriteString(fmt.Sprintf("|  Thinking Tokens: %-42d |\n", stats.ThoughtsTokens))
	}
	sb.WriteString(fmt.Sprintf("|  Total Tokens: %-45d |\n", stats.TotalTokens))
	sb.WriteString("+--------------------------------------------------------------+\n")

	if stats.IsLongContext {
		sb.WriteString("|  [!] Long context pricing (>200K tokens)                     |\n")
	}

	sb.WriteString(fmt.Sprintf("|  Input Cost: $%-47.6f |\n", stats.InputCost))
	sb.WriteString(fmt.Sprintf("|  Output Cost: $%-46.6f |\n", stats.OutputCost))
	if stats.ThoughtsCost > 0 {
		sb.WriteString(fmt.Sprintf("|  Thinking Cost: $%-44.6f |\n", stats.ThoughtsCost))
	}
	sb.WriteString("+--------------------------------------------------------------+\n")
	sb.WriteString(fmt.Sprintf("|  Total Cost: $%-47.6f |\n", stats.TotalCost))
	sb.WriteString("+--------------------------------------------------------------+\n")

	return sb.String()
}

// FormatCostSummarySimple formats a simple one-line cost summary
func (stats *UsageStats) FormatCostSummarySimple() string {
	return fmt.Sprintf("Tokens: %d in / %d out = $%.4f",
		stats.InputTokens, stats.OutputTokens, stats.TotalCost)
}
