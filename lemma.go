package main

import (
	"encoding/json"
	"os"
)

type LemmaStore struct {
	Map  map[string]string `json:"map"`
	Path string
}

func NewLemmaStore(path string) *LemmaStore {
	return &LemmaStore{
		Map:  make(map[string]string),
		Path: path,
	}
}

func (l *LemmaStore) Load() error {
	data, err := os.ReadFile(l.Path)
	if err != nil {
		return nil
	}
	return json.Unmarshal(data, &l.Map)
}

func (l *LemmaStore) Save() error {
	data, _ := json.MarshalIndent(l.Map, "", "  ")
	return os.WriteFile(l.Path, data, 0644)
}

func (l *LemmaStore) Get(w string) (string, bool) {
	v, ok := l.Map[w]
	return v, ok
}

func (l *LemmaStore) Set(w, lemma string) {
	l.Map[w] = lemma
}
