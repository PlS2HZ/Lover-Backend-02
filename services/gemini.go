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

	// 3. เทรน AI ด้วยบทบาทที่เข้มงวด (System Prompt)
	prompt := fmt.Sprintf(`คำสั่ง: คุณคือกรรมการตัดสินเกมทายคำ หน้าที่ของคุณคือเปรียบเทียบคำถามกับคำลับที่กำหนดให้
    
    ข้อมูลคำลับ: "%s"
    คำถามหรือคำทายจากผู้เล่น: "%s"

    กฎการตอบ (ห้ามตอบนอกเหนือจากนี้):
    1. ถ้าผู้เล่นทายชื่อคำลับได้ถูกต้อง (เช่น คำลับคือ โต๊ะ และผู้เล่นพิมพ์ว่า โต๊ะ หรือ คือโต๊ะใช่ไหม) ให้ตอบว่า "ถูกต้อง"
    2. ถ้าคำถามของผู้เล่นสอดคล้องกับคุณสมบัติของคำลับ ให้ตอบว่า "ใช่"
    3. ถ้าคำถามของผู้เล่นไม่สอดคล้องกับคุณสมบัติของคำลับ ให้ตอบว่า "ไม่ใช่"
    4. ห้ามพิมพ์คำอธิบายประกอบ ห้ามมีจุดฟูลสต็อป ตอบเพียงคำเดียวเท่านั้น`, cleanSecret, cleanQuestion)

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
