package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type AIClient interface {
	GenerateWordEntry(word string) (*WordEntry, error)
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

func NewAIClient(provider, model, apiKey string) (AIClient, error) {
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
		return NewOpenAIClient(model, apiKey), nil
	default:
		return nil, fmt.Errorf("unknown AI provider: %s", provider)
	}
}

type OpenAIClient struct {
	URL    string
	Model  string
	APIKey string
	Client *http.Client
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

func NewOpenAIClient(model, apiKey string) *OpenAIClient {
	return &OpenAIClient{
		URL:    defaultOpenAIURL,
		Model:  model,
		APIKey: apiKey,
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
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("openai: empty choices")
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
