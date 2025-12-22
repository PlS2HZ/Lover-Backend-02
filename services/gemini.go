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
		return "API Key missing" //
	}

	// 1. ล้างข้อมูลเบื้องต้นเพื่อความแม่นยำ
	cleanQuestion := strings.TrimSpace(question)
	cleanSecret := strings.TrimSpace(secretWord)

	// 2. ระบบดักคำตอบ (Hard Check): ถ้าพิมพ์ตรงกับคำลับเป๊ะๆ ให้ตอบ "ถูกต้อง" ทันทีโดยไม่ผ่าน AI
	if strings.EqualFold(cleanQuestion, cleanSecret) {
		return "ถูกต้อง"
	}

	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=" + apiKey

	// ปรับ Prompt ใน services/gemini.go ให้ AI "ช่วยใบ้" มากขึ้น
	prompt := fmt.Sprintf(`คุณคือกรรมการในเกมทายคำ หน้าที่ของคุณคือช่วยให้ผู้เล่นทายถูก
คำลับคือ: "%s" 
ผู้เล่นถามว่า: "%s"

กฎการตัดสิน:
1. หากคำถามมีความเกี่ยวข้องหรือเป็นคุณสมบัติที่ "ส่วนใหญ่ยอมรับ" ของคำลับ ให้ตอบว่า "ใช่" 
   (เช่น ถ้าคำลับคือ นม และถามว่า ทำจากวัว หรือ ขายในกล่อง ให้ตอบว่า "ใช่")
2. หากผู้เล่นทายชื่อคำลับได้ถูกต้องเป๊ะ ให้ตอบว่า "ถูกต้อง"
3. หากไม่เกี่ยวข้องเลยจริงๆ ให้ตอบว่า "ไม่ใช่"
ตอบเพียงคำเดียว: "ใช่", "ไม่ใช่", หรือ "ถูกต้อง" เท่านั้น`, cleanSecret, cleanQuestion)

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
		return "ผิดพลาด" //
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var geminiResp GeminiResponse
	json.Unmarshal(body, &geminiResp)

	if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
		aiResult := strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text)

		// 4. ระบบคัดกรองคำตอบสุดท้าย (Logic Check)
		if strings.Contains(aiResult, "ถูกต้อง") {
			return "ถูกต้อง"
		}
		if strings.Contains(aiResult, "ไม่ใช่") {
			return "ไม่ใช่"
		}
		if strings.Contains(aiResult, "ใช่") {
			return "ใช่"
		}
	}

	return "ไม่ใช่" // Default กรณี AI ตอบแปลกๆ
}
