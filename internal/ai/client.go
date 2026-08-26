// Package ai provides an OpenAI-compatible client for auto-fixing code.
package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"

	terr "github.com/HumanHorizon/testwarden/internal/errors"
	"github.com/HumanHorizon/testwarden/internal/retry"
)

// Client wraps the OpenAI-compatible API.
type Client struct {
	client    *openai.Client
	model     string
	timeout   time.Duration
	maxTokens int
}

// Config configures the AI client.
type Config struct {
	Endpoint  string
	APIKey    string
	Model     string
	Timeout   int
	MaxTokens int
}

// New creates a new Client from Config.
func New(cfg Config) *Client {
	config := openai.DefaultConfig(cfg.APIKey)
	if cfg.Endpoint != "" {
		config.BaseURL = cfg.Endpoint
	}
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	return &Client{
		client:    openai.NewClientWithConfig(config),
		model:     cfg.Model,
		timeout:   timeout,
		maxTokens: cfg.MaxTokens,
	}
}

// ErrEmptyResponse indicates the model returned no content.
var ErrEmptyResponse = errors.New("ai returned empty response")

// fixPrompts are specialized prompts per issue type.
var fixPrompts = map[string]string{
	"coverage": `You are a senior Go/TypeScript test engineer.
The file below has low test coverage. Add the MINIMAL number of test cases
required to cover the untested branches. Prefer table-driven tests.
Return the complete updated source file inside a single fenced code block.`,
	"mock": `You are a senior test engineer reviewing a unit test that mocks
a real I/O dependency (database, HTTP, filesystem). Such mocks belong in
INTEGRATION tests, not unit tests. Refactor the file so the unit test does
not import or mock the real dependency. Remove mock setup and use a
lightweight in-memory fake or interface instead.
Return the complete updated source file inside a single fenced code block.`,
	"gap": `You are a senior test engineer. The lines below are NOT covered
by unit OR integration tests. Add tests that exercise these specific lines.
Use table-driven tests where appropriate.
Return the complete updated source file inside a single fenced code block.`,
	"default": `You are a senior test engineer. You receive a problem description
and a code file. Respond ONLY with the complete updated file content inside
a single fenced code block. Do not add explanations outside the code block.
Preserve formatting and imports. Make minimal changes to fix the problem.`,
}

// Fix sends a problem + file content to the AI and returns the full updated file content.
func (c *Client) Fix(ctx context.Context, problem, problemType, rule, filePath, language, content string) (string, error) {
	systemPrompt := fixPrompts[problemType]
	if systemPrompt == "" {
		systemPrompt = fixPrompts["default"]
	}

	userPrompt := fmt.Sprintf(
		"Problem type: %s\nProblem: %s\nRule: %s\nFile: %s\nLanguage: %s\n\nCurrent file:\n```\n%s\n```\n\nReturn the complete updated file in a fenced code block.",
		problemType, problem, rule, filePath, language, content,
	)

	maxTokens := c.maxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		MaxTokens: maxTokens,
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp openai.ChatCompletionResponse
	err := retry.Do(ctx, retry.Default(), func() error {
		r, err := c.client.CreateChatCompletion(ctx, req)
		if err != nil {
			if retry.IsTransient(err) {
				return err
			}
			return terr.Wrap(terr.CodeAI, "request failed", err)
		}
		if len(r.Choices) == 0 {
			return ErrEmptyResponse
		}
		resp = r
		return nil
	})
	if err != nil {
		return "", err
	}

	out := extractCode(resp.Choices[0].Message.Content)
	if out == "" {
		return "", ErrEmptyResponse
	}
	return out, nil
}

// FixStream streams the AI response token-by-token. The onChunk callback
// is invoked for each token (may be empty strings). Returns the full
// extracted code on success.
func (c *Client) FixStream(
	ctx context.Context,
	problem, problemType, rule, filePath, language, content string,
	onChunk func(string),
) (string, error) {
	systemPrompt := fixPrompts[problemType]
	if systemPrompt == "" {
		systemPrompt = fixPrompts["default"]
	}

	userPrompt := fmt.Sprintf(
		"Problem type: %s\nProblem: %s\nRule: %s\nFile: %s\nLanguage: %s\n\nCurrent file:\n```\n%s\n```\n\nReturn the complete updated file in a fenced code block.",
		problemType, problem, rule, filePath, language, content,
	)

	maxTokens := c.maxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	req := openai.ChatCompletionRequest{
		Model:  c.model,
		Stream: true,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		MaxTokens: maxTokens,
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var stream *openai.ChatCompletionStream
	err := retry.Do(ctx, retry.Default(), func() error {
		s, err := c.client.CreateChatCompletionStream(ctx, req)
		if err != nil {
			if retry.IsTransient(err) {
				return err
			}
			return terr.Wrap(terr.CodeAI, "stream failed", err)
		}
		stream = s
		return nil
	})
	if err != nil {
		return "", err
	}
	if stream == nil {
		return "", ErrEmptyResponse
	}
	defer stream.Close()

	var full strings.Builder
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("ai stream recv: %w", err)
		}
		if len(resp.Choices) == 0 {
			continue
		}
		chunk := resp.Choices[0].Delta.Content
		if chunk != "" {
			full.WriteString(chunk)
			if onChunk != nil {
				onChunk(chunk)
			}
		}
	}

	out := extractCode(full.String())
	if out == "" {
		return "", ErrEmptyResponse
	}
	return out, nil
}

var fenceRe = regexp.MustCompile("(?s)```[a-zA-Z]*\\n(.*?)```")

// extractCode pulls the content out of the first ``` fenced block.
// If no fence is present, returns the original string trimmed.
func extractCode(s string) string {
	match := fenceRe.FindStringSubmatch(s)
	if len(match) >= 2 {
		return strings.TrimRight(match[1], "\n")
	}
	return strings.TrimSpace(s)
}
