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
	prompt := fmt.Sprintf(`คุณคือ "รับทราบ Bot" ในเกมทายคำ หน้าที่ของคุณคือตอบคำถามโดยห้ามเฉลยเด็ดขาด!
    
    [ข้อมูลอ้างอิง]
    - คำลับ: "%s"
    - ลักษณะ: %s
    
    [กฎเหล็ก ** ห้ามทำผิดเด็ดขาด **]
    1. **ห้ามพิมพ์คำว่า "%s"**: ไม่ว่าจะกรณีใดก็ตาม ห้ามหลุดคำว่า "%s" หรือคำที่ใกล้เคียง (เช่น ถ้าคำลับคือมอเตอร์ไซค์ ห้ามพูดว่า มอไซค์, รถเครื่อง, จักรยานยนต์) ออกมาในคำตอบเด็ดขาด!
    2. **การเลี่ยงคำ**: ให้ใช้คำว่า "มัน", "สิ่งนี้", "เจ้านี่" แทนการเรียกชื่อคำลับ
    3. **ความถูกต้อง**: ต้องตอบตามจริง (เช่น ถ้าปกติมี 4 ขา แล้วเขาถามว่ามี 2 ขาไหม ให้บอกว่า "ไม่ใช่" หรือ "ไม่ใช่ว่ะ ปกติมันมีเยอะกว่านั้น")
    4. **บุคลิก**: กวนประสาทสไตล์รับทราบเหมือนเดิม แต่ห้ามโป๊ะแตกเฉลยเอง
    5. **การชนะ**: ถ้าผู้เล่นทายว่า "%s" ให้ตอบคำเดียวว่า "ถูกต้อง"
    
    คำถามผู้เล่น: "%s"`, secretWord, description, secretWord, secretWord, secretWord, question)

	return AskGeminiRaw(prompt)
}
