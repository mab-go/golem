package claude

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/mab-go/golem/internal/logging"
)

// DefaultMaxTokens is the default upper bound on tokens per API call.
const DefaultMaxTokens int64 = 4096

// Role identifies a conversation turn author.
type Role string

// Role values as they appear in the Messages API.
const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// BlockType identifies a content block variant.
type BlockType string

// BlockType values we care about in the agentic loop. Other SDK block types
// (thinking, server_tool_use, etc.) are not emitted or consumed by golem.
const (
	BlockText       BlockType = "text"
	BlockToolUse    BlockType = "tool_use"
	BlockToolResult BlockType = "tool_result"
)

// Block is a single content block within a Message. Only the fields relevant
// to the block's Type are populated.
type Block struct {
	Type BlockType

	// Text block
	Text string

	// Tool-use block
	ID    string
	Name  string
	Input json.RawMessage

	// Tool-result block
	ToolUseID string
	Content   string
	IsError   bool

	// Tool-result image payload. Populated ONLY when Type == BlockToolResult
	// AND the tool produced an image (take_screenshot). Carries raw bytes;
	// base64 encoding happens at SDK conversion time.
	ImageMediaType string
	ImageData      []byte
}

// Message is one conversational turn.
type Message struct {
	Role    Role
	Content []Block
}

// ToolUse is a convenience view of a tool_use block.
type ToolUse struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// Tool describes a custom tool we expose to Claude.
type Tool struct {
	Name        string
	Description string
	// InputSchema follows JSON Schema 2020-12. Typically built via helpers in
	// tools.go; passed through as a map so we don't depend on SDK-internal types.
	InputSchema map[string]any
}

// Usage reports tokens consumed by a single API call.
type Usage struct {
	InputTokens        int64
	OutputTokens       int64
	CacheCreationInput int64
	CacheReadInput     int64
}

// Response is the resolved result of a streaming SendMessage call.
type Response struct {
	Text       string
	Blocks     []Block
	ToolUses   []ToolUse
	Usage      Usage
	StopReason string
	Model      string
}

// Client is the agent-facing wrapper over the Anthropic Go SDK. It hides the
// SDK's parameter construction and streaming plumbing so callers work with the
// small, stable Block/Message/Tool/Response types above.
type Client struct {
	sdk       anthropic.Client
	log       logging.Logger
	maxTokens int64
	metrics   *Metrics

	// TextDeltaFunc is called per content_block_delta with non-empty text
	// during streaming. The TUI sets this to publish deltas; nil means no-op.
	// Per-call overrides via WithTextDelta / Silent take precedence.
	TextDeltaFunc func(string)
}

// CallOption configures a single SendMessage / SendMessageParts call.
type CallOption func(*callOpts)

type callOpts struct {
	textDeltaFunc func(string)
}

// WithTextDelta overrides the client-level TextDeltaFunc for this call.
func WithTextDelta(fn func(string)) CallOption {
	return func(o *callOpts) { o.textDeltaFunc = fn }
}

// Silent suppresses text-delta publishing for this call.
func Silent() CallOption {
	return func(o *callOpts) { o.textDeltaFunc = func(string) {} }
}

// NewClient constructs a Client. If apiKey is empty the SDK falls back to the
// ANTHROPIC_API_KEY environment variable. A non-positive maxTokens is replaced
// with DefaultMaxTokens. A nil metrics is a no-op -- SendMessage still succeeds
// but per-model totals are not tracked.
func NewClient(apiKey string, maxTokens int64, metrics *Metrics, log logging.Logger) *Client {
	if log == nil {
		log = logging.NewDefaultLogger()
	}
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	var opts []option.RequestOption
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	return &Client{
		sdk:       anthropic.NewClient(opts...),
		log:       log,
		maxTokens: maxTokens,
		metrics:   metrics,
	}
}

// SendMessage issues a streaming Messages request with a single system-prompt
// string. The whole prompt is treated as cacheable. Callers that separate
// stable persona content from per-turn runtime settings should use
// SendMessageParts instead.
func (c *Client) SendMessage(
	ctx context.Context,
	model string,
	systemPrompt string,
	messages []Message,
	tools []Tool,
	opts ...CallOption,
) (*Response, error) {
	return c.SendMessageParts(ctx, model, CacheableSystemPrompt{Stable: systemPrompt}, messages, tools, opts...)
}

