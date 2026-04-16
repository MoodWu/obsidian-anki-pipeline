package main

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

type WordEntry struct {
	Word        string   `json:"word"`
	Original    string   `json:"original"`
	Type        string   `json:"type"`
	Phonetic    string   `json:"phonetic"`
	Meaning     string   `json:"meaning"`
	Example     string   `json:"example"`
	Translation string   `json:"translation"`
	Cloze       string   `json:"cloze"`
	Note        string   `json:"note"`
	Aliases     []string `json:"aliases"`
	AddedAt     string   `json:"added_at"`
}

type Dictionary struct {
	Words map[string]WordEntry `json:"words"`
	Path  string
}

func NewDictionary(path string) *Dictionary {
	return &Dictionary{
		Words: make(map[string]WordEntry),
		Path:  path,
	}
}

func (d *Dictionary) Load() error {
	data, err := os.ReadFile(d.Path)
	if err != nil {
		return nil
	}
	return json.Unmarshal(data, &d.Words)
}

func (d *Dictionary) Save() error {
	data, _ := json.MarshalIndent(d.Words, "", "  ")
	return os.WriteFile(d.Path, data, 0644)
}

func (d *Dictionary) Add(entry WordEntry) {

	entry.Word = strings.ToLower(strings.TrimSpace(entry.Word))
	entry.AddedAt = time.Now().Format("2006-01-02")

	// ⭐ aliases 清洗
	aliasSet := make(map[string]struct{})

	for _, a := range entry.Aliases {
		a = strings.ToLower(strings.TrimSpace(a))
		if a != "" && a != entry.Word {
			aliasSet[a] = struct{}{}
		}
	}

	entry.Aliases = make([]string, 0, len(aliasSet))
	for k := range aliasSet {
		entry.Aliases = append(entry.Aliases, k)
	}

	d.Words[entry.Word] = entry
}

func (d *Dictionary) ExportMarkdown(file string) error {

	var b strings.Builder

	b.WriteString("# Vocabulary\n\n")

	for _, w := range d.Words {

		b.WriteString("## " + w.Word + "\n\n")

		if w.Type != "" {
			b.WriteString("- Type: " + w.Type + "\n")
		}

		if w.Phonetic != "" {
			b.WriteString("- Phonetic: " + w.Phonetic + "\n")
		}

		b.WriteString("- Meaning: " + w.Meaning + "\n\n")

		b.WriteString("**Example:**\n")
		b.WriteString(w.Example + "\n\n")

		b.WriteString("**Translation:**\n")
		b.WriteString(w.Translation + "\n\n")

		if w.Note != "" {
			b.WriteString("**Note:**\n")
			b.WriteString(w.Note + "\n\n")
		}

		b.WriteString("---\n\n")
	}

	return os.WriteFile(file, []byte(b.String()), 0644)
}
