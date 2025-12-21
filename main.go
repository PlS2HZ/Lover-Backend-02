package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/SherClockHolmes/webpush-go"
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

// --- Notification Systems ---

// ✅ ส่ง Web Push เข้ามือถือ
func triggerPushNotification(userID string, title string, message string) {
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var results []map[string]interface{}

	client.From("push_subscriptions").Select("subscription_json", "exact", false).Eq("user_id", userID).ExecuteTo(&results)

	for _, res := range results {
		subStr, ok := res["subscription_json"].(string)
		if !ok {
			b, _ := json.Marshal(res["subscription_json"])
			subStr = string(b)
		}

		s := &webpush.Subscription{}
		json.Unmarshal([]byte(subStr), s)

		resp, err := webpush.SendNotification([]byte(fmt.Sprintf(`{"title":"%s", "body":"%s", "url":"/"}`, title, message)), s, &webpush.Options{
			Subscriber:      os.Getenv("VAPID_EMAIL"),
			VAPIDPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
			VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
			TTL:             30,
		})
		if err == nil {
			resp.Body.Close()
		}
	}
}

func handleGetMyEvents(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	uID := r.URL.Query().Get("user_id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var data []map[string]interface{}

	// ✅ ปรับ Filter ให้รองรับทั้งวันที่นายสร้าง (created_by) และที่แฟนแชร์ให้ (visible_to)
	filter := fmt.Sprintf("created_by.eq.%s,visible_to.cs.{%s}", uID, uID)
	client.From("events").Select("*", "exact", false).Or(filter, "").Order("event_date", &postgrest.OrderOpts{Ascending: true}).ExecuteTo(&data)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// ✅ ส่ง Discord Embed
func sendDiscordEmbed(title, description string, color int, fields []map[string]interface{}, imageURL string) {
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if webhookURL == "" {
		return
	}

	embed := map[string]interface{}{
		"title": title, "description": description, "color": color,
		"footer": map[string]interface{}{"text": "Lover App • " + time.Now().Format("15:04")},
		"fields": fields,
	}
	if imageURL != "" && imageURL != "NULL" {
		embed["image"] = map[string]string{"url": imageURL}
	}
	payload := map[string]interface{}{"content": "@everyone", "embeds": []map[string]interface{}{embed}}
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

func checkAndNotify() {
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	now := time.Now().UTC().Truncate(time.Minute).Format("2006-01-02T15:04:00.000Z")

	var results []map[string]interface{}
	client.From("events").Select("*", "exact", false).Eq("event_date", now).ExecuteTo(&results)

	if len(results) > 0 {
		for _, ev := range results {
			title := ev["title"].(string)
			sendDiscordEmbed("🔔 แจ้งเตือนวันสำคัญ!", title, 16761035, nil, "")
			if visibleTo, ok := ev["visible_to"].([]interface{}); ok {
				for _, uid := range visibleTo {
					go triggerPushNotification(uid.(string), "🔔 ถึงเวลาแล้วนะ!", title)
				}
			}
		}
	}
}

func startSpecialDayReminder() {
	go func() {
		for {
			now := time.Now()
			target := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, now.Location())
			if now.After(target) {
				target = target.Add(24 * time.Hour)
			}
			time.Sleep(time.Until(target))

			client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
			today := time.Now().Format("2006-01-02")
			var results []map[string]interface{}
			client.From("events").Select("*", "exact", false).Eq("category_type", "special").Like("event_date", today+"%").ExecuteTo(&results)

			for _, ev := range results {
				if v, ok := ev["visible_to"].([]interface{}); ok {
					for _, uid := range v {
						go triggerPushNotification(uid.(string), "💖 Happy Special Day!", ev["title"].(string))
					}
				}
			}
		}
	}()
}

// --- Handlers ---

func saveSubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	var sub PushSubscription
	json.NewDecoder(r.Body).Decode(&sub)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("push_subscriptions").Delete("", "").Eq("user_id", sub.UserID).Execute()
	data := map[string]interface{}{"user_id": sub.UserID, "subscription_json": sub.Subscription}
	client.From("push_subscriptions").Insert(data, false, "", "", "").Execute()
	w.WriteHeader(http.StatusOK)
}

func handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	var req RequestBody
	json.NewDecoder(r.Body).Decode(&req)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	var users []map[string]interface{}
	client.From("users").Select("id, username", "exact", false).Eq("username", req.ReceiverUsername).ExecuteTo(&users)
	if len(users) == 0 {
		http.Error(w, "Not Found", 404)
		return
	}

	rID := users[0]["id"].(string)
	rName := users[0]["username"].(string)

	row := map[string]interface{}{
		"category": req.Header, "title": req.Title, "description": req.Duration,
		"sender_id": req.SenderID, "receiver_id": rID, "status": "pending",
		"sender_name": "Someone", "receiver_name": rName,
		"remark": fmt.Sprintf("%s|%s", req.TimeStart, req.TimeEnd), "image_url": req.ImageURL,
	}
	client.From("requests").Insert(row, false, "", "", "").Execute()

	// ✅ กลับมาส่ง Discord Embed เหมือนเดิมแล้วครับ
	go func() {
		fields := []map[string]interface{}{
			{"name": "👤 ถึง", "value": rName, "inline": true},
			{"name": "⏰ เริ่ม", "value": formatDisplayTime(req.TimeStart), "inline": false},
		}
		sendDiscordEmbed("📢 มีคำขอใหม่ส่งถึงคุณ!", "หัวข้อ: "+req.Header, 16737920, fields, req.ImageURL)

		// และส่งเข้ามือถือด้วย
		triggerPushNotification(rID, "📢 มีคำขอใหม่!", "แฟนส่งคำขอ '"+req.Header+"' มาให้จ้า ❤️")
	}()

	w.WriteHeader(http.StatusCreated)
}

func handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	var body struct {
		ID      string
		Status  string
		Comment string
	}
	json.NewDecoder(r.Body).Decode(&body)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("requests").Update(map[string]interface{}{"status": body.Status, "comment": body.Comment}, "", "").Eq("id", body.ID).Execute()

	var results []map[string]interface{}
	client.From("requests").Select("*", "exact", false).Eq("id", body.ID).ExecuteTo(&results)

	if len(results) > 0 {
		item := results[0]
		go func() {
			// ✅ กลับมาส่ง Discord Embed เมื่ออนุมัติ/ปฏิเสธ
			color := 3066993 // Green
			statusTitle := "✅ อนุมัติคำขอแล้ว!"
			if body.Status == "rejected" {
				color = 15158332 // Red
				statusTitle = "❌ ปฏิเสธคำขอ"
			}
			fields := []map[string]interface{}{
				{"name": "📌 หัวข้อ", "value": fmt.Sprintf("%v", item["category"]), "inline": false},
				{"name": "💬 เหตุผล", "value": body.Comment, "inline": false},
			}
			sendDiscordEmbed(statusTitle, "มีอัปเดตสถานะคำขอของคุณ", color, fields, "")

			// และส่งเข้ามือถือด้วย
			triggerPushNotification(item["sender_id"].(string), statusTitle, "แฟนพิจารณาคำขอ '"+fmt.Sprintf("%v", item["category"])+"' แล้วจ้า")
		}()
	}
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
		"event_date": ev.EventDate, "title": ev.Title, "description": ev.Description,
		"repeat_type": ev.RepeatType, "is_special": true, "category_type": ev.CategoryType,
	}
	if ev.CreatedBy != "" {
		row["created_by"] = ev.CreatedBy
	}
	if len(ev.VisibleTo) > 0 {
		row["visible_to"] = ev.VisibleTo
	}
	client.From("events").Insert(row, false, "", "", "").Execute()

	for _, uid := range ev.VisibleTo {
		go triggerPushNotification(uid, "💖 มีวันพิเศษใหม่!", ev.Title)
	}
	w.WriteHeader(http.StatusCreated)
}

