package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"github.com/supabase-community/supabase-go"
	"golang.org/x/crypto/bcrypt"
)

var jwtKey = []byte("your_secret_key_2025")

type User struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type RequestBody struct {
	ID            string `json:"id,omitempty"`
	Header        string `json:"header"`
	Title         string `json:"title"`
	Duration      string `json:"duration"`
	SenderID      string `json:"sender_id"`
	ReceiverEmail string `json:"receiver_email"`
	TimeStart     string `json:"time_start"`
	TimeEnd       string `json:"time_end"`
}

type Event struct {
	ID          string   `json:"id,omitempty"`
	EventDate   string   `json:"event_date"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	CreatedBy   string   `json:"created_by"`
	VisibleTo   []string `json:"visible_to"`
	RepeatType  string   `json:"repeat_type"`
}

func enableCORS(w *http.ResponseWriter, r *http.Request) bool {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PATCH, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
	if r.Method == "OPTIONS" {
		(*w).WriteHeader(http.StatusOK)
		return true
	}
	return false
}

func sendDiscord(content string) {
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if webhookURL == "" {
		return
	}
	payload := map[string]string{"content": content}
	jsonData, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
}

// ✨ ปรับปรุงให้แสดงเวลาไทย + วินาทีให้ตรงตามหน้าเว็บ
func formatDisplayTime(t string) string {
	parsedTime, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return t
	}
	// บวก 7 ชั่วโมงให้เป็นเวลาไทย
	thailandTime := parsedTime.Add(7 * time.Hour)
	return thailandTime.Format("2006-01-02 TIME 15:04:05")
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	var creds struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&creds)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var users []map[string]interface{}
	filter := fmt.Sprintf("email.eq.%s,username.eq.%s", creds.Identifier, creds.Identifier)
	client.From("users").Select("*", "exact", false).Or(filter, "").ExecuteTo(&users)
	if len(users) == 0 {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(users[0]["password"].(string)), []byte(creds.Password)); err != nil {
		http.Error(w, "Wrong password", http.StatusUnauthorized)
		return
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": users[0]["id"], "username": users[0]["username"], "exp": time.Now().Add(time.Hour * 72).Unix()})
	tokenString, _ := token.SignedString(jwtKey)
	json.NewEncoder(w).Encode(map[string]string{"token": tokenString, "username": users[0]["username"].(string), "user_id": users[0]["id"].(string), "email": users[0]["email"].(string)})
}

func handleGetAllUsers(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var users []map[string]interface{}
	client.From("users").Select("id, email, username", "exact", false).ExecuteTo(&users)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	var req RequestBody
	json.NewDecoder(r.Body).Decode(&req)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var sender []map[string]interface{}
	client.From("users").Select("username", "exact", false).Eq("id", req.SenderID).ExecuteTo(&sender)
	sName := "Unknown"
	if len(sender) > 0 {
		sName = sender[0]["username"].(string)
	}
	var receiver []map[string]interface{}
	client.From("users").Select("id, username", "exact", false).Eq("email", req.ReceiverEmail).ExecuteTo(&receiver)
	if len(receiver) == 0 {
		http.Error(w, "Receiver Not Found", http.StatusNotFound)
		return
	}
	rName := receiver[0]["username"].(string)
	row := map[string]interface{}{"category": req.Header, "title": req.Title, "description": req.Duration, "sender_id": req.SenderID, "receiver_id": receiver[0]["id"].(string), "status": "pending", "sender_name": sName, "receiver_name": rName, "remark": fmt.Sprintf("%s|%s", req.TimeStart, req.TimeEnd)}
	client.From("requests").Insert(row, false, "", "", "").Execute()
	go func() {
		msg := fmt.Sprintf("--------------------------------------------------\n@everyone มีคำขอใหม่ส่งถึงคุณ!\nหัวข้อ: %s\nจาก: %s\nถึง: %s\nรายละเอียด: %s\nเริ่ม: %s\nจบ: %s\nLink: https://lover-frontend-ashen.vercel.app/", req.Header, sName, rName, req.Title, formatDisplayTime(req.TimeStart), formatDisplayTime(req.TimeEnd))
		sendDiscord(msg)
	}()
	w.WriteHeader(http.StatusCreated)
}

func handleGetMyRequests(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	uID := r.URL.Query().Get("user_id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var data []map[string]interface{}
	client.From("requests").Select("*", "exact", false).Or(fmt.Sprintf("sender_id.eq.%s,receiver_id.eq.%s", uID, uID), "").ExecuteTo(&data)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	var body struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Comment string `json:"comment"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	updateData := map[string]interface{}{"status": body.Status, "processed_at": time.Now().Format(time.RFC3339), "comment": body.Comment}
	client.From("requests").Update(updateData, "", "").Eq("id", body.ID).Execute()
	go func() {
		var results []map[string]interface{}
		client.From("requests").Select("*", "exact", false).Eq("id", body.ID).ExecuteTo(&results)
		if len(results) > 0 {
			item := results[0]
			statusText := "อนุมัติ"
			reasonLine := ""
			if body.Status == "rejected" {
				statusText = "ไม่อนุมัติ"
				reasonLine = fmt.Sprintf("\nเหตุผลไม่อนุมัติ: %s", body.Comment)
			}
			msg := fmt.Sprintf("--------------------------------------------------\n@everyone\nผลการพิจารณาคำขอ!สถานะ: %s\nจาก: %v\nถึง: %v\nหัวข้อ: %v\nรายละเอียด: %v%s\nLink: https://lover-frontend-ashen.vercel.app/", statusText, item["sender_name"], item["receiver_name"], item["category"], item["title"], reasonLine)
			sendDiscord(msg)
		}
	}()
	w.WriteHeader(http.StatusOK)
}

