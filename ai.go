package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type AIClient interface {
	GenerateWordEntry(word string) (*WordEntry, error)
	GenerateWordEntryBatch(words []string) (map[string]*WordEntry, map[string]error)
}

const (
	ProviderOllama = "ollama"
	ProviderOpenAI = "openai"

	defaultOllamaURL = "http://127.0.0.1:11434/api/generate"
	defaultOpenAIURL = "https://api.deepseek.com/chat/completions"
)

func defaultModelForProvider(provider string) string {
	switch strings.ToLower(provider) {
	case ProviderOpenAI:
		return "deepseek-chat"
	default:
		return "deepseek-chat"
	}
}

func envDefault(key, defaultValue string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return defaultValue
}

func NewAIClient(provider, model, apiKey, apiBase string, useAPIKeyHeader ...bool) (AIClient, error) {
	useKeyHeader := len(useAPIKeyHeader) > 0 && useAPIKeyHeader[0]
	provider = strings.ToLower(strings.TrimSpace(provider))
	if model == "" {
		model = defaultModelForProvider(provider)
	}

	switch provider {
	case ProviderOllama:
		return NewOllamaClient(model), nil
	case ProviderOpenAI:
		if apiKey == "" {
			return nil, fmt.Errorf("openai: api key is required, pass --api-key or set OPENAI_API_KEY")
		}
		return NewOpenAIClient(model, apiKey, apiBase, useKeyHeader), nil
	default:
		return nil, fmt.Errorf("unknown AI provider: %s", provider)
	}
}

type OpenAIClient struct {
	URL             string
	Model           string
	APIKey          string
	UseAPIKeyHeader bool
	Client          *http.Client
}

type openAIChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type openAIChatChoice struct {
	Message Message `json:"message"`
}

type openAIChatResponse struct {
	Choices []openAIChatChoice `json:"choices"`
}

func NewOpenAIClient(model, apiKey, apiBase string, useAPIKeyHeader bool) *OpenAIClient {
	return &OpenAIClient{
		URL:    func() string { if apiBase != "" { return apiBase }; return envDefault("OPENAI_API_BASE", defaultOpenAIURL) }(),
		Model:  model,
		APIKey: apiKey,
		UseAPIKeyHeader: useAPIKeyHeader,
		Client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *OpenAIClient) GenerateWordEntry(word string) (*WordEntry, error) {
	messages := BuildPrompt(word)

	reqBody := openAIChatRequest{
		Model:    c.Model,
		Messages: messages,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Println("error", err)
		return nil, err
	}

	// fmt.Println(string(data))
req, err := http.NewRequest("POST", c.URL, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		if c.UseAPIKeyHeader {
			req.Header.Set("api-key", c.APIKey)
		} else {
			req.Header.Set("Authorization", "Bearer "+c.APIKey)
		}
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai: failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai: request failed status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	var result openAIChatResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("openai: decode response failed: %w, body=%s", err, string(bodyBytes))
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("openai: empty choices, status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	cleaned := cleanJSONResponse(result.Choices[0].Message.Content)
	var entry []WordEntry
	if err := json.Unmarshal([]byte(cleaned), &entry); err != nil {
		return nil, err
	}
	if len(entry) == 0 {
		return nil, fmt.Errorf("empty entry")
	}

	filtered := make([]WordEntry, 0)
	for _, v := range entry {
		if v.Original != "" && v.Original == word {
			filtered = append(filtered, v)
		}
	}
	entry = filtered
	if len(entry) == 0 {
		return nil, fmt.Errorf("no matching entry found")
	}

	ret := entry[0]
	if ret.Original == "" {
		ret.Original = word
	}
	if len(entry) > 1 {
		for _, v := range entry {
			ret.Meaning = v.Type + ": " + v.Meaning + "\n" + v.Translation + "\n" + v.Example + "\n\n" + ret.Meaning
			ret.Note = v.Type + ": " + v.Note + "\n\n" + ret.Note
			ret.Example = v.Type + ": " + v.Example + "\n\n" + ret.Example
			ret.Translation = v.Type + ": " + v.Translation + "\n\n" + ret.Translation
		}
	}
	if ret.Translation == "" {
		ret.Translation = entry[0].Meaning
	}
	ret.Aliases = make([]string, 0)
	ret.AddedAt = time.Now().Format("2006-01-02")

	return &ret, nil
}

func (c *OpenAIClient) GenerateWordEntryBatch(words []string) (map[string]*WordEntry, map[string]error) {
	results := make(map[string]*WordEntry)
	errs := make(map[string]error)

	messages := BuildBatchPrompt(words)
	reqBody := openAIChatRequest{
		Model:    c.Model,
		Messages: messages,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		for _, w := range words {
			errs[w] = err
		}
		return results, errs
	}

	req, err := http.NewRequest("POST", c.URL, bytes.NewBuffer(data))
	if err != nil {
		for _, w := range words {
			errs[w] = err
		}
		return results, errs
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		if c.UseAPIKeyHeader {
			req.Header.Set("api-key", c.APIKey)
		} else {
			req.Header.Set("Authorization", "Bearer "+c.APIKey)
		}
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		for _, w := range words {
			errs[w] = err
		}
		return results, errs
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		for _, w := range words {
			errs[w] = fmt.Errorf("read body: %w", err)
		}
		return results, errs
	}

	if resp.StatusCode != http.StatusOK {
		for _, w := range words {
			errs[w] = fmt.Errorf("status=%d body=%s", resp.StatusCode, string(bodyBytes))
		}
		return results, errs
	}

	var result openAIChatResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		for _, w := range words {
			errs[w] = fmt.Errorf("decode: %w, body=%s", err, string(bodyBytes))
		}
		return results, errs
	}

	if len(result.Choices) == 0 {
		for _, w := range words {
			errs[w] = fmt.Errorf("empty choices, body=%s", string(bodyBytes))
		}
		return results, errs
	}

	cleaned := cleanJSONResponse(result.Choices[0].Message.Content)
	var entries []WordEntry
	if err := json.Unmarshal([]byte(cleaned), &entries); err != nil {
		for _, w := range words {
			errs[w] = fmt.Errorf("parse entries: %w", err)
		}
		return results, errs
	}

	// Index by Original
	indexed := make(map[string][]WordEntry)
	for _, e := range entries {
		key := strings.ToLower(strings.TrimSpace(e.Original))
		indexed[key] = append(indexed[key], e)
	}

	for _, w := range words {
		wl := strings.ToLower(strings.TrimSpace(w))
		matched, ok := indexed[wl]
		if !ok || len(matched) == 0 {
		errs[w] = fmt.Errorf("no entry in batch response")
			continue
		}
		ret := matched[0]
		if ret.Original == "" {
			ret.Original = w
		}
		if len(matched) > 1 {
			for _, v := range matched {
				ret.Meaning = v.Type + ": " + v.Meaning + "\n" + v.Translation + "\n" + v.Example + "\n\n" + ret.Meaning
				ret.Note = v.Type + ": " + v.Note + "\n\n" + ret.Note
				ret.Example = v.Type + ": " + v.Example + "\n\n" + ret.Example
				ret.Translation = v.Type + ": " + v.Translation + "\n\n" + ret.Translation
			}
		}
		if ret.Translation == "" {
			ret.Translation = matched[0].Meaning
		}
		ret.Aliases = make([]string, 0)
		ret.AddedAt = time.Now().Format("2006-01-02")
		results[w] = &ret
	}

	return results, errs
}
