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

func callAnkiConnect(action string, params interface{}) (interface{}, error) {
	req := AnkiRequest{Action: action, Version: 6, Params: params}
	data, _ := json.Marshal(req)
	resp, err := http.Post("http://localhost:8765", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result AnkiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Error != nil {
		return nil, fmt.Errorf("anki error: %v", result.Error)
	}
	return result.Result, nil
}

func GetAnkiWords(deckName string) (map[string]bool, error) {
	if deckName == "" {
		deckName = defaultAnkiDeck
	}
	ids, err := callAnkiConnect("findNotes", map[string]interface{}{"query": "deck:" + deckName + " tag:auto"})
	if err != nil {
		return nil, err
	}
	idList, ok := ids.([]interface{})
	if !ok || len(idList) == 0 {
		return map[string]bool{}, nil
	}
	intIDs := make([]int, 0, len(idList))
	for _, id := range idList {
		if f, ok := id.(float64); ok {
			intIDs = append(intIDs, int(f))
		}
	}
	info, err := callAnkiConnect("notesInfo", map[string]interface{}{"notes": intIDs})
	if err != nil {
		return nil, err
	}
	words := make(map[string]bool)
	infoList, ok := info.([]interface{})
	if !ok {
		return words, nil
	}
	for _, note := range infoList {
		noteMap, ok := note.(map[string]interface{})
		if !ok {
			continue
		}
		fields, ok := noteMap["fields"].(map[string]interface{})
		if !ok {
			continue
		}
		front, ok := fields["正面"].(map[string]interface{})
		if !ok {
			continue
		}
		val, ok := front["value"].(string)
		if !ok || val == "" {
			continue
		}
		word := strings.ToLower(strings.TrimSpace(strings.Split(val, "<br>")[0]))
		if word != "" {
			words[word] = true
		}
	}
	return words, nil
}
