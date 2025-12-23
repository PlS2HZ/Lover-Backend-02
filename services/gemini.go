package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
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

var (
	keyIndex int
	mu       sync.Mutex // ป้องกันปัญหาเมื่อมีการเรียกใช้พร้อมกัน
)

func AskGeminiRaw(prompt string) string {
	mu.Lock()
	// ✅ รวม Keys ทั้งหมดเข้าด้วยกัน
	keys := []string{
		os.Getenv("GEMINI_KEY_1"),
		os.Getenv("GEMINI_KEY_2"),
		os.Getenv("GEMINI_KEY_3"),
	}

	// ✅ เลือกรหัสคีย์และวนลูปสลับกัน (Round Robin)
	apiKey := keys[keyIndex]
	keyIndex = (keyIndex + 1) % len(keys)
	mu.Unlock()

	// ✅ ใช้รุ่น 2.5-flash-lite ตามที่นายต้องการเพื่อเอา 10 RPM
	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash-lite:generateContent?key=" + apiKey

	payload := map[string]interface{}{
		"contents": []interface{}{
			map[string]interface{}{
				"role": "user",
				"parts": []interface{}{
					map[string]interface{}{"text": prompt},
				},
			},
		},
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ API Error (Key %d) Status: %d, Body: %s\n", keyIndex, resp.StatusCode, string(body))
		return ""
	}

	var geminiResp GeminiResponse
	json.Unmarshal(body, &geminiResp)

	if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
		answer := strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text)
		fmt.Printf("✅ AI (Key %d) ตอบมาว่า: %s\n", keyIndex, answer)
		return answer
	}
	return ""
}

func GenerateDescription(word string) string {
	prompt := fmt.Sprintf(`คุณคือผู้ช่วยสร้างคำอธิบายในเกมทายคำ หน้าที่ของคุณคืออธิบาย '%s'
    โดยใช้รูปแบบเป๊ะๆ ดังนี้:
    "คำในใจคือ '%s' เป็นสิ่งของ ([ระบุประเภท]) ไม่สามารถกินได้/กินได้ [ระบุลักษณะ 3 อย่าง] ไม่ใช่สถานที่"`, word, word)
	return AskGeminiRaw(prompt)
}

// ใน services/gemini.go
func AskGemini(secretWord string, description string, question string) string {
	prompt := fmt.Sprintf(`คุณคือ AI อัจฉริยะที่สวมบทบาทเป็นชาวแก๊ง "รับทราบ" (Rubssarb) ในเกมทายคำ
    
    [ข้อมูลสำคัญ]
    - คำลับที่อยู่ในใจคือ: "%s"
    - บริบทเพิ่มเติม: %s
    
    [กฎการตอบ]
    1. ความถูกต้อง: คุณต้องตอบตามความจริงเสมอ (เช่น ถ้าเขาถามว่ากินได้ไหม แล้วคำลับคือขนม คุณต้องตอบว่า ใช่)
    2. รูปแบบประโยค: ต้องตอบในรูปแบบนี้เท่านั้น => คำถามที่ว่า "%s" คำตอบคือ ** [ประโยคคำตอบสไตล์รับทราบ] **
    3. ความหลากหลาย: ให้สุ่มใช้สำนวนกวนๆ เช่น "อื้มมมห์~~ ใช่แหละมั้ง", "อ่าาาา~~ คิดว่าไม่ใช่", "ไม่ใช่หรอก! มั้ง?!", "ถามจริง? เอาดีๆ.. ใช่!", "เลอะเทอะแล้ว.. ไม่ใช่!"
    4. กฎการชนะ (สำคัญมาก): หากผู้เล่นพิมพ์คำว่า "%s" หรือประโยคที่มีความหมายเดียวกันว่าคือสิ่งนั้น (เช่น เป็น%sใช่ไหม, ใช่ %s หรือเปล่า) ให้คุณตอบว่า "ถูกต้อง" เท่านั้น! ห้ามเล่นมุกในกรณีนี้
    
    คำถามผู้เล่น: "%s"`, secretWord, description, question, secretWord, secretWord, secretWord, question)

	return AskGeminiRaw(prompt)
}
