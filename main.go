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

type DailyMood struct {
	UserID    string `json:"user_id"`
	MoodEmoji string `json:"mood_emoji"`
	MoodText  string `json:"mood_text"`
}

// โครงสร้างข้อมูล Wishlist
type WishlistItem struct {
	ID          string `json:"id,omitempty"`
	UserID      string `json:"user_id"`
	ItemName    string `json:"item_name"`
	Description string `json:"item_description"`
	ItemURL     string `json:"item_url"`
	IsReceived  bool   `json:"is_received"`
}

// โครงสร้างข้อมูล Moment
type Moment struct {
	ID       string `json:"id,omitempty"`
	UserID   string `json:"user_id"`
	ImageURL string `json:"image_url"`
	Caption  string `json:"caption"`
}

// โครงสร้างสำหรับเก็บตั้งค่าหน้า Home
type HomeConfig struct {
	ID         string `json:"id,omitempty"`
	ConfigType string `json:"config_type"` // 'slideshow', 'fixed', 'mosaic'
	Data       string `json:"data"`        // เก็บ JSON string ของข้อมูลรูปภาพ
}

// Handler สำหรับดึงข้อมูลตั้งค่าหน้า Home
func handleGetHomeConfig(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var results []map[string]interface{}
	client.From("home_configs").Select("*", "exact", false).ExecuteTo(&results)
	json.NewEncoder(w).Encode(results)
}

// Handler สำหรับบันทึกตั้งค่าหน้า Home
func handleUpdateHomeConfig(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	var config HomeConfig
	json.NewDecoder(r.Body).Decode(&config)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	// ลบอันเก่าแล้วใส่ใหม่ (Upsert)
	client.From("home_configs").Delete("", "").Eq("config_type", config.ConfigType).Execute()
	client.From("home_configs").Insert(config, false, "", "", "").Execute()

	w.WriteHeader(http.StatusOK)
}

// 1. บันทึกรูปภาพประจำวัน
func handleSaveMoment(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	var m struct {
		UserID    string   `json:"user_id"`
		ImageURL  string   `json:"image_url"`
		Caption   string   `json:"caption"`
		VisibleTo []string `json:"visible_to"`
	}
	json.NewDecoder(r.Body).Decode(&m)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("moments").Insert(m, false, "", "", "").Execute()

	go func() {
		// ✅ ส่ง PWA เฉพาะคนที่ระบุมา
		for _, tid := range m.VisibleTo {
			triggerPushNotification(tid, "📸 Moment ใหม่!", "แฟนของคุณเพิ่งลงรูปภาพประจำวันล่ะ! ✨")
		}
		sendDiscordEmbed("📸 New Moment!", "อัปโหลดรูปภาพประจำวันแล้ว", 3447003, nil, m.ImageURL)
	}()
	w.WriteHeader(http.StatusCreated)
}

// 2. ดึงรูปภาพทั้งหมด
func handleGetMoments(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var results []map[string]interface{}
	client.From("moments").Select("*", "exact", false).Order("created_at", &postgrest.OrderOpts{Ascending: false}).Limit(30, "").ExecuteTo(&results)
	json.NewEncoder(w).Encode(results)
}

// 1. ฟังก์ชันบันทึกของที่อยากได้
func handleSaveWishlist(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	var item struct {
		UserID      string   `json:"user_id"`
		ItemName    string   `json:"item_name"`
		Description string   `json:"item_description"` // เพิ่มตัวแปรรับค่า
		ItemURL     string   `json:"item_url"`
		VisibleTo   []string `json:"visible_to"`
	}
	json.NewDecoder(r.Body).Decode(&item)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("wishlists").Insert(item, false, "", "", "").Execute()

	go func() {
		fields := []map[string]interface{}{
			{"name": "🎁 สิ่งของ", "value": item.ItemName, "inline": true},
			{"name": "รายละเอียด", "value": item.Description, "inline": false},
		}
		if item.ItemURL != "" {
			fields = append(fields, map[string]interface{}{"name": "🔗 ลิงก์สินค้า", "value": item.ItemURL, "inline": false})
		}
		// สีทอง 16753920
		sendDiscordEmbed("🎁 แฟนลงของที่อยากได้ใหม่!", "ไปแอบดูหน่อยว่าแฟนอยากได้อะไรน้า~", 16753920, fields, "")

		for _, tid := range item.VisibleTo {
			triggerPushNotification(tid, "🎁 แฟนลงของที่อยากได้ใหม่!", "อยากได้: "+item.ItemName)
		}
	}()
	w.WriteHeader(http.StatusCreated)
}

// 2. ฟังก์ชันดึงรายการทั้งหมด
func handleGetWishlist(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var results []map[string]interface{}
	client.From("wishlists").Select("*", "exact", false).Order("created_at", &postgrest.OrderOpts{Ascending: false}).ExecuteTo(&results)
	json.NewEncoder(w).Encode(results)
}