func handleCreateEvent(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	var ev Event
	json.NewDecoder(r.Body).Decode(&ev)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	row := map[string]interface{}{"event_date": ev.EventDate, "title": ev.Title, "description": ev.Description, "repeat_type": ev.RepeatType}
	if ev.CreatedBy != "" {
		row["created_by"] = ev.CreatedBy
	}
	if len(ev.VisibleTo) > 0 {
		row["visible_to"] = ev.VisibleTo
	} else {
		row["visible_to"] = []string{}
	}
	client.From("events").Insert(row, false, "", "", "").Execute()
	go func() {
		creator := "ใครบางคน"
		var sender []map[string]interface{}
		client.From("users").Select("username", "exact", false).Eq("id", ev.CreatedBy).ExecuteTo(&sender)
		if len(sender) > 0 {
			creator = sender[0]["username"].(string)
		}
		msg := fmt.Sprintf("--------------------------------------------------\n✨ **บันทึกวันสำคัญใหม่!**\nหัวข้อ: %s\nวันที่: %s\nคนบันทึก: %s\nวนซ้ำ: %s\nLink: https://lover-frontend-ashen.vercel.app/", ev.Title, formatDisplayTime(ev.EventDate), creator, ev.RepeatType)
		sendDiscord(msg)
	}()
	w.WriteHeader(http.StatusCreated)
}

func handleGetMyEvents(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	uID := r.URL.Query().Get("user_id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var data []map[string]interface{}
	filter := fmt.Sprintf("created_by.eq.%s,visible_to.cs.{%s}", uID, uID)
	client.From("events").Select("*", "exact", false).Or(filter, "").ExecuteTo(&data)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func handleDeleteEvent(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	id, title := r.URL.Query().Get("id"), r.URL.Query().Get("title")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("events").Delete("", "").Eq("id", id).Execute()
	go sendDiscord(fmt.Sprintf("--------------------------------------------------\n🗑️ **ลบวันพิเศษแล้ว**\nหัวข้อ: %s\nสถานะ: รายการถูกนำออกแล้ว\n--------------------------------------------------", title))
	w.WriteHeader(http.StatusOK)
}

// ✨ ระบบแจ้งเตือนอัตโนมัติแบบแม่นยำวินาที
func handleCronRemind(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	// 🕒 ดึงเวลาปัจจุบัน UTC และปัดให้เหลือแค่นาที (วินาทีจะเป็น 00)
	now := time.Now().UTC().Truncate(time.Minute)
	targetTime := now.Format("2006-01-02T15:04:00.000Z")

	fmt.Printf("🎯 Cron Calling: %s\n", targetTime)

	var results []map[string]interface{}
	// ค้นหาแบบเป๊ะๆ ที่วินาที 00 ของนาทีนี้
	_, err := client.From("events").
		Select("*", "exact", false).
		Eq("event_date", targetTime).
		ExecuteTo(&results)

	if err == nil && len(results) > 0 {
		for _, ev := range results {
			msg := fmt.Sprintf("--------------------------------------------------\n🔔 **แจ้งเตือนวันสำคัญ!**\n📌 หัวข้อ: %v\n⏰ เวลา: %s",
				ev["title"], formatDisplayTime(ev["event_date"].(string)))
			sendDiscord(msg)
		}
	}
	w.WriteHeader(http.StatusOK)
}

func main() {
	godotenv.Load()
	http.HandleFunc("/api/login", handleLogin)
	http.HandleFunc("/api/users", handleGetAllUsers)
	http.HandleFunc("/api/request", handleCreateRequest)
	http.HandleFunc("/api/my-requests", handleGetMyRequests)
	http.HandleFunc("/api/update-status", handleUpdateStatus)
	http.HandleFunc("/api/events", handleGetMyEvents)
	http.HandleFunc("/api/events/create", handleCreateEvent)
	http.HandleFunc("/api/events/delete", handleDeleteEvent)
	http.HandleFunc("/api/cron/remind", handleCronRemind)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("🚀 Server is live on port %s\n", port)
	http.ListenAndServe(":"+port, nil)
}
