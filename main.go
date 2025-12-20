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

func enableCORS(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PATCH, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
}

// 🔔 ฟังก์ชันแจ้งเตือน Discord (คงไว้เป็นหัวใจหลัก)
func sendDiscord(content string) {
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if webhookURL == "" {
		fmt.Println("❌ Discord Error: Webhook URL missing")
		return
	}
	payload := map[string]string{"content": content}
	jsonData, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
}

func formatDisplayTime(t string) string {
	if len(t) >= 16 {
		return t[:10] + " **TIME** " + t[11:16]
	}
	return t
}

func handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)
	if r.Method == "OPTIONS" {
		return
	}
	var req RequestBody
	json.NewDecoder(r.Body).Decode(&req)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	var sender []map[string]interface{}
	client.From("users").Select("username", "exact", false).Eq("id", req.SenderID).ExecuteTo(&sender)
	sName := "Unknown User"
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

	row := map[string]interface{}{
		"category": req.Header, "title": req.Title, "description": req.Duration,
		"sender_id": req.SenderID, "receiver_id": receiver[0]["id"].(string), "status": "pending",
		"sender_name": sName, "receiver_name": rName,
		"remark": fmt.Sprintf("%s|%s", req.TimeStart, req.TimeEnd),
	}
	client.From("requests").Insert(row, false, "", "", "").Execute()

	// 📢 ส่งแจ้งเตือนไป Discord
	appLink := "https://lover-frontend-ashen.vercel.app"
	msg := fmt.Sprintf("--------------------------------------------------\n🔔 @everyone\n## 💖 มีคำขอใหม่ส่งถึงคุณ!\n> 📌 **หัวข้อ:** %s\n> 👤 **จาก:** %s\n> 📩 **ถึง:** %s\n> 📝 **รายละเอียด:** %s\n---\n> 🗓️ **เริ่ม:** %s\n> 🏁 **จบ:** %s\n🔗 **Link:** %s",
		req.Header, sName, rName, req.Title, formatDisplayTime(req.TimeStart), formatDisplayTime(req.TimeEnd), appLink)

	sendDiscord(msg)

	w.WriteHeader(http.StatusCreated)
}

func handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
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

	w.WriteHeader(http.StatusOK)

	// 📢 แจ้งผลการพิจารณาไป Discord
	// 📢 แจ้งผลการพิจารณาไป Discord
	go func() {
		var results []map[string]interface{}
		client.From("requests").Select("*", "exact", false).Eq("id", body.ID).ExecuteTo(&results)
		if len(results) > 0 {
			item := results[0]

			// ✅ แก้ไข: ใช้แค่ตัวแปร emoji ตัวเดียวเพื่อให้ไม่เกิด Unused Variable Error
			emoji := "✅ อนุมัติ"
			if body.Status == "rejected" {
				emoji = "❌ ไม่อนุมัติ"
			}

			msg := fmt.Sprintf("📢 @everyone ผลการพิจารณาคำขอ!\n**สถานะ:** %s\n> 👤 **จาก:** %v\n> 📩 **ถึง:** %v\n> 📌 **หัวข้อ:** %v\n🔗 **Link:** %s",
				emoji, item["sender_name"], item["receiver_name"], item["category"], "https://lover-frontend-ashen.vercel.app/")

			sendDiscord(msg)
		}
	}()
}

// --- ฟังก์ชันเสริมอื่นๆ ---
func handleLogin(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)
	if r.Method == "OPTIONS" {
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
	err := bcrypt.CompareHashAndPassword([]byte(users[0]["password"].(string)), []byte(creds.Password))
	if err != nil {
		http.Error(w, "Wrong password", http.StatusUnauthorized)
		return
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": users[0]["id"], "username": users[0]["username"], "exp": time.Now().Add(time.Hour * 72).Unix()})
	tokenString, _ := token.SignedString(jwtKey)
	json.NewEncoder(w).Encode(map[string]string{"token": tokenString, "username": users[0]["username"].(string), "user_id": users[0]["id"].(string)})
}

func handleGetMyRequests(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)
	uID := r.URL.Query().Get("user_id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var data []map[string]interface{}
	client.From("requests").Select("*", "exact", false).Or(fmt.Sprintf("sender_id.eq.%s,receiver_id.eq.%s", uID, uID), "").ExecuteTo(&data)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func handleGetAllUsers(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var users []map[string]interface{}
	client.From("users").Select("id, email, username", "exact", false).ExecuteTo(&users)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func main() {
	godotenv.Load()
	http.HandleFunc("/api/login", handleLogin)
	http.HandleFunc("/api/request", handleCreateRequest)
	http.HandleFunc("/api/my-requests", handleGetMyRequests)
	http.HandleFunc("/api/update-status", handleUpdateStatus)
	http.HandleFunc("/api/users", handleGetAllUsers)

	fmt.Println("Server is running (Discord Notification Only)...")
	http.ListenAndServe(":8080", nil)
}
