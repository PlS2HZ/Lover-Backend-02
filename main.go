package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"github.com/supabase-community/postgrest-go"
	"github.com/supabase-community/supabase-go"
	"golang.org/x/crypto/bcrypt"
)

var jwtKey = []byte("your_secret_key_2025")

// --- Types ---
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
	ID           string   `json:"id,omitempty"`
	EventDate    string   `json:"event_date"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	CreatedBy    string   `json:"created_by"`
	VisibleTo    []string `json:"visible_to"`
	RepeatType   string   `json:"repeat_type"`
	IsSpecial    bool     `json:"is_special"`
	CategoryType string   `json:"category_type"`
}
type PushSubscription struct {
	UserID       string      `json:"user_id"`
	Subscription interface{} `json:"subscription"`
}

// --- Discord Embed System ---

// ✅ ฟังก์ชันส่ง Embed แบบสวยงาม
func sendDiscordEmbed(title, description string, color int, fields []map[string]interface{}, imageURL string) {
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if webhookURL == "" {
		return
	}

	embed := map[string]interface{}{
		"title":       title,
		"description": description,
		"color":       color,
		"footer": map[string]interface{}{
			"text": "Lover App Notification • " + time.Now().Format("15:04"),
		},
		"fields": fields,
	}

	// ถ้ามีรูปภาพ ให้ใส่ใน Embed
	if imageURL != "" && imageURL != "NULL" {
		embed["image"] = map[string]string{"url": imageURL}
	}

	payload := map[string]interface{}{
		"content": "@everyone", // แท็กทุกคนเหมือนเดิม
		"embeds":  []map[string]interface{}{embed},
	}

	jsonData, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
}

// --- Helpers ---

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

func formatDisplayTime(t string) string {
	parsedTime, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return t
	}
	thailandTime := parsedTime.In(time.FixedZone("Asia/Bangkok", 7*3600))
	return thailandTime.Format("2006-01-02 เวลา 15:04 น.")
}

// --- Cron Jobs ---

var lastNotifiedMinute string

func checkAndNotify() {
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	now := time.Now().UTC().Truncate(time.Minute)
	targetTime := now.Format("2006-01-02T15:04:00.000Z")

	if lastNotifiedMinute == targetTime {
		return
	}

	var results []map[string]interface{}
	_, err := client.From("events").Select("*", "exact", false).Eq("event_date", targetTime).ExecuteTo(&results)

	if err == nil && len(results) > 0 {
		lastNotifiedMinute = targetTime
		for _, ev := range results {
			fields := []map[string]interface{}{
				{"name": "📝 รายละเอียด", "value": ev["description"], "inline": false},
				{"name": "⏰ เวลา", "value": formatDisplayTime(ev["event_date"].(string)), "inline": true},
			}
			sendDiscordEmbed("🔔 แจ้งเตือนวันสำคัญถึงเวลาแล้ว!", fmt.Sprintf("หัวข้อ: %v", ev["title"]), 16761035, fields, "")
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

// --- Handlers ---

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

	// ✅ ปรับแจ้งเตือนคำขอใหม่
	go func() {
		fields := []map[string]interface{}{
			{"name": "👤 จาก", "value": sName, "inline": true},
			{"name": "👤 ถึง", "value": rName, "inline": true},
			{"name": "⏰ เริ่ม", "value": formatDisplayTime(req.TimeStart), "inline": false},
			{"name": "⏰ จบ", "value": formatDisplayTime(req.TimeEnd), "inline": false},
		}
		sendDiscordEmbed("📢 มีคำขอใหม่ส่งถึงคุณ!", "หัวข้อ: "+req.Header, 16737920, fields, req.ImageURL)
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

	// ✅ ปรับแจ้งเตือนผลการพิจารณา
	go func() {
		var results []map[string]interface{}
		client.From("requests").Select("*", "exact", false).Eq("id", body.ID).ExecuteTo(&results)
		if len(results) > 0 {
			item := results[0]
			color := 3066993 // Green
			statusTitle := "✅ อนุมัติคำขอแล้ว!"
			if body.Status == "rejected" {
				color = 15158332 // Red
				statusTitle = "❌ ปฏิเสธคำขอ"
			}
			fields := []map[string]interface{}{
				{"name": "📌 หัวข้อ", "value": fmt.Sprintf("%v", item["category"]), "inline": false},
				{"name": "💬 เหตุผล", "value": body.Comment, "inline": false},
				{"name": "👤 โดย", "value": fmt.Sprintf("%v", item["receiver_name"]), "inline": true},
			}
			img, _ := item["image_url"].(string)
			sendDiscordEmbed(statusTitle, "มีอัปเดตสถานะคำขอของคุณ", color, fields, img)
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
		"event_date":    ev.EventDate,
		"title":         ev.Title,
		"description":   ev.Description,
		"repeat_type":   ev.RepeatType,
		"is_special":    true,
		"category_type": ev.CategoryType,
	}
	if ev.CreatedBy != "" {
		row["created_by"] = ev.CreatedBy
	}
	if len(ev.VisibleTo) > 0 {
		row["visible_to"] = ev.VisibleTo
	}
	client.From("events").Insert(row, false, "", "", "").Execute()

	// ✅ แจ้งเตือนบันทึกวันพิเศษใหม่
	go func() {
		fields := []map[string]interface{}{
			{"name": "📅 วันที่", "value": formatDisplayTime(ev.EventDate), "inline": true},
			{"name": "🔄 วนซ้ำ", "value": ev.RepeatType, "inline": true},
		}
		sendDiscordEmbed("💖 มีการบันทึกวันพิเศษใหม่!", "หัวข้อ: "+ev.Title, 16737920, fields, "")
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
	title := r.URL.Query().Get("title")
	uID := r.URL.Query().Get("user_id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var user []map[string]interface{}
	client.From("users").Select("username", "exact", false).Eq("id", uID).ExecuteTo(&user)
	uName := "ใครบางคน"
	if len(user) > 0 {
		uName = user[0]["username"].(string)
	}
	client.From("events").Delete("", "").Eq("id", id).Execute()

	// ✅ แจ้งเตือนการลบ
	go func() {
		fields := []map[string]interface{}{
			{"name": "👤 ผู้ดำเนินการ", "value": uName, "inline": true},
		}
		sendDiscordEmbed("🗑️ มีการลบวันพิเศษออก!", "หัวข้อที่ลบ: "+title, 15158332, fields, "")
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
		"username": body.Username, "description": body.Description, "gender": body.Gender, "avatar_url": body.AvatarURL,
	}
	client.From("users").Update(updateData, "", "").Eq("id", body.ID).Execute()
	w.WriteHeader(http.StatusOK)
}

// วางไว้ด้านบนของ func main()
func saveSubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	} // อย่าลืมใส่ CORS ด้วยครับเพื่อให้หน้าบ้านส่งมาได้

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var sub PushSubscription
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data := map[string]interface{}{
		"user_id":           sub.UserID,
		"subscription_json": sub.Subscription,
	}

	// แก้ไข: ใช้ client (supabase) ที่นายมีอยู่แล้วใน main.go
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	_, _, err := client.From("push_subscriptions").Insert(data, false, "", "", "").Execute()

	if err != nil {
		log.Printf("Error saving subscription: %v", err) // ตอนนี้ log จะไม่ undefined แล้ว
		http.Error(w, "Failed to save subscription", http.StatusInternalServerError)
		return
	}

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
	http.HandleFunc("/api/save-subscription", saveSubscriptionHandler)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("🚀 Server live on %s\n", port)
	http.ListenAndServe(":"+port, nil)
}
