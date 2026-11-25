package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type geminiPart struct {
	Text       string `json:"text,omitempty"`
	InlineData *struct {
		MimeType string `json:"mime_type"`
		Data     string `json:"data"`
	} `json:"inline_data,omitempty"`
}

type geminiRequest struct {
	SystemInstruction *struct {
		Parts []geminiPart `json:"parts"`
	} `json:"systemInstruction,omitempty"`
	Contents []struct {
		Role  string       `json:"role"`
		Parts []geminiPart `json:"parts"`
	} `json:"contents"`
}

func portalStudentAI(w http.ResponseWriter, r *http.Request, userID int64) {
	if r.Method == http.MethodGet {
		rows, err := db.Query(`SELECT role,content,created_at FROM ai_chat_messages WHERE user_id=$1 ORDER BY created_at DESC LIMIT 30`, userID)
		if err != nil {
			writeAdminError(w, 500, "Не удалось загрузить историю помощника")
			return
		}
		defer rows.Close()
		type chatItem struct {
			Role      string    `json:"role"`
			Content   string    `json:"content"`
			CreatedAt time.Time `json:"created_at"`
		}
		items := []chatItem{}
		for rows.Next() {
			var x chatItem
			if err := rows.Scan(&x.Role, &x.Content, &x.CreatedAt); err != nil {
				writeAdminError(w, 500, "Ошибка чтения истории")
				return
			}
			items = append(items, x)
		}
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
		writeAdminJSON(w, 200, items)
		return
	}
	if r.Method != http.MethodPost {
		writeAdminError(w, 405, "Метод не поддерживается")
		return
	}
	var req struct {
		Message    string `json:"message"`
		MaterialID int64  `json:"material_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeAdminError(w, 400, "Некорректный запрос помощнику")
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" || len(req.Message) > 12000 {
		writeAdminError(w, 400, "Напишите вопрос (до 12000 символов)")
		return
	}
	apiKey := strings.TrimSpace(os.Getenv("AI_API_KEY"))
	model := strings.TrimSpace(os.Getenv("AI_MODEL"))
	if apiKey == "" || model == "" {
		writeAdminError(w, http.StatusServiceUnavailable, "AI-помощник не настроен: администратору нужно задать AI_API_KEY и AI_MODEL")
		return
	}
	systemPrompt := "Ты — AI-помощник университетского портала YOUR DAY. Отвечай на русском языке, объясняй понятно и по шагам, не выдумывай факты из учебных материалов. Если информации недостаточно, прямо скажи об этом. Помогай учиться, а не выдавай себя за преподавателя."
	parts := []geminiPart{{Text: req.Message}}
	if req.MaterialID > 0 {
		var filePath, fileName string
		err := db.QueryRow(`SELECT cm.file_path,cm.file_name FROM course_materials cm JOIN teacher_assignments a ON a.id=cm.assignment_id JOIN students st ON st.group_id=a.group_id WHERE cm.id=$1 AND st.user_id=$2`, req.MaterialID, userID).Scan(&filePath, &fileName)
		if err != nil {
			writeAdminError(w, 404, "Материал не найден или недоступен")
			return
		}
		name := filepath.Base(strings.TrimPrefix(filePath, "/uploads/"))
		if name == "." || name == "" || name != strings.TrimPrefix(filePath, "/uploads/") {
			writeAdminError(w, 400, "Некорректный путь материала")
			return
		}
		data, err := os.ReadFile(filepath.Join("uploads", name))
		if err != nil {
			writeAdminError(w, 404, "Файл материала не найден на сервере")
			return
		}
		if len(data) > 12<<20 {
			writeAdminError(w, 413, "Материал слишком большой для анализа (максимум 12 МБ)")
			return
		}
		ext := strings.ToLower(filepath.Ext(fileName))
		switch ext {
		case ".pdf":
			p := geminiPart{}
			p.InlineData = &struct {
				MimeType string `json:"mime_type"`
				Data     string `json:"data"`
			}{MimeType: "application/pdf", Data: base64.StdEncoding.EncodeToString(data)}
			parts = append(parts, geminiPart{Text: "Проанализируй прикреплённый учебный PDF-файл. Сначала кратко определи тему и структуру, затем объясни ключевые понятия, выдели важное для экзамена и ответь на вопрос студента."}, p)
		case ".txt":
			parts = append(parts, geminiPart{Text: "Учебный материал преподавателя (" + fileName + "):\n" + string(data)})
		case ".docx":
			text, err := extractOfficeText(data, "word/document.xml")
			if err != nil {
				writeAdminError(w, 400, "Не удалось прочитать DOCX-файл")
				return
			}
			parts = append(parts, geminiPart{Text: "Учебный материал преподавателя (" + fileName + "):\n" + text})
		default:
			writeAdminError(w, http.StatusUnsupportedMediaType, "AI-анализ пока поддерживает PDF, DOCX и TXT. Для других форматов попросите преподавателя загрузить PDF-версию.")
			return
		}
	}
	payload := geminiRequest{}
	payload.SystemInstruction = &struct {
		Parts []geminiPart `json:"parts"`
	}{Parts: []geminiPart{{Text: systemPrompt}}}
	historyRows, historyErr := db.Query(`SELECT role,content FROM ai_chat_messages WHERE user_id=$1 ORDER BY created_at DESC LIMIT 10`, userID)
	if historyErr == nil {
		type historyMessage struct {
			Role    string
			Content string
		}
		history := []historyMessage{}
		for historyRows.Next() {
			var item historyMessage
			if historyRows.Scan(&item.Role, &item.Content) == nil {
				history = append(history, item)
			}
		}
		historyRows.Close()
		for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
			history[i], history[j] = history[j], history[i]
		}
		for _, item := range history {
			role := "user"
			if item.Role == "assistant" {
				role = "model"
			}
			payload.Contents = append(payload.Contents, struct {
				Role  string       `json:"role"`
				Parts []geminiPart `json:"parts"`
			}{Role: role, Parts: []geminiPart{{Text: item.Content}}})
		}
	}
	payload.Contents = append(payload.Contents, struct {
		Role  string       `json:"role"`
		Parts []geminiPart `json:"parts"`
	}{Role: "user", Parts: parts})
	body, err := json.Marshal(payload)
	if err != nil {
		writeAdminError(w, 500, "Не удалось подготовить запрос")
		return
	}
	endpoint := strings.TrimSpace(os.Getenv("AI_API_URL"))
	if endpoint == "" {
		endpoint = "https://generativelanguage.googleapis.com/v1beta/models/" + model + ":generateContent"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	aiReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		writeAdminError(w, 500, "Некорректный адрес AI-сервиса")
		return
	}
	aiReq.Header.Set("Content-Type", "application/json")
	aiReq.Header.Set("x-goog-api-key", apiKey)
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(aiReq)
	if err != nil {
		writeAdminError(w, 502, "AI-сервис временно недоступен")
		return
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		writeAdminError(w, 502, "Не удалось получить ответ AI-сервиса")
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeAdminError(w, 502, "AI-сервис отклонил запрос. Проверьте ключ и модель AI_MODEL")
		return
	}
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err = json.Unmarshal(respBody, &result); err != nil || len(result.Candidates) == 0 {
		writeAdminError(w, 502, "AI-сервис вернул ответ в неизвестном формате")
		return
	}
	answerParts := []string{}
	for _, p := range result.Candidates[0].Content.Parts {
		if strings.TrimSpace(p.Text) != "" {
			answerParts = append(answerParts, p.Text)
		}
	}
	answer := strings.TrimSpace(strings.Join(answerParts, "\n"))
	if answer == "" {
		writeAdminError(w, 502, "AI-сервис вернул пустой ответ")
		return
	}
	_, _ = db.Exec(`INSERT INTO ai_chat_messages(user_id,role,content) VALUES($1,'user',$2),($1,'assistant',$3)`, userID, req.Message, answer)
	writeAdminJSON(w, 200, map[string]string{"answer": answer})
}

func extractOfficeText(data []byte, target string) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	var file io.ReadCloser
	for _, f := range zr.File {
		if f.Name == target {
			file, err = f.Open()
			if err != nil {
				return "", err
			}
			break
		}
	}
	if file == nil {
		return "", fmt.Errorf("document XML not found")
	}
	defer file.Close()
	dec := xml.NewDecoder(io.LimitReader(file, 8<<20))
	var b strings.Builder
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if text, ok := tok.(xml.CharData); ok {
			v := strings.TrimSpace(string(text))
			if v != "" {
				if b.Len() > 0 {
					b.WriteByte(' ')
				}
				b.WriteString(v)
			}
		}
	}
	return b.String(), nil
}
