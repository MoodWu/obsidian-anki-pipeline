package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type OllamaClient struct {
	URL    string
	Model  string
	Client *http.Client
}

type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenerateResponse struct {
	Response string `json:"response"`
}

func NewOllamaClient(model string) *OllamaClient {
	url := defaultOllamaURL
	if envURL := strings.TrimSpace(os.Getenv("OLLAMA_URL")); envURL != "" {
		url = envURL
	}

	return &OllamaClient{
		URL:    url,
		Model:  model,
		Client: &http.Client{Timeout: 120 * time.Second},
	}
}

func BuildPrompt(content string) []Message {
	system := `你是一个专业的Anki卡片整理师，可以为给定的单词或短语生成英语学习卡片(JSON)。整理的步骤：
1. word字段使用输入的原型，如果输入不是原型必须转换为原型，动词的其他时态转化为原型，名词的复数形式转化为单数，如果输入是一个动名词，默认为动词的现在进行时，同时要在note中标注出此单词的现在进行时同时是一个名词
2. 判断类型：word / phrase
3. phonetic字段 word字段的国际音标，如果word字段与original不同，一定要输出word的读音
4. note字段包含一些特殊说明，比如发音规则、词根词缀、同根词、构词法等，如果动词的现在进行时是名词，也需要在此说明，如果original的读音有些特殊规则，也在这里说明
5.note字段也要有中英两种输出
6. original 字段使用输入词
7. meaning字段是单词的释义，格式是 词性 加上 释义，如果有多种词性，则不同词性的释义间以回车分隔，如果同一词性有不同释义，则不同释义间用回车分隔，先用英文解释，然后回车再附上中文释义
8. example 和 cloze 必须使用原词（上下文自然），且example与cloze不要相同
9. translation 是 example的中文释义  

返回：
{
"word": "",
"original": "",
"type": "",
"phonetic": "",
"meaning": "",	
"example": "",
"translation": "",
"cloze": "",
"note": "",
"aliases": []
}

只返回JSON`

	ret := []Message{
		{Role: "system", Content: system},
		{Role: "user", Content: content},
	}

	return ret

}


func BuildBatchPrompt(words []string) []Message {
	wordList := strings.Join(words, ", ")
	system := "你是一个专业的Anki卡片整理师，请为以下多个英语单词/短语各生成一个学习卡片(JSON)。\n" +
		"整理规则：\n" +
		"1. word字段使用输入的原形，如果不是原形必须转为原形\n" +
		"2. 判断类型：word / phrase\n" +
		"3. phonetic字段是word的国际音标\n" +
		"4. note字段包含发音规则、词根词缀等特殊说明（中英两种输出）\n" +
		"5. original字段使用输入词\n" +
		"6. meaning字段格式：词性+释义，先英文后中文，多种词性用回车分隔\n" +
		"7. example和cloze必须使用原词，且不要相同\n" +
		"8. translation是example的中文释义\n\n" +
		"返回一个JSON数组，每个元素格式：\n" +
		"{\"word\":\"\",\"original\":\"\",\"type\":\"\",\"phonetic\":\"\",\"meaning\":\"\",\"example\":\"\",\"translation\":\"\",\"cloze\":\"\",\"note\":\"\",\"aliases\":[]}\n\n" +
		"只返回JSON数组，不要其他文字。必须为每个输入单词都生成条目，顺序与输入一致。"

	return []Message{
		{Role: "system", Content: system},
		{Role: "user", Content: wordList},
	}
}
func (c *OllamaClient) GenerateWordEntry(word string) (*WordEntry, error) {

	// reqBody := GenerateRequest{
	// 	Model:  c.Model,
	// 	Prompt: prompt,
	// 	Stream: false,
	// }

	messages := BuildPrompt(word)

	reqBody := map[string]interface{}{
		"model":    c.Model,
		"messages": messages,
	}
	data, _ := json.Marshal(reqBody)

	// fmt.Println(string(data))

	req, err := http.NewRequest("POST", c.URL, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey := strings.TrimSpace(os.Getenv("OLLAMA_API_KEY")); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// resp, err := c.Client.Post(c.URL, "application/json", bytes.NewBuffer(data))
	resp, err := c.Client.Do(req)
	if err != nil {
		fmt.Println("error", err)
		return nil, err
	}
	defer resp.Body.Close()

	var result GenerateResponse
	json.NewDecoder(resp.Body).Decode(&result)

	// fmt.Println("Ollama响应:", result)
	cleaned := cleanJSONResponse(result.Response)
	// cleaned := result.Response

	// fmt.Println("Ollama 原始响应:", result.Response)
	// fmt.Println("Ollama 清理后响应:", cleaned)

	var entry []WordEntry
	var ret WordEntry
	entry = make([]WordEntry, 0)
	if err := json.Unmarshal([]byte(cleaned), &entry); err != nil {
		fmt.Println("JSON解析错误:", err)
		return nil, err
	}

	if len(entry) == 0 {
		return nil, fmt.Errorf("empty entry")
	}
	filtered := make([]WordEntry, 0)

	//删除不是原始词的输出
	for _, v := range entry {
		if v.Original != "" && v.Original == word {
			filtered = append(filtered, v)
		}
	}
	entry = filtered

	if len(entry) == 0 {
		return nil, fmt.Errorf("no matching entry found")
	}

	// fmt.Println(entry)
	ret = entry[0]

	// 设置默认值
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
	if entry[0].Translation == "" {
		ret.Translation = entry[0].Meaning // 如果没有翻译，用英文含义代替
	}
	ret.Aliases = make([]string, 0)
	ret.AddedAt = time.Now().Format("2006-01-02")

	return &ret, nil
}

// func cleanJSONResponse(s string) string {
// 	s = strings.TrimSpace(s)
// 	s = strings.TrimPrefix(s, "```json")
// 	s = strings.TrimPrefix(s, "```")
// 	s = strings.TrimSuffix(s, "```")
// 	return strings.TrimSpace(s)
// }

func (c *OllamaClient) GenerateWordEntryBatch(words []string) (map[string]*WordEntry, map[string]error) {
	results := make(map[string]*WordEntry)
	errs := make(map[string]error)

	messages := BuildBatchPrompt(words)
	reqBody := map[string]interface{}{
		"model":    c.Model,
		"messages": messages,
	}
	data, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", c.URL, bytes.NewBuffer(data))
	if err != nil {
		for _, w := range words {
			errs[w] = err
		}
		return results, errs
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey := strings.TrimSpace(os.Getenv("OLLAMA_API_KEY")); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		for _, w := range words {
			errs[w] = err
		}
		return results, errs
	}
	defer resp.Body.Close()

	var result GenerateResponse
	json.NewDecoder(resp.Body).Decode(&result)

	cleaned := cleanJSONResponse(result.Response)
	var entries []WordEntry
	if err := json.Unmarshal([]byte(cleaned), &entries); err != nil {
		for _, w := range words {
			errs[w] = fmt.Errorf("parse entries: %w", err)
		}
		return results, errs
	}

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

func cleanJSONResponse(s string) string {
	s = strings.TrimSpace(s)

	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		lines = lines[1 : len(lines)-1]
		s = strings.Join(lines, "\n")
	}

	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")

	if start >= 0 && end > start {
		s = s[start : end+1]
	}
	s = "[" + s + "]"
	return s
}