// SendMessageParts issues a streaming Messages request with the system prompt
// split into cacheable stable content and a dynamic tail. The stable block is
// marked with an ephemeral cache breakpoint; the tool catalog is also cached
// (its last entry carries the breakpoint so everything before it is covered).
// Partial text deltas are logged at Debug level for operator visibility.
func (c *Client) SendMessageParts(
	ctx context.Context,
	model string,
	parts CacheableSystemPrompt,
	messages []Message,
	tools []Tool,
	opts ...CallOption,
) (*Response, error) {
	if model == "" {
		return nil, fmt.Errorf("model is required")
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages is empty")
	}

	var co callOpts
	for _, o := range opts {
		o(&co)
	}
	deltaFn := c.TextDeltaFunc
	if co.textDeltaFunc != nil {
		deltaFn = co.textDeltaFunc
	}

	params := anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: c.maxTokens,
		Messages:  toSDKMessages(messages),
		System:    systemPromptBlocks(parts),
		Tools:     toSDKTools(tools),
	}

	stream := c.sdk.Messages.NewStreaming(ctx, params)
	return c.accumulate(stream, deltaFn)
}

// accumulate drains the SSE stream into a resolved Response. It is split out
// so unit tests can feed a fake stream via a custom ssestream.Stream.
func (c *Client) accumulate(stream streamSource, deltaFn func(string)) (*Response, error) {
	var msg anthropic.Message
	for stream.Next() {
		ev := stream.Current()
		if err := msg.Accumulate(ev); err != nil {
			return nil, fmt.Errorf("accumulate stream event: %w", err)
		}
		if ev.Type == "content_block_delta" {
			delta := ev.Delta
			if delta.Text != "" {
				c.log.WithField("delta", delta.Text).Debug(eventTextDelta)
				if deltaFn != nil {
					deltaFn(delta.Text)
				}
			}
		}
	}
	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("stream: %w", err)
	}

	resp := responseFromSDKMessage(&msg)
	c.metrics.Record(resp.Model, resp.Usage)
	return resp, nil
}

// streamSource is the tiny subset of ssestream.Stream we depend on. Defined
// here so tests can inject a fake.
type streamSource interface {
	Next() bool
	Current() anthropic.MessageStreamEventUnion
	Err() error
}

// --- SDK <-> internal type conversion ---------------------------------------

// systemPromptBlocks splits a CacheableSystemPrompt into up to two
// TextBlockParams. The stable block (if present) gets an ephemeral cache
// breakpoint; the dynamic block never carries one so per-turn settings cannot
// invalidate the cache.
func systemPromptBlocks(p CacheableSystemPrompt) []anthropic.TextBlockParam {
	stable := strings.TrimSpace(p.Stable)
	dynamic := strings.TrimSpace(p.Dynamic)

	var blocks []anthropic.TextBlockParam
	if stable != "" {
		blocks = append(blocks, anthropic.TextBlockParam{
			Text:         p.Stable,
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
		})
	}
	if dynamic != "" {
		blocks = append(blocks, anthropic.TextBlockParam{Text: p.Dynamic})
	}
	return blocks
}

func toSDKMessages(messages []Message) []anthropic.MessageParam {
	out := make([]anthropic.MessageParam, 0, len(messages))
	for _, m := range messages {
		blocks := make([]anthropic.ContentBlockParamUnion, 0, len(m.Content))
		for _, b := range m.Content {
			blocks = append(blocks, toSDKBlock(b))
		}
		switch m.Role {
		case RoleAssistant:
			out = append(out, anthropic.NewAssistantMessage(blocks...))
		default:
			out = append(out, anthropic.NewUserMessage(blocks...))
		}
	}
	// Advancing cache breakpoint: mark the last block of the last message
	// so the growing conversation caches cumulatively across turns. Next
	// turn the breakpoint moves forward; the API cache-hits the prefix up
	// to the previous breakpoint and writes from there.
	markLastBlockCached(out)
	return out
}

// markLastBlockCached sets an ephemeral cache_control breakpoint on the
// final content block of the final message. cache_control lives on the
// concrete block struct, so we mutate whichever Of* field is populated.
func markLastBlockCached(msgs []anthropic.MessageParam) {
	var last *anthropic.ContentBlockParamUnion
	for i := len(msgs) - 1; i >= 0 && last == nil; i-- {
		if n := len(msgs[i].Content); n > 0 {
			last = &msgs[i].Content[n-1]
		}
	}
	if last == nil {
		return
	}
	cc := anthropic.NewCacheControlEphemeralParam()
	switch {
	case last.OfText != nil:
		last.OfText.CacheControl = cc
	case last.OfToolUse != nil:
		last.OfToolUse.CacheControl = cc
	case last.OfToolResult != nil:
		last.OfToolResult.CacheControl = cc
	case last.OfImage != nil:
		last.OfImage.CacheControl = cc
	}
}

