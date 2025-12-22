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

// โครงสร้างสำหรับแกะ JSON จาก Gemini API
type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// ฟังก์ชันหลักที่ใช้ถาม Gemini
func AskGemini(secretWord string, question string) string {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return "API Key missing"
	}

	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=" + apiKey

	// สร้าง Prompt ที่เข้มงวดเพื่อให้บอทตอบแค่ 3 คำที่กำหนด
	prompt := fmt.Sprintf(`คุณคือบอทในเกมทายคำในใจ 
    คำลับที่ถูกต้องคือ: "%s" 
    ผู้เล่นถามหรือทายว่า: "%s"
    
    กฎการตอบ:
    1. ถ้าผู้เล่นทายได้ตรงกับคำลับ หรือมีความหมายเดียวกันเป๊ะ ให้ตอบว่า "ถูกต้อง" เท่านั้น
    2. ถ้าเป็นคำถามทั่วไป ให้ตอบว่า "ใช่" หรือ "ไม่ใช่" ตามความเป็นจริงของคำลับนั้น
    3. ห้ามตอบอย่างอื่นนอกเหนือจาก "ใช่", "ไม่ใช่", หรือ "ถูกต้อง"`, secretWord, question)

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
		fmt.Println("Gemini Error:", err)
		return "ผิดพลาด"
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		fmt.Println("JSON Unmarshal Error:", err)
		return "ผิดพลาด"
	}

	// แกะข้อความที่ AI ตอบกลับมา
	if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
		aiResult := strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text)

		// กรองเอาเฉพาะคำที่เราต้องการ เผื่อ AI ตอบแถม
		if strings.Contains(aiResult, "ถูกต้อง") {
			return "ถูกต้อง"
		} else if strings.Contains(aiResult, "ใช่") && !strings.Contains(aiResult, "ไม่ใช่") {
			return "ใช่"
		} else {
			return "ไม่ใช่"
		}
	}

	return "ไม่ใช่"
}
