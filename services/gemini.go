package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func AskGemini(secretWord string, description string, question string) string {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return "API Key missing"
	}

	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=" + apiKey

	// สร้าง Prompt ที่มี Context เพื่อให้ AI ฉลาดขึ้น
	prompt := fmt.Sprintf(`คุณคือผู้ช่วยในเกมทายใจ หน้าที่ของคุณคือตอบคำถามของผู้เล่น
    คำลับที่ผู้เล่นต้องทายคือ: "%s"
    คำอธิบายเพิ่มเติมเกี่ยวกับคำลับนี้: "%s"
    
    กฎการตอบ:
    1. ตอบได้เพียง 3 คำเท่านั้นคือ "ใช่", "ไม่ใช่", หรือ "ถูกต้อง"
    2. ถ้าผู้เล่นทายคำได้ตรงกับคำลับเป๊ะๆ ให้ตอบว่า "ถูกต้อง"
    3. ถ้าคำถามมีความเกี่ยวข้องหรือเป็นคุณลักษณะของคำลับ ให้ตอบว่า "ใช่" (ยืดหยุ่นตามบริบท อย่าซื่อตรงเกินไป)
    4. ถ้าไม่เกี่ยวข้องเลย ให้ตอบว่า "ไม่ใช่"
    
    คำถามจากผู้เล่น: "%s"`, secretWord, description, question)

	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
				},
			},
		},
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "ไม่ใช่"
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var geminiResp GeminiResponse
	json.Unmarshal(body, &geminiResp)

	if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
		answer := strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text)
		return answer
	}

	return "ไม่ใช่"
}