// 3. ฟังก์ชันอัปเดตว่าได้รับของแล้ว
func handleCompleteWish(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	id := r.URL.Query().Get("id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("wishlists").Update(map[string]interface{}{"is_received": true}, "", "").Eq("id", id).Execute()
	w.WriteHeader(http.StatusOK)
}

// --- Notification Systems ---

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

	filter := fmt.Sprintf("created_by.eq.%s,visible_to.cs.{%s}", uID, uID)
	client.From("events").Select("*", "exact", false).Or(filter, "").Order("event_date", &postgrest.OrderOpts{Ascending: true}).ExecuteTo(&data)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

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

// ✅ เพิ่มฟังก์ชัน Unsubscribe ตามที่นายต้องการ
func handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	var body struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid Body", 400)
		return
	}
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	// ลบการลงทะเบียนทั้งหมดของ User คนนี้ออก
	client.From("push_subscriptions").Delete("", "").Eq("user_id", body.UserID).Execute()
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

	go func() {
		fields := []map[string]interface{}{
			{"name": "👤 ถึงคุณ", "value": rName, "inline": true},
			{"name": "📝 หัวข้อ", "value": req.Title, "inline": true},
			{"name": "⏰ เวลา", "value": formatDisplayTime(req.TimeStart), "inline": false},
		}
		// สีส้มทอง 16753920
		sendDiscordEmbed("💌 มีคำขอใหม่ส่งถึงคุณ!", "หมวดหมู่: "+req.Header, 16753920, fields, req.ImageURL)
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
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		http.Error(w, "Invalid Body", 400)
		return
	}

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

	// บันทึกลง Database
	client.From("events").Insert(row, false, "", "", "").Execute()

	// ✅ ส่งแจ้งเตือนแบบจัดเต็ม
	go func() {
		// 1. ส่ง Discord แบบสวยงาม (สีชมพูสดใส 16738740)
		fields := []map[string]interface{}{
			{"name": "📅 วันที่", "value": ev.EventDate[:10], "inline": true},
			{"name": "📌 ประเภท", "value": ev.CategoryType, "inline": true},
			{"name": "📝 รายละเอียด", "value": ev.Description, "inline": false},
		}
		sendDiscordEmbed("💖 เพิ่มวันสำคัญใหม่แล้ว!", "หัวข้อ: "+ev.Title, 16738740, fields, "")

		// 2. ส่ง Push เข้ามือถือ (PWA)
		for _, uid := range ev.VisibleTo {
			triggerPushNotification(uid, "💖 มีวันพิเศษใหม่!", "อย่าลืมนะ: "+ev.Title)
		}
	}()

	w.WriteHeader(http.StatusCreated)
}

func handleDeleteEvent(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	id := r.URL.Query().Get("id")
	title := r.URL.Query().Get("title")
	uID := r.URL.Query().Get("user_id")

	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	// ✅ 1. ต้องดึงข้อมูลไว้ก่อนลบ เพราะถ้าลบแล้วจะหา visible_to ไม่เจอ
	var results []map[string]interface{}
	client.From("events").Select("visible_to", "exact", false).Eq("id", id).ExecuteTo(&results)

	// ✅ 2. ทำการลบข้อมูลจริง
	client.From("events").Delete("", "").Eq("id", id).Execute()

	go func() {
		// ส่ง Discord
		sendDiscordEmbed("🗑️ ลบวันพิเศษ", "ลบหัวข้อ: "+title, 15158332, nil, "")

		// ✅ 3. ส่ง PWA แจ้งเตือนแฟน (คนที่มีรายชื่อใน visible_to แต่ไม่ใช่คนลบ)
		if len(results) > 0 {
			if v, ok := results[0]["visible_to"].([]interface{}); ok {
				for _, uid := range v {
					targetID := uid.(string)
					if targetID != uID { // ไม่ส่งหาคนกดลบ
						triggerPushNotification(targetID, "🗑️ นัดหมายถูกยกเลิก", "นัดหมาย '"+title+"' ถูกลบออกแล้ว")
					}
				}
			}
		}
	}()
	w.WriteHeader(http.StatusOK)
}

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

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var users []map[string]interface{}
	client.From("users").Select("*", "exact", false).Eq("id", body.ID).ExecuteTo(&users)

	if len(users) > 0 {
		if body.Username != users[0]["username"].(string) {
			if err := bcrypt.CompareHashAndPassword([]byte(users[0]["password"].(string)), []byte(body.ConfirmPassword)); err != nil {
				http.Error(w, "รหัสผ่านไม่ถูกต้องสำหรับการเปลี่ยนชื่อผู้ใช้งาน", http.StatusUnauthorized)
				return
			}
		}
	}

	updateData := map[string]interface{}{
		"username":    body.Username,
		"description": body.Description,
		"gender":      body.Gender,
		"avatar_url":  body.AvatarURL,
	}

	_, _, err := client.From("users").Update(updateData, "", "").Eq("id", body.ID).Execute()
	if err != nil {
		http.Error(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Update successful")
}

func handleCheckSubscription(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	uID := r.URL.Query().Get("user_id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var results []map[string]interface{}
	// ✅ ตรวจสอบว่ามีแถวข้อมูลของ User นี้ในตารางแจ้งเตือนไหม
	client.From("push_subscriptions").Select("id", "exact", false).Eq("user_id", uID).ExecuteTo(&results)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"subscribed": len(results) > 0})
}