func handleDeleteEvent(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	id := r.URL.Query().Get("id")
	title := r.URL.Query().Get("title")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("events").Delete("", "").Eq("id", id).Execute()
	sendDiscordEmbed("🗑️ ลบวันพิเศษ", "ลบหัวข้อ: "+title, 15158332, nil, "")
	w.WriteHeader(http.StatusOK)
}

// --- Standard Handlers ---

func handleRegister(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	var u User
	json.NewDecoder(r.Body).Decode(&u)
	hashed, _ := bcrypt.GenerateFromPassword([]byte(u.Password), 10)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("users").Insert(map[string]interface{}{"username": u.Username, "password": string(hashed)}, false, "", "", "").Execute()
	w.WriteHeader(201)
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	var c struct{ Username, Password string }
	json.NewDecoder(r.Body).Decode(&c)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var users []map[string]interface{}
	client.From("users").Select("*", "exact", false).Eq("username", c.Username).ExecuteTo(&users)
	if len(users) > 0 && bcrypt.CompareHashAndPassword([]byte(users[0]["password"].(string)), []byte(c.Password)) == nil {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": users[0]["id"], "exp": time.Now().Add(72 * time.Hour).Unix()})
		t, _ := token.SignedString(jwtKey)
		json.NewEncoder(w).Encode(map[string]interface{}{"token": t, "user_id": users[0]["id"], "username": users[0]["username"]})
		return
	}
	http.Error(w, "Unauthorized", 401)
}

func handleGetHighlights(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	uID := r.URL.Query().Get("user_id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var data []map[string]interface{}
	client.From("events").Select("*", "exact", false).Eq("is_special", "true").Filter("visible_to", "cs", "{"+uID+"}").Order("event_date", &postgrest.OrderOpts{Ascending: true}).ExecuteTo(&data)
	json.NewEncoder(w).Encode(data)
}

// (รวม Endpoint อื่นๆ ตามโครงสร้างเดิม)
func handleGetAllUsers(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var users []map[string]interface{}
	client.From("users").Select("id, username, avatar_url", "exact", false).ExecuteTo(&users)
	json.NewEncoder(w).Encode(users)
}

func handleGetMyRequests(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	uID := r.URL.Query().Get("user_id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var data []map[string]interface{}
	client.From("requests").Select("*", "exact", false).Or(fmt.Sprintf("sender_id.eq.%s,receiver_id.eq.%s", uID, uID), "").Order("created_at", &postgrest.OrderOpts{Ascending: false}).ExecuteTo(&data)
	json.NewEncoder(w).Encode(data)
}

func main() {
	godotenv.Load()
	startSpecialDayReminder()
	ticker := time.NewTicker(time.Minute)
	go func() {
		for range ticker.C {
			checkAndNotify()
		}
	}()

	http.HandleFunc("/api/register", handleRegister)
	http.HandleFunc("/api/login", handleLogin)
	http.HandleFunc("/api/users", handleGetAllUsers)
	http.HandleFunc("/api/request", handleCreateRequest)
	http.HandleFunc("/api/events", handleGetMyEvents) // ดึงรายการมาโชว์ที่ปฏิทิน
	http.HandleFunc("/api/update-status", handleUpdateStatus)
	http.HandleFunc("/api/events/create", handleCreateEvent)
	http.HandleFunc("/api/events/delete", handleDeleteEvent)
	http.HandleFunc("/api/highlights", handleGetHighlights)
	http.HandleFunc("/api/my-requests", handleGetMyRequests)
	http.HandleFunc("/api/save-subscription", saveSubscriptionHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("🚀 Server live on %s", port)
	http.ListenAndServe(":"+port, nil)
}
