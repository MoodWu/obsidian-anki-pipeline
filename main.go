package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultAIProvider = "ollama"
	defaultAIModel    = "deepseek-chat"
	defaultAnkiDeck   = "背单词"
	defaultAnkiModel  = "记单词"
)

func main() {

	if len(os.Args) < 2 {
		fmt.Println("用法:")
		fmt.Println("  scan <dir> [--dry-run] [--provider=ollama|openai] [--model=model_name] [--api-key=key] [--anki-deck=name] [--anki-model=name]")
		fmt.Println("  add <word1> [word2 ...] [--provider=ollama|openai] [--model=model_name] [--api-key=key] [--anki-deck=name] [--anki-model=name]")
		fmt.Println("  batch <file> [--provider=ollama|openai] [--model=model_name] [--api-key=key] [--anki-deck=name] [--anki-model=name]")
		fmt.Println("  export <file>")
		return
	}

	switch os.Args[1] {

	// ========================
	// 🔍 SCAN
	// ========================
	case "scan":
		start := time.Now()
		args := os.Args[2:]
		args, provider, model, apiKey, ankiDeck, ankiModel := extractAIOptions(args)

		if len(args) < 1 {
			fmt.Println("用法: scan <dir> [--dry-run] [--provider=ollama|openai] [--api-key=key] [--anki-deck=name] [--anki-model=name]")
			return
		}

		dir := args[0]
		dryRun := len(args) > 1 && args[1] == "--dry-run"

		dictDir := filepath.Join(dir, "dict")
		jsonPath := filepath.Join(dir, "dictionary.json")
		lemmaPath := filepath.Join(dir, "lemma.json")

		aiClient, err := NewAIClient(provider, model, apiKey)
		if err != nil {
			log.Fatal(err)
		}

		dict := NewDictionary(jsonPath)
		if err := dict.Load(); err != nil {
			log.Fatal(err)
		}

		lemma := NewLemmaStore(lemmaPath)
		if err := lemma.Load(); err != nil {
			log.Fatal(err)
		}

		ctx := &ProcessContext{
			Dict:      dict,
			Lemma:     lemma,
			AI:        aiClient,
			DictDir:   dictDir,
			DryRun:    dryRun,
			AnkiDeck:  ankiDeck,
			AnkiModel: ankiModel,
		}

		if err := ScanObsidian(dir, ctx); err != nil {
			log.Fatal(err)
		}

		// 统一保存（重要）
		if !dryRun {
			if err := dict.Save(); err != nil {
				log.Fatal(err)
			}
			if err := lemma.Save(); err != nil {
				log.Fatal(err)
			}
		}

		fmt.Println("scan 完成")
		fmt.Printf("耗时: %.2fs\n", time.Since(start).Seconds())

	// ========================
	// ➕ ADD（手动加词）
	// ========================
	case "add":
		start := time.Now()
		args := os.Args[2:]
		args, provider, model, apiKey, ankiDeck, ankiModel := extractAIOptions(args)

		if len(args) < 1 {
			fmt.Println("用法: add <word1> [word2 ...] [--provider=ollama|openai] [--api-key=key] [--anki-deck=name] [--anki-model=name]")
			return
		}

		aiClient, err := NewAIClient(provider, model, apiKey)
		if err != nil {
			log.Fatal(err)
		}

		parent := "."
		dictDir := filepath.Join(parent, "dict")
		jsonPath := filepath.Join(parent, "dictionary.json")
		lemmaPath := filepath.Join(parent, "lemma.json")

		dict := NewDictionary(jsonPath)
		if err := dict.Load(); err != nil {
			log.Fatal(err)
		}

		lemma := NewLemmaStore(lemmaPath)
		if err := lemma.Load(); err != nil {
			log.Fatal(err)
		}

		ctx := &ProcessContext{
			Dict:      dict,
			Lemma:     lemma,
			AI:        aiClient,
			DictDir:   dictDir,
			DryRun:    false,
			AnkiDeck:  ankiDeck,
			AnkiModel: ankiModel,
		}

		for _, w := range args {
			ProcessWord(w, ctx)
		}

		// 统一保存
		if err := dict.Save(); err != nil {
			log.Fatal(err)
		}
		if err := lemma.Save(); err != nil {
			log.Fatal(err)
		}

		fmt.Println("add 完成")
		fmt.Printf("耗时: %.2fs\n", time.Since(start).Seconds())

	// ========================
	// 📦 BATCH（文件批量）
	// ========================
	case "batch":
		start := time.Now()
		args := os.Args[2:]
		args, provider, model, apiKey, ankiDeck, ankiModel := extractAIOptions(args)

		if len(args) < 1 {
			fmt.Println("用法: batch <file> [--provider=ollama|openai] [--api-key=key] [--anki-deck=name] [--anki-model=name]")
			return
		}

		file := args[0]

		aiClient, err := NewAIClient(provider, model, apiKey)
		if err != nil {
			log.Fatal(err)
		}

		parent := "."
		dictDir := filepath.Join(parent, "dict")
		jsonPath := filepath.Join(parent, "dictionary.json")
		lemmaPath := filepath.Join(parent, "lemma.json")

		dict := NewDictionary(jsonPath)
		if err := dict.Load(); err != nil {
			log.Fatal(err)
		}

		lemma := NewLemmaStore(lemmaPath)
		if err := lemma.Load(); err != nil {
			log.Fatal(err)
		}

		ctx := &ProcessContext{
			Dict:      dict,
			Lemma:     lemma,
			AI:        aiClient,
			DictDir:   dictDir,
			DryRun:    false,
			AnkiDeck:  ankiDeck,
			AnkiModel: ankiModel,
		}

		f, err := os.Open(file)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			ProcessWord(line, ctx)
		}

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}

		// 统一保存
		if err := dict.Save(); err != nil {
			log.Fatal(err)
		}
		if err := lemma.Save(); err != nil {
			log.Fatal(err)
		}

		fmt.Println("batch 完成")
		fmt.Printf("耗时: %.2fs\n", time.Since(start).Seconds())

	// ========================
	// 📤 EXPORT
	// ========================
	case "export":
		start := time.Now()

		if len(os.Args) < 3 {
			fmt.Println("用法: export <file>")
			return
		}

		file := os.Args[2]

		dict := NewDictionary("dictionary.json")
		if err := dict.Load(); err != nil {
			log.Fatal(err)
		}

		if err := dict.ExportMarkdown(file); err != nil {
			log.Fatal(err)
		}

		fmt.Println("export 完成")
		fmt.Printf("耗时: %.2fs\n", time.Since(start).Seconds())

	default:
		fmt.Println("未知命令:", os.Args[1])
	}
}