// ฟังก์ชันบันทึกอารมณ์
// ✅ ฟังก์ชันบันทึกอารมณ์ (Mood)
func handleSaveMood(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	var m struct {
		UserID    string   `json:"user_id"`
		MoodEmoji string   `json:"mood_emoji"`
		MoodText  string   `json:"mood_text"`
		VisibleTo []string `json:"visible_to"`
	}
	json.NewDecoder(r.Body).Decode(&m)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("daily_moods").Insert(m, false, "", "", "").Execute()

	go func() {
		// ✅ ปรับ Discord ให้แสดง MoodText ด้วย
		fields := []map[string]interface{}{
			{"name": "✨ ความรู้สึก", "value": m.MoodEmoji, "inline": true},
			{"name": "📝 บันทึก", "value": m.MoodText, "inline": false},
		}
		// สีชมพูพาสเทล 16744619
		sendDiscordEmbed("🌈 แฟนอัปเดตอารมณ์ใหม่!", "วันนี้แฟนของคุณรู้สึกอย่างไรบ้างนะ?", 16744619, fields, "")

		for _, tid := range m.VisibleTo {
			triggerPushNotification(tid, "🌈 แฟนอัปเดตอารมณ์แล้ว", "ตอนนี้รู้สึก: "+m.MoodEmoji)
		}
	}()
	w.WriteHeader(http.StatusCreated)
}

// ฟังก์ชันดึงประวัติอารมณ์
// ฟังก์ชันดึงประวัติอารมณ์
func handleGetMoods(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var results []map[string]interface{}

	// ✅ แก้ไขจาก .Limit(20) เป็น .Limit(20, "")
	// พารามิเตอร์ตัวที่สองคือ offset (ตำแหน่งเริ่มต้น) ให้ใส่เป็นค่าว่าง "" ครับ
	client.From("daily_moods").Select("*", "exact", false).Order("created_at", &postgrest.OrderOpts{Ascending: false}).Limit(20, "").ExecuteTo(&results)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// ลบ Mood
func handleDeleteMood(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	id := r.URL.Query().Get("id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("daily_moods").Delete("", "").Eq("id", id).Execute()
	w.WriteHeader(http.StatusOK)
}

// ลบ Wishlist
func handleDeleteWishlist(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	id := r.URL.Query().Get("id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("wishlists").Delete("", "").Eq("id", id).Execute()
	w.WriteHeader(http.StatusOK)
}

// ลบ Moment
func handleDeleteMoment(w http.ResponseWriter, r *http.Request) {
	if enableCORS(&w, r) {
		return
	}
	id := r.URL.Query().Get("id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("moments").Delete("", "").Eq("id", id).Execute()
	w.WriteHeader(http.StatusOK)
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
	http.HandleFunc("/api/events", handleGetMyEvents)
	http.HandleFunc("/api/update-status", handleUpdateStatus)
	http.HandleFunc("/api/events/create", handleCreateEvent)
	http.HandleFunc("/api/events/delete", handleDeleteEvent)
	http.HandleFunc("/api/highlights", handleGetHighlights)
	http.HandleFunc("/api/my-requests", handleGetMyRequests)
	http.HandleFunc("/api/save-subscription", saveSubscriptionHandler)
	http.HandleFunc("/api/unsubscribe", handleUnsubscribe) // ✅ เพิ่ม Route นี้
	http.HandleFunc("/api/users/update", handleUpdateProfile)
	http.HandleFunc("/api/check-subscription", handleCheckSubscription)
	http.HandleFunc("/api/save-mood", handleSaveMood) // ฟังก์ชันบันทึกอารมณ์
	http.HandleFunc("/api/get-moods", handleGetMoods)
	http.HandleFunc("/api/wishlist/save", handleSaveWishlist)
	http.HandleFunc("/api/wishlist/get", handleGetWishlist)
	http.HandleFunc("/api/wishlist/complete", handleCompleteWish)
	http.HandleFunc("/api/moment/save", handleSaveMoment)
	http.HandleFunc("/api/moment/get", handleGetMoments)
	http.HandleFunc("/api/mood/delete", handleDeleteMood)
	http.HandleFunc("/api/wishlist/delete", handleDeleteWishlist)
	http.HandleFunc("/api/moment/delete", handleDeleteMoment)
	http.HandleFunc("/api/home-config/get", handleGetHomeConfig)
	http.HandleFunc("/api/home-config/update", handleUpdateHomeConfig)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("🚀 Server live on %s", port)
	http.ListenAndServe(":"+port, nil)
}
