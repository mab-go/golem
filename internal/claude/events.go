package claude

import "github.com/mab-go/golem/internal/logging"

const (
	eventTextDelta    logging.Event = "anthropic.text_delta"
	eventTokenSummary logging.Event = "anthropic.token_summary"
)