func toSDKBlock(b Block) anthropic.ContentBlockParamUnion {
	switch b.Type {
	case BlockToolUse:
		// Input is json.RawMessage; passed as any, it marshals back to itself
		// because json.RawMessage implements json.Marshaler.
		return anthropic.NewToolUseBlock(b.ID, b.Input, b.Name)
	case BlockToolResult:
		return toolResultBlock(b)
	default:
		return anthropic.NewTextBlock(b.Text)
	}
}

// toolResultBlock builds a tool_result block. If an image payload is present,
// the content is a mixed text+image array; otherwise it mirrors the SDK's
// NewToolResultBlock helper (text-only).
func toolResultBlock(b Block) anthropic.ContentBlockParamUnion {
	if len(b.ImageData) == 0 {
		return anthropic.NewToolResultBlock(b.ToolUseID, b.Content, b.IsError)
	}

	content := make([]anthropic.ToolResultBlockParamContentUnion, 0, 2)
	if b.Content != "" {
		content = append(content, anthropic.ToolResultBlockParamContentUnion{
			OfText: &anthropic.TextBlockParam{Text: b.Content},
		})
	}
	mediaType := b.ImageMediaType
	if mediaType == "" {
		mediaType = "image/png"
	}
	content = append(content, anthropic.ToolResultBlockParamContentUnion{
		OfImage: &anthropic.ImageBlockParam{
			Source: anthropic.ImageBlockParamSourceUnion{
				OfBase64: &anthropic.Base64ImageSourceParam{
					Data:      base64.StdEncoding.EncodeToString(b.ImageData),
					MediaType: anthropic.Base64ImageSourceMediaType(mediaType),
				},
			},
		},
	})

	return anthropic.ContentBlockParamUnion{
		OfToolResult: &anthropic.ToolResultBlockParam{
			ToolUseID: b.ToolUseID,
			Content:   content,
			IsError:   anthropic.Bool(b.IsError),
		},
	}
}

func toSDKTools(tools []Tool) []anthropic.ToolUnionParam {
	if len(tools) == 0 {
		return nil
	}
	out := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		param := anthropic.ToolParam{
			Name:        t.Name,
			InputSchema: anthropic.ToolInputSchemaParam{Properties: t.InputSchema["properties"]},
		}
		if t.Description != "" {
			param.Description = anthropic.String(t.Description)
		}
		if req, ok := t.InputSchema["required"].([]string); ok {
			param.InputSchema.Required = req
		}
		out = append(out, anthropic.ToolUnionParam{OfTool: &param})
	}
	// Cache the stable tool catalog. Anthropic caches up to the nearest
	// breakpoint, so marking the last tool covers everything above it.
	out[len(out)-1].OfTool.CacheControl = anthropic.NewCacheControlEphemeralParam()
	return out
}

func responseFromSDKMessage(msg *anthropic.Message) *Response {
	resp := &Response{
		Model:      msg.Model,
		StopReason: string(msg.StopReason),
		Usage: Usage{
			InputTokens:        msg.Usage.InputTokens,
			OutputTokens:       msg.Usage.OutputTokens,
			CacheCreationInput: msg.Usage.CacheCreationInputTokens,
			CacheReadInput:     msg.Usage.CacheReadInputTokens,
		},
	}
	var textParts []string
	for _, block := range msg.Content {
		switch v := block.AsAny().(type) {
		case anthropic.TextBlock:
			resp.Blocks = append(resp.Blocks, Block{Type: BlockText, Text: v.Text})
			textParts = append(textParts, v.Text)
		case anthropic.ToolUseBlock:
			resp.Blocks = append(resp.Blocks, Block{
				Type:  BlockToolUse,
				ID:    v.ID,
				Name:  v.Name,
				Input: append(json.RawMessage(nil), v.Input...),
			})
			resp.ToolUses = append(resp.ToolUses, ToolUse{
				ID:    v.ID,
				Name:  v.Name,
				Input: append(json.RawMessage(nil), v.Input...),
			})
		}
	}
	resp.Text = strings.Join(textParts, "")
	return resp
}
