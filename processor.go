package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ProcessContext struct {
	Dict      *Dictionary
	Lemma     *LemmaStore
	AI        AIClient
	DictDir   string
	DryRun    bool
	AnkiDeck  string
	AnkiModel string
}

func normalizeFileName(w string) string {
	return strings.ReplaceAll(w, " ", "_")
}

func ProcessWord(rawInput string, ctx *ProcessContext) (*WordEntry, error) {

	raw := strings.ToLower(strings.TrimSpace(rawInput))
	if raw == "" {
		return nil, nil
	}

	// ⭐ phrase 优先（关键）
	isPhrase := strings.Contains(raw, " ")

	// lemma
	word, ok := ctx.Lemma.Get(raw)
	if !ok {
		word = raw
	}

	// 去重 - 检查是否已在字典中
	if existing, ok := ctx.Dict.Words[word]; ok {
		fmt.Println("[SKIP - dict]", raw)
		// 从 .md 文件读取完整信息
		entry := loadWordFromFile(ctx.DictDir, word)
		if entry != nil {
			AddToAnki(entry, ctx.AnkiDeck, ctx.AnkiModel)
		} else {
			// 如果文件不存在，使用字典中的数据
			AddToAnki(&existing, ctx.AnkiDeck, ctx.AnkiModel)
		}
		return nil, nil
	}

	file := filepath.Join(ctx.DictDir, normalizeFileName(word)+".md")
	if _, err := os.Stat(file); err == nil {
		// 文件存在但不在字典中？尝试从字典获取
		if existing, ok := ctx.Dict.Words[raw]; ok {
			fmt.Println("[SKIP - file]", raw)
			entry := loadWordFromFile(ctx.DictDir, raw)
			if entry != nil {
				AddToAnki(entry, ctx.AnkiDeck, ctx.AnkiModel)
			} else {
				AddToAnki(&existing, ctx.AnkiDeck, ctx.AnkiModel)
			}
		}
		return nil, nil
	}

	fmt.Println("[PROCESS]", raw)

	if ctx.DryRun {
		return nil, nil
	}

	entry, err := ctx.AI.GenerateWordEntry(raw)
	if err != nil {
		fmt.Println("process error", err)
		return nil, err
	}
	// fmt.Println("get entry", entry)
	// fallback type
	if entry.Type == "" {
		if isPhrase {
			entry.Type = "phrase"
		} else {
			entry.Type = "word"
		}
	}

	// lemma缓存
	ctx.Lemma.Set(raw, entry.Word)

	// 存储
	ctx.Dict.Add(*entry)

	writeWordNoteWithDir(ctx.DictDir, entry)

	AddToAnki(entry, ctx.AnkiDeck, ctx.AnkiModel)

	return entry, nil
}

// loadWordFromFile 从 .md 文件读取 WordEntry
func loadWordFromFile(dictDir, word string) *WordEntry {
	file := filepath.Join(dictDir, normalizeFileName(word)+".md")
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}

	content := string(data)
	entry := &WordEntry{
		Word: word,
	}

	// 解析各字段
	lines := strings.Split(content, "\n")
	inSection := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## Meaning") {
			inSection = "Meaning"
			continue
		}
		if strings.HasPrefix(line, "## Example") {
			inSection = "Example"
			continue
		}
		if strings.HasPrefix(line, "## Translation") {
			inSection = "Translation"
			continue
		}
		if strings.HasPrefix(line, "## Cloze") {
			inSection = "Cloze"
			continue
		}
		if strings.HasPrefix(line, "## Note") {
			inSection = "Note"
			continue
		}

		// 解析内容
		if inSection == "Meaning" && strings.HasPrefix(line, "noun") || strings.HasPrefix(line, "verb") || strings.HasPrefix(line, "adj") || strings.HasPrefix(line, "phrase") {
			entry.Type = strings.TrimSpace(strings.Split(line, " ")[0])
		} else if inSection == "Meaning" && line != "" && entry.Meaning == "" {
			entry.Meaning = line
		} else if inSection == "Example" && line != "" && entry.Example == "" {
			entry.Example = line
		} else if inSection == "Translation" && line != "" && entry.Translation == "" {
			entry.Translation = line
		} else if inSection == "Cloze" && line != "" && entry.Cloze == "" {
			entry.Cloze = line
		} else if inSection == "Note" && line != "" && entry.Note == "" {
			entry.Note = line
		}
	}

	// 解析 phonetic (在 # word 之后的音标)
	// 格式: # word\n/phonetic/\n
	title := "# " + word
	if idx := strings.Index(content, title); idx != -1 {
		// 找到标题后的内容
		afterTitle := content[idx+len(title):]
		// 查找音标 /.../，保留斜杠
		if start := strings.Index(afterTitle, "/"); start != -1 && start < len(afterTitle)-1 {
			remaining := afterTitle[start+1:]
			if end := strings.Index(remaining, "/"); end != -1 {
				entry.Phonetic = "/" + remaining[:end] + "/"
			}
		}
	}

	return entry
}
