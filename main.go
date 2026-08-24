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

	// 输出所有接收到的参数
	fmt.Println("========== 接收到的参数 ==========")
	for i, arg := range os.Args {
		fmt.Printf("[%d] %s\n", i, arg)
	}
	fmt.Println("====================================")

	if len(os.Args) < 2 {
		fmt.Println("用法:")
		fmt.Println("  scan <dir> [--dry-run] [--provider=ollama|openai] [--model=model_name] [--api-key=key] [--api-base=url] [--api-key-header] [--batchcount=N] [--anki-deck=name] [--anki-model=name]")
		fmt.Println("  add <word1> [word2 ...] [--provider=ollama|openai] [--model=model_name] [--api-key=key] [--api-base=url] [--api-key-header] [--batchcount=N] [--anki-deck=name] [--anki-model=name]")
		fmt.Println("  batch <file> [--provider=ollama|openai] [--model=model_name] [--api-key=key] [--api-base=url] [--api-key-header] [--batchcount=N] [--anki-deck=name] [--anki-model=name]")
		fmt.Println("  process [dir] [--batchcount=N] [--provider=ollama|openai] [--model=model_name] [--api-key=key] [--api-base=url] [--api-key-header] [--anki-deck=name] [--anki-model=name]")
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
		args, provider, model, apiKey, apiBase, ankiDeck, ankiModel, apiKeyHeader, batchCount := extractAIOptions(args)

		// 调试：输出解析后的参数
		fmt.Printf("provider: %s\n", provider)
		fmt.Printf("apiBase: %s\n", apiBase)
		fmt.Printf("model: %s\n", model)
		fmt.Printf("apiKey: %s\n", apiKey)
		fmt.Printf("ankiDeck: %s\n", ankiDeck)
		fmt.Printf("ankiModel: %s\n", ankiModel)

		if len(args) < 1 {
			fmt.Println("用法: scan <dir> [--dry-run] [--provider=ollama|openai] [--api-key=key] [--api-base=url] [--api-key-header] [--batchcount=N] [--anki-deck=name] [--anki-model=name]")
			return
		}

		dir := args[0]
		dryRun := false
		for _, a := range args[1:] {
			if a == "--dry-run" {
				dryRun = true
			}
		}

		dictDir := filepath.Join(dir, "dict")
		jsonPath := filepath.Join(dir, "dictionary.json")
		lemmaPath := filepath.Join(dir, "lemma.json")

		aiClient, err := NewAIClient(provider, model, apiKey, apiBase, apiKeyHeader)
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

		pendingPath := filepath.Join(dir, "pending.json")
		pq := NewPendingQueue(pendingPath)
		if err := pq.Load(); err != nil {
			log.Fatal(err)
		}

		procLog := NewProcessLog(filepath.Join(dir, "log.json"))
		procLog.Load()
		batchID := procLog.StartBatch("scan", ankiDeck, ankiModel)

		ctx := &ProcessContext{
			Dict:        dict,
			Lemma:       lemma,
			AI:          aiClient,
			DictDir:     dictDir,
			DryRun:      dryRun,
			PendingOnly: true,
			Pending:     pq,
			AnkiDeck:  ankiDeck,
			AnkiModel: ankiModel,
			BatchID:   batchID,
			Log:       procLog,
		}

		if err := ScanObsidian(dir, ctx); err != nil {
			log.Fatal(err)
		}

		if !dryRun {
			if err := dict.Save(); err != nil {
				log.Fatal(err)
			}
			if err := lemma.Save(); err != nil {
				log.Fatal(err)
			}
			if err := procLog.Save(); err != nil {
				log.Fatal(err)
			}
			if err := pq.Save(); err != nil {
				log.Fatal(err)
			}

		// Phase 2: batch process the queue
		if !dryRun && pq.HasProcessable() {
			fmt.Println("\n--- Starting batch processing ---")

			processLog := NewProcessLog(filepath.Join(dir, "log.json"))
			processLog.Load()
			processBatchID := processLog.StartBatch("process", ankiDeck, ankiModel)

			pctx := &ProcessContext{
				Dict:      dict,
				Lemma:     lemma,
				AI:        aiClient,
				DictDir:   dictDir,
				AnkiDeck:  ankiDeck,
				AnkiModel: ankiModel,
				BatchID:   processBatchID,
				Log:       processLog,
			}

			totalProcessed := 0
			totalFailed := 0
			round := 0
			for pq.HasProcessable() {
				round++
				batch := pq.GetBatch(batchCount)
				if len(batch) == 0 {
					break
				}
				words := make([]string, len(batch))
				for i, b := range batch {
					words[i] = b.Word
				}
				fmt.Printf("\n--- Batch %d: %d words ---\n", round, len(words))

				errMap := ProcessWordBatch(words, pctx)
				for _, w := range words {
					if e, ok := errMap[w]; ok && e != nil {
						pq.RecordFailure(w, e.Error())
						totalFailed++
						fmt.Printf("[FAIL] %s: %v\n", w, e)
					} else {
						pq.RecordSuccess(w)
						totalProcessed++
					}
				}

				if err := pq.Save(); err != nil {
					log.Fatal(err)
				}
				if err := dict.Save(); err != nil {
					log.Fatal(err)
				}
				if err := lemma.Save(); err != nil {
					log.Fatal(err)
				}
				if err := processLog.Save(); err != nil {
					log.Fatal(err)
				}
			}

			failed := pq.RemoveFailed()
			if err := pq.Save(); err != nil {
				log.Fatal(err)
			}
			if len(failed) > 0 {
				fmt.Printf("\nFailed words (3+ errors):\n")
				for _, f := range failed {
					fmt.Printf("  %s: %s\n", f.Word, f.LastError)
				}
			}
			pq.PrintStatus()
			fmt.Printf("Batch done: %d processed, %d failed\n", totalProcessed, totalFailed)
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
		args, provider, model, apiKey, apiBase, ankiDeck, ankiModel, apiKeyHeader, _ := extractAIOptions(args)

		if len(args) < 1 {
			fmt.Println("用法: add <word1> [word2 ...] [--provider=ollama|openai] [--api-key=key] [--api-base=url] [--api-key-header] [--batchcount=N] [--anki-deck=name] [--anki-model=name]")
			return
		}

		aiClient, err := NewAIClient(provider, model, apiKey, apiBase, apiKeyHeader)
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

		procLog := NewProcessLog(filepath.Join(".", "log.json"))
		procLog.Load()
		batchID := procLog.StartBatch("add", ankiDeck, ankiModel)

		ctx := &ProcessContext{
			Dict:      dict,
			Lemma:     lemma,
			AI:        aiClient,
			DictDir:   dictDir,
			DryRun:    false,
			AnkiDeck:  ankiDeck,
			AnkiModel: ankiModel,
			BatchID:   batchID,
			Log:       procLog,
		}

		for _, w := range args {
			ProcessWord(w, ctx)
		}

		if err := dict.Save(); err != nil {
			log.Fatal(err)
		}
		if err := lemma.Save(); err != nil {
			log.Fatal(err)
		}
		if err := procLog.Save(); err != nil {
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
		args, provider, model, apiKey, apiBase, ankiDeck, ankiModel, apiKeyHeader, _ := extractAIOptions(args)

		if len(args) < 1 {
			fmt.Println("用法: batch <file> [--provider=ollama|openai] [--api-key=key] [--api-base=url] [--api-key-header] [--batchcount=N] [--anki-deck=name] [--anki-model=name]")
			return
		}

		file := args[0]

		aiClient, err := NewAIClient(provider, model, apiKey, apiBase, apiKeyHeader)
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

		procLog := NewProcessLog(filepath.Join(".", "log.json"))
		procLog.Load()
		batchID := procLog.StartBatch("batch", ankiDeck, ankiModel)

		ctx := &ProcessContext{
			Dict:      dict,
			Lemma:     lemma,
			AI:        aiClient,
			DictDir:   dictDir,
			DryRun:    false,
			AnkiDeck:  ankiDeck,
			AnkiModel: ankiModel,
			BatchID:   batchID,
			Log:       procLog,
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
		if err := procLog.Save(); err != nil {
			log.Fatal(err)
		}

		fmt.Println("batch 完成")
		fmt.Printf("耗时: %.2fs\n", time.Since(start).Seconds())

	// ========================
	// 🔄 SYNC
	// ========================
	case "sync":
		dir := "."
		if len(os.Args) > 2 && !strings.HasPrefix(os.Args[2], "--") {
			dir = os.Args[2]
		}

		dictDir := filepath.Join(dir, "dict")
		dict := NewDictionary(filepath.Join(dir, "dictionary.json"))
		if err := dict.Load(); err != nil {
			log.Fatal(err)
		}

		lemma := NewLemmaStore(filepath.Join(dir, "lemma.json"))
		if err := lemma.Load(); err != nil {
			log.Fatal(err)
		}

		syncDict(dictDir, dict, lemma, false)

		if err := dict.Save(); err != nil {
			log.Fatal(err)
		}
		if err := lemma.Save(); err != nil {
			log.Fatal(err)
		}

		fmt.Println("sync 完成")

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


	// ========================
	// SYNC-ANKI
	// ========================
	case "sync-anki":
		start := time.Now()
		args := os.Args[2:]
		args, _, _, _, _, ankiDeck, ankiModel, _, _ := extractAIOptions(args)

		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		dict := NewDictionary(filepath.Join(dir, "dictionary.json"))
		if err := dict.Load(); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("[DEBUG] dict path: %s, words: %d\n", dict.Path, len(dict.Words))

		// 获取 Anki 中已有的单词
		existing, err := GetAnkiWords(ankiDeck)
		if err != nil {
			fmt.Println("[WARN] cannot query Anki, will try adding all:", err)
			existing = map[string]bool{}
		}
		fmt.Printf("Anki 中已有 %d 个词条\n", len(existing))

		dictDir := filepath.Join(dir, "dict")
		total := len(dict.Words)
		skipped := 0
		synced := 0
		failed := 0

		for word, entry := range dict.Words {
			if existing[word] {
				skipped++
				continue
			}
			full := loadWordFromFile(dictDir, word)
			var target *WordEntry
			if full != nil {
				target = full
			} else {
				tmp := entry
				target = &tmp
			}
			if err := AddToAnki(target, ankiDeck, ankiModel); err != nil {
				fmt.Printf("[FAIL] %s: %v\n", word, err)
				failed++
				continue
			}
			synced++
			fmt.Printf("[SYNC] %s\n", word)
		}

		fmt.Printf("sync-anki done: %d new, %d skipped, %d failed, total %d, %.2fs\n", synced, skipped, failed, total, time.Since(start).Seconds())

	// ========================
	// 📝 LOG
	// ========================
	case "log":
		dir := "."
		if len(os.Args) > 2 && !strings.HasPrefix(os.Args[2], "--") {
			dir = os.Args[2]
		}
		logPath := filepath.Join(dir, "log.json")
		procLog := NewProcessLog(logPath)
		if err := procLog.Load(); err != nil {
			log.Fatal(err)
		}
		procLog.ListBatches()

	// ========================
	// 🔄 RESYNC
	// ========================
	case "resync":
		if len(os.Args) < 3 {
			fmt.Println("用法: resync <batch-id> [dir] [--anki-deck=name] [--anki-model=name]")
			return
		}

		start := time.Now()
		batchID := os.Args[2]
		args := os.Args[3:]
		args, _, _, _, _, ankiDeck, ankiModel, _, _ := extractAIOptions(args)

		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		logPath := filepath.Join(dir, "log.json")
		procLog := NewProcessLog(logPath)
		if err := procLog.Load(); err != nil {
			log.Fatal(err)
		}

		batch := procLog.GetBatch(batchID)
		if batch == nil {
			fmt.Printf("batch %s not found\n", batchID)
			return
		}

		if ankiDeck == "" {
			ankiDeck = batch.AnkiDeck
		}
		if ankiModel == "" {
			ankiModel = batch.AnkiModel
		}

		dict := NewDictionary(filepath.Join(dir, "dictionary.json"))
		if err := dict.Load(); err != nil {
			log.Fatal(err)
		}

		// 获取 Anki 中已有的单词
		existing, err := GetAnkiWords(ankiDeck)
		if err != nil {
			fmt.Println("[WARN] cannot query Anki, will try adding all:", err)
			existing = map[string]bool{}
		}
		fmt.Printf("Anki 中已有 %d 个词条\n", len(existing))

		dictDir := filepath.Join(dir, "dict")
		synced := 0
		skipped := 0
		failed := 0

		for _, wl := range batch.Words {
			if wl.Status == "fail" {
				continue
			}
			if existing[wl.Word] {
				skipped++
				continue
			}
			full := loadWordFromFile(dictDir, wl.Word)
			var target *WordEntry
			if full != nil {
				target = full
			} else if entry, ok := dict.Words[wl.Word]; ok {
				tmp := entry
				target = &tmp
			} else {
				fmt.Printf("[SKIP] %s not in dict\n", wl.Word)
				continue
			}
			if err := AddToAnki(target, ankiDeck, ankiModel); err != nil {
				fmt.Printf("[FAIL] %s: %v\n", wl.Word, err)
				failed++
				continue
			}
			synced++
			fmt.Printf("[SYNC] %s\n", wl.Word)
		}

		fmt.Printf("resync done: %d new, %d skipped, %d failed, total %d, %.2fs\n", synced, skipped, failed, len(batch.Words), time.Since(start).Seconds())

		// ========================
		// ?? RETRY-ANKI
		// ========================
		case "retry-anki":
			start := time.Now()
			args := os.Args[2:]
			args, _, _, _, _, ankiDeck, ankiModel, _, _ := extractAIOptions(args)

			var batchID string
			dir := "."
			for _, a := range args {
				if strings.HasPrefix(a, "--") {
					continue
				}
				if batchID == "" {
					batchID = a
				} else if dir == "." {
					dir = a
				}
			}

			logPath := filepath.Join(dir, "log.json")
			procLog := NewProcessLog(logPath)
			if err := procLog.Load(); err != nil {
				log.Fatal(err)
			}

			if batchID == "" {
				if len(procLog.Batches) == 0 {
					fmt.Println("no batches found")
					return
				}
				batchID = procLog.Batches[len(procLog.Batches)-1].BatchID
			}

			batch := procLog.GetBatch(batchID)
			if batch == nil {
				fmt.Printf("batch %s not found\n", batchID)
				return
			}

			if ankiDeck == "" {
				ankiDeck = batch.AnkiDeck
			}
			if ankiModel == "" {
				ankiModel = batch.AnkiModel
			}

			dict := NewDictionary(filepath.Join(dir, "dictionary.json"))
			if err := dict.Load(); err != nil {
				log.Fatal(err)
			}

			existing, err := GetAnkiWords(ankiDeck)
			if err != nil {
				fmt.Println("[WARN] cannot query Anki, will try adding all:", err)
				existing = map[string]bool{}
			}
			fmt.Printf("Anki ??? %d ???\n", len(existing))

			dictDir := filepath.Join(dir, "dict")
			retried := 0
			skipped := 0
			synced := 0
			failed := 0

			for i := range batch.Words {
				wl := batch.Words[i]
				if wl.Status == "fail" || wl.Status == "new:anki-ok" {
					continue
				}
				if existing[wl.Word] {
					skipped++
					continue
				}
				full := loadWordFromFile(dictDir, wl.Word)
				var target *WordEntry
				if full != nil {
					target = full
				} else if entry, ok := dict.Words[wl.Word]; ok {
					tmp := entry
					target = &tmp
				} else {
					fmt.Printf("[SKIP] %s not in dict\n", wl.Word)
					continue
				}
				retried++
				if err := AddToAnki(target, ankiDeck, ankiModel); err != nil {
					fmt.Printf("[FAIL] %s: %v\n", wl.Word, err)
					batch.Words[i].Status = "new:anki-fail"
					failed++
					continue
				}
				batch.Words[i].Status = "new:anki-ok"
				synced++
				fmt.Printf("[SYNC] %s\n", wl.Word)
			}

			if err := procLog.Save(); err != nil {
				log.Fatal(err)
			}

			fmt.Printf("retry-anki done: %d new, %d skipped, %d failed, retried %d, total %d, %.2fs\n", synced, skipped, failed, retried, len(batch.Words), time.Since(start).Seconds())

		default:
			fmt.Println("未知命令:", os.Args[1])
	}
}

func extractAIOptions(args []string) ([]string, string, string, string, string, string, string, bool, int) {
	provider := getEnvDefault("AI_PROVIDER", defaultAIProvider)
	model := getEnvDefault("AI_MODEL", "")
	apiKey := getEnvDefault("OPENAI_API_KEY", "")
	apiBase := getEnvDefault("OPENAI_API_BASE", "")
	apiKeyHeader := false
	batchCount := 20
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
		if strings.HasPrefix(arg, "--batchcount=") {
		fmt.Sscanf(strings.TrimPrefix(arg, "--batchcount="), "%d", &batchCount)
		continue
	}
	if arg == "--api-key-header" {
		apiKeyHeader = true
		continue
	}
	if strings.HasPrefix(arg, "--api-base=") {
		apiBase = strings.TrimPrefix(arg, "--api-base=")
		continue
	}
	if arg == "--api-base" && i+1 < len(args) {
		apiBase = args[i+1]
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

	return filtered, provider, model, apiKey, apiBase, ankiDeck, ankiModel, apiKeyHeader, batchCount
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
