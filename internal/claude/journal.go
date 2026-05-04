package claude

import (
	"context"
	"fmt"
	"strings"
)

const journalCompactionSystemPrompt = `You compact first-person Minecraft journal entries written by an autonomous
agent named golem. Your job is to produce a shorter summary that preserves:

- Locations, coordinates, and named places
- Names of players met and their disposition
- Milestones, deaths, and notable losses
- Standing goals and plans the agent is tracking

Drop narrative padding, weather chatter, and routine survival actions.
Write in the first person, past tense, plain prose. Aim for roughly one
paragraph per day of game time, or one paragraph per distinct episode.
Do not invent details not present in the source entries.`

// CompactJournal asks ModelWriter (Sonnet) to summarize a block of journal
// entries. The returned string is the summary text; callers are responsible
// for persisting it. An empty entries payload is rejected so we do not spend
// tokens on a no-op.
func (c *Client) CompactJournal(ctx context.Context, entries string) (string, error) {
	if strings.TrimSpace(entries) == "" {
		return "", fmt.Errorf("journal entries payload is empty")
	}
	parts, msgs := compactJournalRequest(entries)
	resp, err := c.SendMessageParts(ctx, ModelWriter, parts, msgs, nil, Silent())
	if err != nil {
		return "", fmt.Errorf("compact journal: %w", err)
	}
	return strings.TrimSpace(resp.Text), nil
}

// compactJournalRequest builds the (system prompt, user messages) payload for
// a CompactJournal call. Split out for unit testing without a network round
// trip.
func compactJournalRequest(entries string) (CacheableSystemPrompt, []Message) {
	parts := CacheableSystemPrompt{Stable: journalCompactionSystemPrompt}
	msgs := []Message{{
		Role: RoleUser,
		Content: []Block{{
			Type: BlockText,
			Text: "Compact these journal entries:\n\n" + entries,
		}},
	}}
	return parts, msgs
}
