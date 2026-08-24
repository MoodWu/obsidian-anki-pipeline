package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type PendingWord struct {
	Word       string `json:"word"`
	Errors     int    `json:"errors"`
	LastError  string `json:"last_error,omitempty"`
	LastTried  string `json:"last_tried,omitempty"`
}

type PendingQueue struct {
	Path  string         `json:"-"`
	Words []PendingWord  `json:"words"`
}

func NewPendingQueue(path string) *PendingQueue {
	return &PendingQueue{Path: path, Words: make([]PendingWord, 0)}
}

func (pq *PendingQueue) Load() error {
	data, err := os.ReadFile(pq.Path)
	if err != nil {
		if os.IsNotExist(err) {
			pq.Words = make([]PendingWord, 0)
			return nil
		}
		return err
	}
	return json.Unmarshal(data, pq)
}

func (pq *PendingQueue) Save() error {
	data, err := json.MarshalIndent(pq, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pq.Path, data, 0644)
}

func (pq *PendingQueue) Add(word string) {
	for _, w := range pq.Words {
		if w.Word == word {
			return
		}
	}
	pq.Words = append(pq.Words, PendingWord{Word: word})
}

func (pq *PendingQueue) AddBatch(words []string) {
	for _, w := range words {
		pq.Add(w)
	}
}

func (pq *PendingQueue) GetBatch(batchCount int) []PendingWord {
	batch := make([]PendingWord, 0, batchCount)
	for _, w := range pq.Words {
		if w.Errors < 3 {
			batch = append(batch, w)
			if len(batch) >= batchCount {
				break
			}
		}
	}
	return batch
}

func (pq *PendingQueue) HasProcessable() bool {
	for _, w := range pq.Words {
		if w.Errors < 3 {
			return true
		}
	}
	return false
}

func (pq *PendingQueue) RecordSuccess(word string) {
	filtered := make([]PendingWord, 0, len(pq.Words))
	for _, w := range pq.Words {
		if w.Word != word {
			filtered = append(filtered, w)
		}
	}
	pq.Words = filtered
}

func (pq *PendingQueue) RecordFailure(word string, errMsg string) {
	for i, w := range pq.Words {
		if w.Word == word {
			pq.Words[i].Errors++
			pq.Words[i].LastError = errMsg
			pq.Words[i].LastTried = time.Now().Format("2006-01-02 15:04:05")
			break
		}
	}
}

func (pq *PendingQueue) Stats() (processable int, failed int) {
	for _, w := range pq.Words {
		if w.Errors >= 3 {
			failed++
		} else {
			processable++
		}
	}
	return
}

func (pq *PendingQueue) PrintStatus() {
	processable, failed := pq.Stats()
	fmt.Printf("Pending: %d total, %d processable, %d failed (3+ errors)\n",
		len(pq.Words), processable, failed)
}

func (pq *PendingQueue) RemoveFailed() []PendingWord {
	failed := make([]PendingWord, 0)
	remaining := make([]PendingWord, 0)
	for _, w := range pq.Words {
		if w.Errors >= 3 {
			failed = append(failed, w)
		} else {
			remaining = append(remaining, w)
		}
	}
	pq.Words = remaining
	return failed
}
