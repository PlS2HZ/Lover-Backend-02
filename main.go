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
	"github.com/supabase-community/postgrest-go" // ✨ เพิ่มอันนี้เพื่อให้ใช้ OrderOpts ได้
	"github.com/supabase-community/supabase-go"
	"golang.org/x/crypto/bcrypt"
)

var jwtKey = []byte("your_secret_key_2025")

type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	AvatarURL   string `json:"avatar_url"`
	Description string `json:"description"`
	Gender      string `json:"gender"`
}

type RequestBody struct {
	ID               string `json:"id,omitempty"`
	Header           string `json:"header"`
	Title            string `json:"title"`
	Duration         string `json:"duration"`
	SenderID         string `json:"sender_id"`
	ReceiverUsername string `json:"receiver_username"`
	TimeStart        string `json:"time_start"`
	TimeEnd          string `json:"time_end"`
	ImageURL         string `json:"image_url"`
}

type Event struct {
	ID          string   `json:"id,omitempty"`
	EventDate   string   `json:"event_date"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	CreatedBy   string   `json:"created_by"`
	VisibleTo   []string `json:"visible_to"`
	RepeatType  string   `json:"repeat_type"`
	IsSpecial   bool     `json:"is_special"` // ✅ สำหรับดึงวันสำคัญไปโชว์หน้า Home
}

func enableCORS(w *http.ResponseWriter, r *http.Request) bool {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

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

func formatDisplayTime(t string) string {
	parsedTime, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return t
	}
	thailandTime := parsedTime.In(time.FixedZone("Asia/Bangkok", 7*3600))
	return thailandTime.Format("2006-01-02 เวลา 15:04:05")
}

var lastNotifiedMinute string

func checkAndNotify() {
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	// ✅ ดึงเวลา UTC ปัจจุบัน และตัดวินาที/มิลลิวินาทีให้เป็น 00:00
	now := time.Now().UTC().Truncate(time.Minute)
	targetTime := now.Format("2006-01-02T15:04:00.000Z")

	if lastNotifiedMinute == targetTime {
		return
	}

	var results []map[string]interface{}
	// ค้นหา Event ที่มีเวลาตรงกับ targetTime เป๊ะๆ
	_, err := client.From("events").Select("*", "exact", false).Eq("event_date", targetTime).ExecuteTo(&results)

	if err != nil {
		fmt.Printf("❌ Database error: %v\n", err)
		return
	}

	if len(results) > 0 {

		lastNotifiedMinute = targetTime

		for _, ev := range results {
			msg := fmt.Sprintf("--------------------------------------------------\n🔔 **แจ้งเตือนวันสำคัญถึงเวลาแล้ว!**\n📌 หัวข้อ: %v\n📝 รายละเอียด: %v\n⏰ เวลา: %s\nLink: https://lover-frontend-ashen.vercel.app/",
				ev["title"], ev["description"], formatDisplayTime(ev["event_date"].(string)))
			sendDiscord(msg)
		}
	}
}

func startCronJob() {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for range ticker.C {
			checkAndNotify()
		}
	}()
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	var user User
	json.NewDecoder(r.Body).Decode(&user)
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	row := map[string]interface{}{"username": user.Username, "password": string(hashedPassword)}
	_, _, err := client.From("users").Insert(row, false, "", "", "").Execute()
	if err != nil {
		http.Error(w, "Conflict", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&creds)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var users []map[string]interface{}
	client.From("users").Select("*", "exact", false).Eq("username", creds.Username).ExecuteTo(&users)
	if len(users) == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(users[0]["password"].(string)), []byte(creds.Password)); err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": users[0]["id"], "username": users[0]["username"], "exp": time.Now().Add(time.Hour * 72).Unix()})
	tokenString, _ := token.SignedString(jwtKey)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": tokenString, "username": users[0]["username"], "user_id": users[0]["id"], "avatar_url": users[0]["avatar_url"],
	})
}

func handleGetAllUsers(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var users []map[string]interface{}
	client.From("users").Select("id, username, avatar_url, description, gender", "exact", false).ExecuteTo(&users)
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
	sName := sender[0]["username"].(string)

	var receiver []map[string]interface{}
	client.From("users").Select("id, username", "exact", false).Eq("username", req.ReceiverUsername).ExecuteTo(&receiver)
	if len(receiver) == 0 {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	rName := receiver[0]["username"].(string)

	row := map[string]interface{}{
		"category":      req.Header,
		"title":         req.Title,
		"description":   req.Duration,
		"sender_id":     req.SenderID,
		"receiver_id":   receiver[0]["id"].(string),
		"status":        "pending",
		"sender_name":   sName,
		"receiver_name": rName,
		"remark":        fmt.Sprintf("%s|%s", req.TimeStart, req.TimeEnd),
		"image_url":     req.ImageURL,
	}
	client.From("requests").Insert(row, false, "", "", "").Execute()

	go func() {
		msg := fmt.Sprintf("--------------------------------------------------\n@everyone มีคำขอใหม่ส่งถึงคุณ!\nหัวข้อ: %s\nจาก: %s\nถึง: %s\nเริ่ม: %s\nจบ: %s\nLink: https://lover-frontend-ashen.vercel.app/", req.Header, sName, rName, formatDisplayTime(req.TimeStart), formatDisplayTime(req.TimeEnd))
		sendDiscord(msg)
	}()
	w.WriteHeader(http.StatusCreated)
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
	updateData := map[string]interface{}{
		"status":       body.Status,
		"processed_at": time.Now().Format(time.RFC3339),
		"comment":      body.Comment,
	}
	client.From("requests").Update(updateData, "", "").Eq("id", body.ID).Execute()

	go func() {
		var results []map[string]interface{}
		client.From("requests").Select("*", "exact", false).Eq("id", body.ID).ExecuteTo(&results)
		if len(results) > 0 {
			item := results[0]
			statusText := "อนุมัติ"
			if body.Status == "rejected" {
				statusText = "ไม่อนุมัติ"
			}
			msg := fmt.Sprintf("--------------------------------------------------\n@everyone ผลการพิจารณาคำขอ!\nสถานะ: %s\nจาก: %v\nถึง: %v\nหัวข้อ: %v\nLink: https://lover-frontend-ashen.vercel.app/", statusText, item["sender_name"], item["receiver_name"], item["category"])
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

	row := map[string]interface{}{
		"event_date":  ev.EventDate,
		"title":       ev.Title,
		"description": ev.Description,
		"repeat_type": ev.RepeatType,
		"is_special":  true, // บังคับให้เป็น true
	}
	if ev.CreatedBy != "" {
		row["created_by"] = ev.CreatedBy
	}
	if len(ev.VisibleTo) > 0 {
		row["visible_to"] = ev.VisibleTo
	}
	client.From("events").Insert(row, false, "", "", "").Execute()

	// ✨ แจ้งเตือน Discord ทันทีเมื่อบันทึก
	go func() {
		msg := fmt.Sprintf("--------------------------------------------------\n💖 **มีการบันทึกวันพิเศษใหม่!**\n📌 หัวข้อ: %s\n📅 วันที่: %s\nLink: https://lover-frontend-ashen.vercel.app/", ev.Title, formatDisplayTime(ev.EventDate))
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

	// ✅ แก้ไข: ใช้ postgrest.OrderOpts ตามที่ Compiler ต้องการ
	client.From("requests").
		Select("*", "exact", false).
		Or(fmt.Sprintf("sender_id.eq.%s,receiver_id.eq.%s", uID, uID), "").
		Order("created_at", &postgrest.OrderOpts{Ascending: false}).
		ExecuteTo(&data)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func handleGetMyEvents(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	uID := r.URL.Query().Get("user_id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var data []map[string]interface{}
	filter := fmt.Sprintf("created_by.eq.%s,visible_to.cs.{%s}", uID, uID)

	// ✅ แก้ไข: เรียงตามวันที่จัดงาน (Ascending: true)
	client.From("events").
		Select("*", "exact", false).
		Or(filter, "").
		Order("event_date", &postgrest.OrderOpts{Ascending: true}).
		ExecuteTo(&data)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func handleGetHighlights(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	uID := r.URL.Query().Get("user_id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var data []map[string]interface{}
	filter := fmt.Sprintf("is_special.eq.true,and(or(created_by.eq.%s,visible_to.cs.{%s}))", uID, uID)

	// ✅ แก้ไข: เรียงวันสำคัญตามลำดับเวลา
	client.From("events").
		Select("*", "exact", false).
		Or(filter, "").
		Order("event_date", &postgrest.OrderOpts{Ascending: true}).
		ExecuteTo(&data)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func handleDeleteEvent(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	id := r.URL.Query().Get("id")
	title := r.URL.Query().Get("title") // ดึงชื่อมาโชว์
	uID := r.URL.Query().Get("user_id")

	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	// ดึงชื่อผู้ลบ (ถ้าต้องการ)
	var user []map[string]interface{}
	client.From("users").Select("username", "exact", false).Eq("id", uID).ExecuteTo(&user)
	uName := "ใครบางคน"
	if len(user) > 0 {
		uName = user[0]["username"].(string)
	}

	// ลบข้อมูล
	client.From("events").Delete("", "").Eq("id", id).Execute()

	// ✨ แจ้งเตือน Discord เมื่อมีการลบ
	go func() {
		msg := fmt.Sprintf("--------------------------------------------------\n🗑️ **มีการลบวันพิเศษออก!**\n📌 หัวข้อที่ลบ: %s\n👤 ผู้ดำเนินการ: %s\nLink: https://lover-frontend-ashen.vercel.app/", title, uName)
		sendDiscord(msg)
	}()

	w.WriteHeader(http.StatusOK)
}

func handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	var body struct {
		ID              string `json:"id"`
		Username        string `json:"username"`
		Description     string `json:"description"`
		Gender          string `json:"gender"`
		AvatarURL       string `json:"avatar_url"`
		ConfirmPassword string `json:"confirm_password"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	var users []map[string]interface{}
	client.From("users").Select("*", "exact", false).Eq("id", body.ID).ExecuteTo(&users)

	if len(users) > 0 && body.Username != users[0]["username"].(string) {
		if err := bcrypt.CompareHashAndPassword([]byte(users[0]["password"].(string)), []byte(body.ConfirmPassword)); err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	updateData := map[string]interface{}{
		"username":    body.Username,
		"description": body.Description,
		"gender":      body.Gender,
		"avatar_url":  body.AvatarURL,
	}
	client.From("users").Update(updateData, "", "").Eq("id", body.ID).Execute()
	w.WriteHeader(http.StatusOK)
}

func main() {
	godotenv.Load()
	startCronJob()

	http.HandleFunc("/api/register", handleRegister)
	http.HandleFunc("/api/login", handleLogin)
	http.HandleFunc("/api/users", handleGetAllUsers)
	http.HandleFunc("/api/request", handleCreateRequest)
	http.HandleFunc("/api/my-requests", handleGetMyRequests)
	http.HandleFunc("/api/update-status", handleUpdateStatus)
	http.HandleFunc("/api/events", handleGetMyEvents)
	http.HandleFunc("/api/events/create", handleCreateEvent)
	http.HandleFunc("/api/events/delete", handleDeleteEvent)
	http.HandleFunc("/api/highlights", handleGetHighlights)
	http.HandleFunc("/api/users/update", handleUpdateProfile)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("🚀 Server live on %s\n", port)
	http.ListenAndServe(":"+port, nil)
}
