package claude

import (
	"context"
	"fmt"
	"strings"
)

const knowledgeSynthesisSystemPrompt = `You merge scattered Minecraft world observations from an autonomous agent
named golem into a coherent world-knowledge entry. Your job is to produce
consolidated notes that preserve:

- Named places, biomes, and their coordinates
- Hostile zones, spawner locations, and travel hazards
- Resource clusters (ore veins, villages, structures) and how to reach them
- Routes and landmarks useful for navigation

Combine duplicates. Resolve conflicts toward the more recent or more specific
observation. Drop filler like weather chatter and routine survival details.
Write in third-person declarative notes -- short sentences or bullet lines,
grouped by location. Do not invent details not present in the source.`

// SynthesizeKnowledge asks ModelWriter (Sonnet) to consolidate scattered
// observations into a coherent world-knowledge entry. Callers persist the
// returned string. Empty input is rejected so we do not spend tokens on a
// no-op.
func (c *Client) SynthesizeKnowledge(ctx context.Context, observations string) (string, error) {
	if strings.TrimSpace(observations) == "" {
		return "", fmt.Errorf("observations payload is empty")
	}
	parts, msgs := synthesizeKnowledgeRequest(observations)
	resp, err := c.SendMessageParts(ctx, ModelWriter, parts, msgs, nil, Silent())
	if err != nil {
		return "", fmt.Errorf("synthesize knowledge: %w", err)
	}
	return strings.TrimSpace(resp.Text), nil
}

// synthesizeKnowledgeRequest builds the (system prompt, user messages) payload
// for a SynthesizeKnowledge call. Split out for unit testing without a network
// round trip.
func synthesizeKnowledgeRequest(observations string) (CacheableSystemPrompt, []Message) {
	parts := CacheableSystemPrompt{Stable: knowledgeSynthesisSystemPrompt}
	msgs := []Message{{
		Role: RoleUser,
		Content: []Block{{
			Type: BlockText,
			Text: "Synthesize a world-knowledge entry from these observations:\n\n" + observations,
		}},
	}}
	return parts, msgs
}
