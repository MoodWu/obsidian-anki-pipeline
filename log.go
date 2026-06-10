package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type WordLog struct {
	Raw   string `json:"raw"`
	Word  string `json:"word"`
	Status string `json:"status"` // new, skip, fail
}

type BatchLog struct {
	BatchID   string    `json:"batch_id"`
	Command   string    `json:"command"`
	AnkiDeck  string    `json:"anki_deck"`
	AnkiModel string    `json:"anki_model"`
	Words     []WordLog `json:"words"`
	CreatedAt string    `json:"created_at"`
}

type ProcessLog struct {
	Batches []BatchLog `json:"batches"`
	Path    string
}

func NewProcessLog(path string) *ProcessLog {
	return &ProcessLog{
		Batches: make([]BatchLog, 0),
		Path:    path,
	}
}

func (pl *ProcessLog) Load() error {
	data, err := os.ReadFile(pl.Path)
	if err != nil {
		return nil
	}
	return json.Unmarshal(data, &pl.Batches)
}

func (pl *ProcessLog) Save() error {
	data, err := json.MarshalIndent(pl.Batches, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pl.Path, data, 0644)
}

func (pl *ProcessLog) StartBatch(command, ankiDeck, ankiModel string) string {
	batchID := time.Now().Format("20060102-150405")
	pl.Batches = append(pl.Batches, BatchLog{
		BatchID:   batchID,
		Command:   command,
		AnkiDeck:  ankiDeck,
		AnkiModel: ankiModel,
		Words:     make([]WordLog, 0),
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
	})
	return batchID
}

func (pl *ProcessLog) LogWord(batchID, raw, word, status string) {
	for i := range pl.Batches {
		if pl.Batches[i].BatchID == batchID {
			pl.Batches[i].Words = append(pl.Batches[i].Words, WordLog{
				Raw:    raw,
				Word:   word,
				Status: status,
			})
			return
		}
	}
}

func (pl *ProcessLog) GetBatch(batchID string) *BatchLog {
	for i := range pl.Batches {
		if pl.Batches[i].BatchID == batchID {
			return &pl.Batches[i]
		}
	}
	return nil
}

func (pl *ProcessLog) ListBatches() {
	if len(pl.Batches) == 0 {
		fmt.Println("no batches found")
		return
	}
	for _, b := range pl.Batches {
		newCount := 0
		skipCount := 0
		failCount := 0
		for _, w := range b.Words {
			switch w.Status {
			case "new":
				newCount++
			case "skip":
				skipCount++
			case "fail":
				failCount++
			}
		}
		fmt.Printf("[%s] %s %s | %d words: %d new, %d skip, %d fail\n",
			b.BatchID, b.Command, b.CreatedAt, len(b.Words), newCount, skipCount, failCount)
	}
}