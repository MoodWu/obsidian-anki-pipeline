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
	BatchID   string
	Log       *ProcessLog
}

func normalizeFileName(w string) string {
	return strings.ReplaceAll(w, " ", "_")
}

func ProcessWord(rawInput string, ctx *ProcessContext) (*WordEntry, error) {

	raw := strings.ToLower(strings.TrimSpace(rawInput))
	if raw == "" {
		return nil, nil
	}

	// 閻?phrase 濞村吋锚閸樻盯鏁嶉崼婵嗗綘闂佹鍣槐?
	isPhrase := strings.Contains(raw, " ")

	// lemma
	word, ok := ctx.Lemma.Get(raw)
	if !ok {
		word = raw
	}

	// 闁告ê顭烽崳?- 婵☆偀鍋撻柡灞诲劜濡叉悂宕ラ敃鈧崙锟犲捶閵娿儳鎽熼柛蹇涙櫜閼?
	if existing, ok := ctx.Dict.Words[word]; ok {
		fmt.Println("[SKIP - dict]", raw)
		// 濞?.md 闁哄倸娲ｅ▎銏㈡嫚鐠囨彃绲块悗鐟版湰閺嗭絾绌遍埄鍐х礀
		entry := loadWordFromFile(ctx.DictDir, word)
		var ankiErr error
		if entry != nil {
			ankiErr = AddToAnki(entry, ctx.AnkiDeck, ctx.AnkiModel)
		} else {
			// 濠碘€冲€归悘澶愬棘閸ワ附顐藉☉鎾崇Т閻°劑宕烽…鎺旂濞达綀娉曢弫銈団偓娑欘殔閸氣偓濞戞搩鍘惧▓鎴﹀极閻楀牆绁?
			ankiErr = AddToAnki(&existing, ctx.AnkiDeck, ctx.AnkiModel)
		}
		if ctx.Log != nil {
			status := "skip:anki-ok"
			if ankiErr != nil {
				status = "skip:anki-fail"
				fmt.Println("[Anki] skip failed:", raw, ankiErr)
			}
			ctx.Log.LogWord(ctx.BatchID, raw, word, status)
		}
		return nil, nil
	}

	file := filepath.Join(ctx.DictDir, normalizeFileName(word)+".md")
	if _, err := os.Stat(file); err == nil {
		// 闁哄倸娲ｅ▎銏⑩偓娑櫭﹢顏呮媴閸℃洜鐟濋柛锔哄妼閻⊙囧礂闂€鎰幀闁挎稓鍠庨惃鍓ф嫚閺囨氨鐭ら悗娑欘殔閸氣偓闁兼儳鍢茶ぐ?
		var ankiErr error
		if existing, ok := ctx.Dict.Words[raw]; ok {
			fmt.Println("[SKIP - file]", raw)
			entry := loadWordFromFile(ctx.DictDir, raw)
			if entry != nil {
				ankiErr = AddToAnki(entry, ctx.AnkiDeck, ctx.AnkiModel)
			} else {
				ankiErr = AddToAnki(&existing, ctx.AnkiDeck, ctx.AnkiModel)
			}
		}
		if ctx.Log != nil {
			status := "skip:anki-ok"
			if ankiErr != nil {
				status = "skip:anki-fail"
				fmt.Println("[Anki] skip failed:", raw, ankiErr)
			}
			ctx.Log.LogWord(ctx.BatchID, raw, raw, status)
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
		if ctx.Log != nil {
			ctx.Log.LogWord(ctx.BatchID, raw, raw, "fail")
		}
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

	// lemma缂傚倹鎸搁悺?
	ctx.Lemma.Set(raw, entry.Word)

	// 閻庢稒锚閸?
	ctx.Dict.Add(*entry)

	writeWordNoteWithDir(ctx.DictDir, entry)

	ankiErr := AddToAnki(entry, ctx.AnkiDeck, ctx.AnkiModel)

	if ctx.Log != nil {
		status := "new:anki-ok"
		if ankiErr != nil {
			status = "new:anki-fail"
			fmt.Println("[Anki] add failed:", raw, ankiErr)
		}
		ctx.Log.LogWord(ctx.BatchID, raw, entry.Word, status)
	}

	return entry, nil
}

// loadWordFromFile 濞?.md 闁哄倸娲ｅ▎銏㈡嫚鐠囨彃绲?WordEntry
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

	// 閻熸瑱绲鹃悗浠嬪触閸曨偆鎽熸繛?
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

		// 閻熸瑱绲鹃悗浠嬪礃閸涱収鍟?
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

	// 閻熸瑱绲鹃悗?phonetic (闁?# word 濞戞柨顑呴幃妤呮儍閸曨垳鍙鹃柡?
	// 闁哄秶鍘х槐? # word\n/phonetic/\n
	title := "# " + word
	if idx := strings.Index(content, title); idx != -1 {
		// 闁瑰灚鍎抽崺宀勫冀閸ヮ剦鏆柛姘捣濞堟垿宕橀崨顓у晣
		afterTitle := content[idx+len(title):]
		// 闁哄被鍎叉竟姗€妫呴搹顐ゅ灱 /.../闁挎稑濂旂换姘舵偩濞嗘劖鐏橀柡?
		if start := strings.Index(afterTitle, "/"); start != -1 && start < len(afterTitle)-1 {
			remaining := afterTitle[start+1:]
			if end := strings.Index(remaining, "/"); end != -1 {
				entry.Phonetic = "/" + remaining[:end] + "/"
			}
		}
	}

	return entry
}
