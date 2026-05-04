package claude

import (
	"context"
	"fmt"
	"strings"
)

const escalationSystemPrompt = `You are the strategic advisor for a Minecraft survival agent. The agent
has escalated a decision to you because it needs deeper reasoning than
its moment-to-moment gameplay model can provide.

You will see the agent's current situation description and a context
summary of its game state (vitals, position, goals, recent history,
world knowledge). Provide concise, actionable strategic guidance.

Focus on:
- What to prioritize and why
- Risks the agent might not have considered
- Non-obvious approaches or alternatives
- How this decision connects to longer-term goals

Keep your response focused and practical -- the agent will incorporate
your guidance into its next actions. Two to four paragraphs is usually
the right length. Don't narrate or roleplay; advise.`

// ThinkDeeply calls ModelDeep (Opus) as a strategic advisor, passing the
// agent's situation description and a pre-formatted context summary. Returns
// the advisor's text guidance as the Response. No tools are provided -- Opus
// reasons in pure text.
func (c *Client) ThinkDeeply(ctx context.Context, situation, contextSummary string) (*Response, error) {
	var body strings.Builder
	fmt.Fprintf(&body, "## Situation\n\n%s\n", situation)
	if contextSummary != "" {
		fmt.Fprintf(&body, "\n## Context\n\n%s\n", contextSummary)
	}

	messages := []Message{
		{Role: RoleUser, Content: []Block{{Type: BlockText, Text: body.String()}}},
	}

	return c.SendMessage(ctx, ModelDeep, escalationSystemPrompt, messages, nil)
}
