package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var wordPattern = regexp.MustCompile(`==(.+?)==`)

func ScanObsidian(notesDir string, ctx *ProcessContext) error {

	syncDict(ctx.DictDir, ctx.Dict, ctx.DryRun)

	return filepath.Walk(notesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		contentBytes, _ := os.ReadFile(path)
		content := string(contentBytes)

		matches := wordPattern.FindAllStringSubmatch(content, -1)
		if len(matches) == 0 {
			return nil
		}

		updated := content

		for _, m := range matches {
			raw := strings.ToLower(strings.TrimSpace(m[1]))
			if raw == "" {
				continue
			}

			// ⭐ 调统一入口
			entry, _ := ProcessWord(raw, ctx)

			// ⭐ 获取 lemma（无论是否新建）
			word, ok := ctx.Lemma.Get(raw)
			if !ok && entry != nil {
				word = entry.Word
			}
			if word == "" {
				word = raw
			}

			// ⭐ 替换为 Obsidian 链接
			old := m[0]
			new := "[[dict/" + word + "|" + word + "]]"

			updated = strings.ReplaceAll(updated, old, new)
		}

		// dry-run 不写文件
		if ctx.DryRun {
			return nil
		}

		// 写回文件
		if updated != content {
			if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
				return err
			}
		}

		return nil
	})
}

func syncDict(dictDir string, dict *Dictionary, dryRun bool) {

	files, _ := os.ReadDir(dictDir)

	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".md") {
			continue
		}

		word := strings.TrimSuffix(f.Name(), ".md")

		if _, ok := dict.Words[word]; !ok {
			fmt.Println("[SYNC add json]", word)
			if !dryRun {
				dict.Words[word] = WordEntry{Word: word}
			}
		}
	}

	for word, entry := range dict.Words {
		path := filepath.Join(dictDir, word+".md")

		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Println("[SYNC add file]", word)
			if !dryRun {
				writeWordNoteWithDir(dictDir, &entry)
			}
		}
	}

	if !dryRun {
		dict.Save()
	}
}

func writeWordNoteWithDir(dictDir string, entry *WordEntry) error {

	os.MkdirAll(dictDir, 0755)

	aliasStr := strings.Join(entry.Aliases, ", ")

	content := fmt.Sprintf(`---
aliases: [%s]
---

# %s

%s

## Meaning
%s

## Example
%s

## Translation
%s

## Cloze
%s

## Note
%s
`,
		aliasStr,
		entry.Word,
		entry.Phonetic,
		entry.Meaning,
		entry.Example,
		entry.Translation,
		replaceClozeVariables(entry.Cloze),
		entry.Note,
	)

	return os.WriteFile(filepath.Join(dictDir, entry.Word+".md"), []byte(content), 0644)
}
