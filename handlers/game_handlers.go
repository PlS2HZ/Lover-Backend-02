package handlers

import (
	"couple-app/models"
	"couple-app/services"
	"couple-app/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/supabase-community/postgrest-go"
	"github.com/supabase-community/supabase-go"
)

// HandleCreateHeartGame สร้างโจทย์ใหม่
func HandleCreateHeartGame(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var g models.HeartGame
	json.NewDecoder(r.Body).Decode(&g)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	row := map[string]interface{}{
		"host_id":     g.HostID,
		"guesser_id":  g.GuesserID,
		"secret_word": g.SecretWord,
		"use_bot":     g.UseBot,
		"status":      "waiting",
	}
	var results []map[string]interface{}
	client.From("heart_games").Insert(row, false, "", "", "").ExecuteTo(&results)
	go func() {
		msg := "มีคำทายรออยู่ในใจเค้า... ❤️"
		if g.UseBot {
			msg = "เค้าส่งบอท Gemini มาท้าทายเธอ! 🤖"
		}
		services.TriggerPushNotification(g.GuesserID, "🎮 Mind Game", msg)
	}()
	json.NewEncoder(w).Encode(results[0])
}

func HandleGenerateAIDescription(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var body struct {
		SecretWord string `json:"secret_word"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return
	}

	description := services.GenerateDescription(body.SecretWord)

	if description == "" {
		fmt.Println("⚠️ AI ส่งค่าว่างกลับมา กรุณาตรวจสอบ API Key หรือ Quota")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"description": description})
}

func HandleStartHeartGame(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	gameID := r.URL.Query().Get("id")

	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	now := time.Now()

	client.From("heart_games").Update(map[string]interface{}{
		"status":     "playing",
		"start_time": now,
	}, "", "").Eq("id", gameID).Execute()

	w.WriteHeader(http.StatusOK)
}

// ✅ HandleAskQuestion: แก้ไขจุด mismatch และเพิ่ม Log ตรวจสอบ
func HandleAskQuestion(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var msg struct {
		GameID   string `json:"game_id"` // นี่คือ Session ID จากหน้าแชท
		SenderID string `json:"sender_id"`
		Message  string `json:"message"`
	}
	json.NewDecoder(r.Body).Decode(&msg)

	fmt.Printf("📥 รับคำถามจาก SessionID: %s, ข้อความ: %s\n", msg.GameID, msg.Message)

	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	// ✅ 1. หาว่า Session นี้ผูกกับ Heart Game อันไหน เพื่อเอา ID ที่แท้จริงไปใช้
	var sessionData []map[string]interface{}
	client.From("game_sessions").Select("game_id", "", false).Eq("id", msg.GameID).ExecuteTo(&sessionData)

	if len(sessionData) > 0 {
		heartGameID := sessionData[0]["game_id"].(string)

		// ✅ 2. ดึงคำลับและคำอธิบายมาให้ AI ใช้ตัดสินใจ
		var gameData []map[string]interface{}
		client.From("heart_games").Select("*", "", false).Eq("id", heartGameID).ExecuteTo(&gameData)

		if len(gameData) > 0 {
			secretWord := gameData[0]["secret_word"].(string)
			description := ""
			if gameData[0]["description"] != nil {
				description = gameData[0]["description"].(string)
			}

			// เรียกใช้ AI (ระบบจะสลับคีย์ 1-3 ให้อัตโนมัติใน services)
			botAnswer := services.AskGemini(secretWord, description, msg.Message)

			// ✅ 3. ตรวจสอบการชนะเกม (ถ้ามีคำว่าถูกต้อง ให้จบเกมทันที)
			if strings.Contains(botAnswer, "ถูกต้อง") {
				client.From("heart_games").Update(map[string]interface{}{
					"status": "finished",
				}, "", "").Eq("id", heartGameID).Execute()
				botAnswer = "ถูกต้อง"
			}

			// ✅ 4. บันทึกข้อความลง Database (ใช้ 3 ตัวแปรเพื่อไม่ให้ mismatch)
			_, _, err := client.From("game_messages").Insert(map[string]interface{}{
				"game_id":   heartGameID, // บันทึกด้วย Heart Game ID เพื่อให้ความสัมพันธ์ข้อมูลถูกต้อง
				"sender_id": msg.SenderID,
				"message":   msg.Message,
				"answer":    botAnswer,
			}, false, "", "", "").Execute()

			if err != nil {
				fmt.Printf("❌ บันทึกไม่สำเร็จ: %v\n", err)
			} else {
				fmt.Printf("✅ AI ตอบและบันทึกสำเร็จ: %s\n", botAnswer)
			}
		}
	} else {
		fmt.Printf("❌ ไม่พบข้อมูล Session สำหรับ ID: %s\n", msg.GameID)
	}
	w.WriteHeader(http.StatusCreated)
}

func HandleGetLevels(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var levels []map[string]interface{}
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	client.From("heart_games").Select("*, users(username)", "", false).Gte("created_at", thirtyDaysAgo).Order("created_at", &postgrest.OrderOpts{Ascending: false}).ExecuteTo(&levels)
	json.NewEncoder(w).Encode(levels)
}

func HandleCreateGame(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var body struct {
		GameID    string `json:"game_id"`
		GuesserID string `json:"guesser_id"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var session []map[string]interface{}
	client.From("game_sessions").Insert(map[string]interface{}{
		"game_id": body.GameID, "guesser_id": body.GuesserID, "mode": "bot", "status": "playing",
	}, false, "", "", "").ExecuteTo(&session)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(session[0])
}
