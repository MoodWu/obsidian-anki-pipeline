package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

type AnkiRequest struct {
	Action  string      `json:"action"`
	Version int         `json:"version"`
	Params  interface{} `json:"params"`
}

type AnkiResponse struct {
	Result interface{} `json:"result"`
	Error  interface{} `json:"error"`
}

func replaceCarry(v string) string {
	return strings.ReplaceAll(v, "\n", "<br>")
}

func replaceClozeVariables(cloze string) string {
	re := regexp.MustCompile(`(?i)\{\{c\d+::[^}]+\}\}`)
	return re.ReplaceAllString(cloze, "____")
}

func AddToAnki(entry *WordEntry, deckName, modelName string) error {

	// fmt.Printf("[AddToAnki] deckName: %s, modelName: %s, word: %s\n", deckName, modelName, entry.Word)

	if deckName == "" {
		deckName = defaultAnkiDeck
	}
	if modelName == "" {
		modelName = defaultAnkiModel
	}

	// fmt.Println("AddToAnki")
	req := AnkiRequest{
		Action:  "addNote",
		Version: 6,
		Params: map[string]interface{}{
			"note": map[string]interface{}{
				"deckName":  deckName,
				"modelName": modelName,
				"fields": map[string]string{
					"正面": replaceCarry(entry.Word + "\n" + entry.Phonetic),
					"背面": replaceCarry(entry.Meaning + "<hr><h3>笔记</h3>" + entry.Note + "<hr><h3>例句</h3>" + entry.Example + "<hr><h3>翻译</h3>" + entry.Translation + "<hr><h3>填空</h3>" + replaceClozeVariables(entry.Cloze)),
				},
				"options": map[string]interface{}{
					"allowDuplicate": false,
				},
				"tags": []string{"auto", "ai"},
			},
		},
	}

	data, _ := json.Marshal(req)

	// fmt.Println(string(data))

	resp, err := http.Post("http://localhost:8765", "application/json", bytes.NewBuffer(data))
	if err != nil {
		fmt.Println("[AddToAnki] HTTP error:", err)
		return err
	}
	defer resp.Body.Close()

	var result AnkiResponse
	json.NewDecoder(resp.Body).Decode(&result)

	// fmt.Printf("[AddToAnki] response: %+v\n", result)
	if result.Error != nil {
		return fmt.Errorf("anki error: %v", result.Error)
	}

	return nil
}
