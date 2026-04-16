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

	// 去重
	if _, ok := ctx.Dict.Words[word]; ok {
		fmt.Println("[SKIP]", raw)
		return nil, nil
	}

	file := filepath.Join(ctx.DictDir, normalizeFileName(word)+".md")
	if _, err := os.Stat(file); err == nil {
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