func extractAIOptions(args []string) ([]string, string, string, string, string, string) {
	provider := getEnvDefault("AI_PROVIDER", defaultAIProvider)
	model := getEnvDefault("AI_MODEL", "")
	apiKey := getEnvDefault("OPENAI_API_KEY", "")
	ankiDeck := getEnvDefault("ANKI_DECK", defaultAnkiDeck)
	ankiModel := getEnvDefault("ANKI_MODEL", defaultAnkiModel)
	filtered := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--provider=") {
			provider = strings.TrimPrefix(arg, "--provider=")
			continue
		}
		if arg == "--provider" && i+1 < len(args) {
			provider = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(arg, "--model=") {
			model = strings.TrimPrefix(arg, "--model=")
			continue
		}
		if arg == "--model" && i+1 < len(args) {
			model = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(arg, "--api-key=") {
			apiKey = strings.TrimPrefix(arg, "--api-key=")
			continue
		}
		if arg == "--api-key" && i+1 < len(args) {
			apiKey = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(arg, "--anki-deck=") {
			ankiDeck = strings.TrimPrefix(arg, "--anki-deck=")
			continue
		}
		if arg == "--anki-deck" && i+1 < len(args) {
			ankiDeck = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(arg, "--anki-model=") {
			ankiModel = strings.TrimPrefix(arg, "--anki-model=")
			continue
		}
		if arg == "--anki-model" && i+1 < len(args) {
			ankiModel = args[i+1]
			i++
			continue
		}
		filtered = append(filtered, arg)
	}

	if model == "" {
		model = defaultModelForProvider(provider)
	}

	return filtered, provider, model, apiKey, ankiDeck, ankiModel
}

func getEnvDefault(key, defaultValue string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	return value
}

/*
func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法:")
		fmt.Println("  go run . add <word>     # 添加单个单词")
		fmt.Println("  go run . batch          # 批量处理 words.txt")
		fmt.Println("  go run . list           # 列出所有单词")
		fmt.Println("  go run . export         # 导出 Markdown")
		fmt.Println("  go run . review         # 随机复习")
		return
	}

	ollama := NewOllamaClient("qwen2.5:3b")
	dict := NewDictionary("dictionary.json")
	if err := dict.Load(); err != nil {
		log.Fatal(err)
	}

	switch os.Args[1] {
	case "add":
		if len(os.Args) < 3 {
			fmt.Println("请提供单词: go run . add hello")
			return
		}
		word := strings.ToLower(os.Args[2])
		fmt.Printf("正在查询: %s...\n", word)

		entry, err := ollama.GenerateWordEntry(word)
		if err != nil {
			log.Printf("生成失败: %v", err)
			return
		}

		dict.Add(*entry)
		if err := dict.Save(); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("已添加: %s - %s\n", entry.Word, entry.Meaning)

	case "batch":
		wordsFile := "words.txt"
		if len(os.Args) > 2 {
			wordsFile = os.Args[2]
		}

		file, err := os.Open(wordsFile)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			word := strings.TrimSpace(scanner.Text())
			if word == "" || strings.HasPrefix(word, "#") {
				continue
			}

			// 跳过已存在的
			if _, exists := dict.Words[word]; exists {
				fmt.Printf("跳过已存在: %s\n", word)
				continue
			}

			fmt.Printf("处理: %s...\n", word)
			entry, err := ollama.GenerateWordEntry(word)
			if err != nil {
				log.Printf("失败 %s: %v", word, err)
				continue
			}

			dict.Add(*entry)
			dict.Save() // 实时保存
			fmt.Printf("✓ %s: %s\n", entry.Word, entry.Meaning)
		}

	case "list":
		for word, entry := range dict.Words {
			fmt.Printf("%s [%s] %s\n", word, entry.Phonetic, entry.Meaning)
		}

	case "export":
		md := dict.ExportMarkdown()
		if err := os.WriteFile("dictionary.md", []byte(md), 0644); err != nil {
			log.Fatal(err)
		}
		fmt.Println("已导出 dictionary.md")

	case "review":
		// 简单复习模式：随机显示单词测试
		fmt.Println("复习模式 - 输入任意键查看答案，q退出")
		// 实现略...
	}
}
*/
