package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time" // ✅ ต้องมีเพื่อใช้ใน HandleLogin และ Reminder

	"couple-app/models"
	"couple-app/services"
	"couple-app/utils"

	"github.com/golang-jwt/jwt/v5"
	"github.com/supabase-community/postgrest-go"
	"github.com/supabase-community/supabase-go"
	"golang.org/x/crypto/bcrypt"
)

var jwtKey = []byte("your_secret_key_2025")

// --- Auth & User Handlers ---

func HandleRegister(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var u models.User
	json.NewDecoder(r.Body).Decode(&u)
	hashed, _ := bcrypt.GenerateFromPassword([]byte(u.Password), 10)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("users").Insert(map[string]interface{}{"username": u.Username, "password": string(hashed)}, false, "", "", "").Execute()
	w.WriteHeader(201)
}

func HandleLogin(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var c struct{ Username, Password string }
	json.NewDecoder(r.Body).Decode(&c)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var users []map[string]interface{}
	client.From("users").Select("*", "exact", false).Eq("username", c.Username).ExecuteTo(&users)
	if len(users) > 0 && bcrypt.CompareHashAndPassword([]byte(users[0]["password"].(string)), []byte(c.Password)) == nil {
		// ✅ แก้ไข: ใช้ jwt.NumericDate เพื่อความถูกต้องของ Library
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": users[0]["id"],
			"exp":     time.Now().Add(72 * time.Hour).Unix(),
		})
		t, _ := token.SignedString(jwtKey)
		json.NewEncoder(w).Encode(map[string]interface{}{"token": t, "user_id": users[0]["id"], "username": users[0]["username"]})
		return
	}
	http.Error(w, "Unauthorized", 401)
}

func HandleGetAllUsers(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var users []map[string]interface{}
	client.From("users").Select("id, username, avatar_url, description, gender", "exact", false).ExecuteTo(&users)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func HandleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
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

	updateData := map[string]interface{}{"username": body.Username, "description": body.Description, "gender": body.Gender, "avatar_url": body.AvatarURL}
	client.From("users").Update(updateData, "", "").Eq("id", body.ID).Execute()
	w.WriteHeader(http.StatusOK)
}

// --- Home Config Handlers ---

func HandleGetHomeConfig(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var results []map[string]interface{}
	client.From("home_configs").Select("*", "exact", false).ExecuteTo(&results)
	json.NewEncoder(w).Encode(results)
}

func HandleUpdateHomeConfig(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var config models.HomeConfig
	json.NewDecoder(r.Body).Decode(&config)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("home_configs").Delete("", "").Eq("config_type", config.ConfigType).Execute()
	client.From("home_configs").Insert(config, false, "", "", "").Execute()
	w.WriteHeader(http.StatusOK)
}

// --- Mood Handlers ---

func HandleSaveMood(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
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
		fields := []map[string]interface{}{
			{"name": "✨ ความรู้สึก", "value": m.MoodEmoji, "inline": true},
			{"name": "📝 บันทึก", "value": m.MoodText, "inline": false},
		}
		services.SendDiscordEmbed("🌈 แฟนอัปเดตอารมณ์ใหม่!", "วันนี้แฟนของคุณรู้สึกอย่างไรบ้างนะ?", 16744619, fields, "")
		for _, tid := range m.VisibleTo {
			services.TriggerPushNotification(tid, "🌈 แฟนอัปเดตอารมณ์แล้ว", "ตอนนี้รู้สึก: "+m.MoodEmoji)
		}
	}()
	w.WriteHeader(http.StatusCreated)
}

func HandleGetMoods(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var results []map[string]interface{}
	client.From("daily_moods").Select("*", "exact", false).Order("created_at", &postgrest.OrderOpts{Ascending: false}).Limit(20, "").ExecuteTo(&results)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func HandleDeleteMood(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	id := r.URL.Query().Get("id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("daily_moods").Delete("", "").Eq("id", id).Execute()
	w.WriteHeader(http.StatusOK)
}

// --- Wishlist Handlers ---

func HandleSaveWishlist(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var item struct {
		UserID      string   `json:"user_id"`
		ItemName    string   `json:"item_name"`
		Description string   `json:"item_description"`
		ItemURL     string   `json:"item_url"`
		VisibleTo   []string `json:"visible_to"`
	}
	json.NewDecoder(r.Body).Decode(&item)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("wishlists").Insert(item, false, "", "", "").Execute()

	go func() {
		fields := []map[string]interface{}{{"name": "🎁 สิ่งของ", "value": item.ItemName, "inline": true}, {"name": "รายละเอียด", "value": item.Description, "inline": false}}
		if item.ItemURL != "" {
			fields = append(fields, map[string]interface{}{"name": "🔗 ลิงก์สินค้า", "value": item.ItemURL, "inline": false})
		}
		services.SendDiscordEmbed("🎁 แฟนลงของที่อยากได้ใหม่!", "ไปแอบดูหน่อยว่าแฟนอยากได้อะไรน้า~", 16753920, fields, "")
		for _, tid := range item.VisibleTo {
			services.TriggerPushNotification(tid, "🎁 แฟนลงของที่อยากได้ใหม่!", "อยากได้: "+item.ItemName)
		}
	}()
	w.WriteHeader(http.StatusCreated)
}

func HandleGetWishlist(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var results []map[string]interface{}
	client.From("wishlists").Select("*", "exact", false).Order("created_at", &postgrest.OrderOpts{Ascending: false}).ExecuteTo(&results)
	json.NewEncoder(w).Encode(results)
}

func HandleCompleteWish(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	id := r.URL.Query().Get("id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("wishlists").Update(map[string]interface{}{"is_received": true}, "", "").Eq("id", id).Execute()
	w.WriteHeader(http.StatusOK)
}

func HandleDeleteWishlist(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	id := r.URL.Query().Get("id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("wishlists").Delete("", "").Eq("id", id).Execute()
	w.WriteHeader(http.StatusOK)
}

// --- Moment Handlers ---

func HandleSaveMoment(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
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
		for _, tid := range m.VisibleTo {
			services.TriggerPushNotification(tid, "📸 Moment ใหม่!", "แฟนของคุณเพิ่งลงรูปภาพประจำวันล่ะ! ✨")
		}
		services.SendDiscordEmbed("📸 New Moment!", "อัปโหลดรูปภาพประจำวันแล้ว", 3447003, nil, m.ImageURL)
	}()
	w.WriteHeader(http.StatusCreated)
}

func HandleGetMoments(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var results []map[string]interface{}
	client.From("moments").Select("*", "exact", false).Order("created_at", &postgrest.OrderOpts{Ascending: false}).Limit(30, "").ExecuteTo(&results)
	json.NewEncoder(w).Encode(results)
}

func HandleDeleteMoment(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	id := r.URL.Query().Get("id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("moments").Delete("", "").Eq("id", id).Execute()
	w.WriteHeader(http.StatusOK)
}

// --- Event Handlers ---

func HandleCreateEvent(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var ev models.Event
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

	go func() {
		fields := []map[string]interface{}{{"name": "📅 วันที่", "value": ev.EventDate[:10], "inline": true}, {"name": "📌 ประเภท", "value": ev.CategoryType, "inline": true}}
		services.SendDiscordEmbed("💖 เพิ่มวันสำคัญใหม่แล้ว!", "หัวข้อ: "+ev.Title, 16738740, fields, "")
		for _, uid := range ev.VisibleTo {
			services.TriggerPushNotification(uid, "💖 มีวันพิเศษใหม่!", "อย่าลืมนะ: "+ev.Title)
		}
	}()
	w.WriteHeader(http.StatusCreated)
}

func HandleDeleteEvent(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	id := r.URL.Query().Get("id")
	title := r.URL.Query().Get("title")
	uID := r.URL.Query().Get("user_id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var results []map[string]interface{}
	client.From("events").Select("visible_to", "exact", false).Eq("id", id).ExecuteTo(&results)
	client.From("events").Delete("", "").Eq("id", id).Execute()

	go func() {
		services.SendDiscordEmbed("🗑️ ลบวันพิเศษ", "ลบหัวข้อ: "+title, 15158332, nil, "")
		if len(results) > 0 {
			if v, ok := results[0]["visible_to"].([]interface{}); ok {
				for _, uid := range v {
					if uid.(string) != uID {
						services.TriggerPushNotification(uid.(string), "🗑️ นัดหมายถูกยกเลิก", "นัดหมาย '"+title+"' ถูกลบออกแล้ว")
					}
				}
			}
		}
	}()
	w.WriteHeader(http.StatusOK)
}

func HandleGetMyEvents(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
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

func HandleGetHighlights(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	uID := r.URL.Query().Get("user_id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var data []map[string]interface{}
	client.From("events").Select("*", "exact", false).Eq("is_special", "true").Filter("visible_to", "cs", "{"+uID+"}").Order("event_date", &postgrest.OrderOpts{Ascending: true}).ExecuteTo(&data)
	json.NewEncoder(w).Encode(data)
}

// --- Request Handlers ---

func HandleCreateRequest(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var req models.RequestBody
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

	row := map[string]interface{}{"category": req.Header, "title": req.Title, "description": req.Duration, "sender_id": req.SenderID, "receiver_id": rID, "status": "pending", "sender_name": "Someone", "receiver_name": rName, "remark": fmt.Sprintf("%s|%s", req.TimeStart, req.TimeEnd), "image_url": req.ImageURL}
	client.From("requests").Insert(row, false, "", "", "").Execute()

	go func() {
		fields := []map[string]interface{}{{"name": "👤 ถึงคุณ", "value": rName, "inline": true}, {"name": "📝 หัวข้อ", "value": req.Title, "inline": true}, {"name": "⏰ เวลา", "value": utils.FormatDisplayTime(req.TimeStart), "inline": false}}
		services.SendDiscordEmbed("💌 มีคำขอใหม่ส่งถึงคุณ!", "หมวดหมู่: "+req.Header, 16753920, fields, req.ImageURL)
		services.TriggerPushNotification(rID, "📢 มีคำขอใหม่!", "แฟนส่งคำขอ '"+req.Header+"' มาให้จ้า ❤️")
	}()
	w.WriteHeader(http.StatusCreated)
}

func HandleGetMyRequests(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	uID := r.URL.Query().Get("user_id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var data []map[string]interface{}
	client.From("requests").Select("*", "exact", false).Or(fmt.Sprintf("sender_id.eq.%s,receiver_id.eq.%s", uID, uID), "").Order("created_at", &postgrest.OrderOpts{Ascending: false}).ExecuteTo(&data)
	json.NewEncoder(w).Encode(data)
}

func HandleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
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
			services.SendDiscordEmbed(statusTitle, "มีอัปเดตสถานะคำขอของคุณ", color, fields, "")
			services.TriggerPushNotification(item["sender_id"].(string), statusTitle, "แฟนพิจารณาคำขอ '"+fmt.Sprintf("%v", item["category"])+"' แล้วจ้า")
		}()
	}
	w.WriteHeader(http.StatusOK)
}

// --- Subscription Handlers ---

func SaveSubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var sub models.PushSubscription
	json.NewDecoder(r.Body).Decode(&sub)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("push_subscriptions").Delete("", "").Eq("user_id", sub.UserID).Execute()
	client.From("push_subscriptions").Insert(map[string]interface{}{"user_id": sub.UserID, "subscription_json": sub.Subscription}, false, "", "", "").Execute()
	w.WriteHeader(http.StatusOK)
}

func HandleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var body struct {
		UserID string `json:"user_id"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("push_subscriptions").Delete("", "").Eq("user_id", body.UserID).Execute()
	w.WriteHeader(http.StatusOK)
}

func HandleCheckSubscription(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	uID := r.URL.Query().Get("user_id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var results []map[string]interface{}
	client.From("push_subscriptions").Select("id", "exact", false).Eq("user_id", uID).ExecuteTo(&results)
	json.NewEncoder(w).Encode(map[string]bool{"subscribed": len(results) > 0})
}

// --- Background Task Functions ---

// func StartSpecialDayReminder() {
// 	go func() {
// 		for {
// 			now := time.Now()
// 			target := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, now.Location())
// 			if now.After(target) {
// 				target = target.Add(24 * time.Hour)
// 			}
// 			time.Sleep(time.Until(target))

// 			client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
// 			today := time.Now().Format("2006-01-02")
// 			var results []map[string]interface{}
// 			client.From("events").Select("*", "exact", false).Eq("category_type", "special").Like("event_date", today+"%").ExecuteTo(&results)

// 			for _, ev := range results {
// 				if v, ok := ev["visible_to"].([]interface{}); ok {
// 					for _, uid := range v {
// 						go services.TriggerPushNotification(uid.(string), "💖 Happy Special Day!", ev["title"].(string))
// 					}
// 				}
// 			}
// 		}
// 	}()
// }

// ✅ ก๊อปปี้มาจาก checkAndNotify ใน main.go เดิม
func CheckAndNotify() {
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	// ใช้เวลาไทย (Local) ในการเช็ค
	now := time.Now().Format("2006-01-02T15:04:00.000Z")

	var results []map[string]interface{}
	// ดึงเฉพาะ event ที่ตรงกับเวลานี้เป๊ะๆ
	client.From("events").Select("*", "exact", false).Eq("event_date", now).ExecuteTo(&results)

	if len(results) > 0 {
		for _, ev := range results {
			title := ev["title"].(string)
			// ส่งไป Discord
			services.SendDiscordEmbed("🔔 แจ้งเตือนวันสำคัญ!", title, 16761035, nil, "")

			// ส่ง Push Notification
			if visibleTo, ok := ev["visible_to"].([]interface{}); ok {
				for _, uid := range visibleTo {
					go services.TriggerPushNotification(uid.(string), "🔔 ถึงเวลาแล้วนะ!", title)
				}
			}
		}
	}
}

// --- Heart Game Handlers (อะไรอยู่ในใจฉ้านนน) ---

// 1. ฟังก์ชันสร้างเกม/ตั้งโจทย์
func HandleCreateHeartGame(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var g models.HeartGame
	if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
		http.Error(w, "Invalid Body", 400)
		return
	}

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
		msg := "มีคำทายรออยู่ในใจเค้า... พร้อมไหม? ❤️"
		if g.UseBot {
			msg = "เค้าส่งบอท Gemini มาท้าทายเธอ! พร้อมทายหรือยัง? 🤖"
		}
		services.TriggerPushNotification(g.GuesserID, "🎮 เกมอะไรอยู่ในใจฉ้านนน", msg)
		services.SendDiscordEmbed("🎮 เริ่มเกมใหม่!", "มีโจทย์ใหม่รอให้ทายแล้วจ้า", 16738740, nil, "")
	}()

	json.NewEncoder(w).Encode(results[0])
}

// 2. ฟังก์ชันเริ่มเกม (แฟนกดรับคำท้า)
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

// ใน handlers.go
func HandleAskQuestion(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}

	var msg struct {
		GameID   string `json:"game_id"`
		SenderID string `json:"sender_id"`
		Message  string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	var session []map[string]interface{}
	// ✅ ดึงข้อมูล Session พร้อม Join โจทย์หลัก
	client.From("game_sessions").
		Select("*, heart_games(id, secret_word, description, host_id)", "", false).
		Eq("id", msg.GameID).
		ExecuteTo(&session)

	if len(session) > 0 {
		mode := session[0]["mode"].(string)
		heartGame := session[0]["heart_games"].(map[string]interface{})
		secretWord := heartGame["secret_word"].(string)
		levelID := heartGame["id"].(string)

		description := ""
		if heartGame["description"] != nil {
			description = heartGame["description"].(string)
		}

		// บันทึกคำถามก่อน
		var savedMsg []map[string]interface{}
		client.From("game_messages").Insert(map[string]interface{}{
			"game_id":   levelID,
			"sender_id": msg.SenderID,
			"message":   msg.Message,
		}, false, "", "", "").ExecuteTo(&savedMsg)

		if len(savedMsg) > 0 {
			msgID := savedMsg[0]["id"].(string)

			if mode == "bot" {
				// ✅ ส่งคำอธิบายไปเทรน AI
				botAnswer := services.AskGemini(secretWord, description, msg.Message)
				client.From("game_messages").Update(map[string]interface{}{"answer": botAnswer}, "", "").Eq("id", msgID).Execute()

				if botAnswer == "ถูกต้อง" {
					client.From("heart_games").Update(map[string]interface{}{"status": "finished"}, "", "").Eq("id", levelID).Execute()
				}
			} else {
				hostID := heartGame["host_id"].(string)
				go services.TriggerPushNotification(hostID, "🎮 แฟนถามมาแล้ว!", "รีบไปตอบเร็ว! ❤️")
			}
		}
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// 4. ฟังก์ชันสำหรับคนตั้งโจทย์กดตอบ (ใช่ / ไม่ใช่ / ถูกต้อง)
func HandleAnswerQuestion(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var body struct {
		MessageID string `json:"message_id"` // ID ของคำถามที่แฟนถามมา
		Answer    string `json:"answer"`     // "ใช่", "ไม่ใช่", "ถูกต้อง"
		GameID    string `json:"game_id"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	// อัปเดตคำตอบลงในข้อความนั้นๆ
	client.From("game_messages").Update(map[string]interface{}{
		"answer": body.Answer,
	}, "", "").Eq("id", body.MessageID).Execute()

	// ถ้าตอบว่า "ถูกต้อง" ให้จบเกมและบันทึกเวลาเลิก
	if body.Answer == "ถูกต้อง" {
		now := time.Now()
		client.From("heart_games").Update(map[string]interface{}{
			"status":   "finished",
			"end_time": now,
		}, "", "").Eq("id", body.GameID).Execute()

		// ส่งแจ้งเตือนฉลองชัยชนะ!
		go services.TriggerPushNotification("", "🎉 เย้! ทายถูกแล้ว", "เก่งที่สุด! คำตอบคือสิ่งที่อยู่ในใจเค้าจริงๆ ด้วย")
	}

	w.WriteHeader(http.StatusOK)
}

// 1. ดึงรายการโจทย์ทั้งหมดที่ยังไม่หมดอายุ (30 วัน)
func HandleGetGameLevels(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	thirtyDaysAgo := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	var results []map[string]interface{}

	// ดึงโจทย์ที่ไม่ใช่ของตัวเอง (หรือดึงทั้งหมดแล้วไปเช็คที่หน้าจอ)
	client.From("heart_games").Select("id, host_id, created_at", "", false).
		Gte("created_at", thirtyDaysAgo).
		Order("created_at", &postgrest.OrderOpts{Ascending: false}).
		ExecuteTo(&results)

	json.NewEncoder(w).Encode(results)
}

// 1. ดึงรายการด่านทั้งหมด (Lobby)
func HandleGetLevels(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	var levels []map[string]interface{}
	// ดึงโจทย์ย้อนหลัง 30 วัน
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	client.From("heart_games").Select("*, users(username)", "", false).Gte("created_at", thirtyDaysAgo).Order("created_at", &postgrest.OrderOpts{Ascending: false}).ExecuteTo(&levels)

	json.NewEncoder(w).Encode(levels)
}

// 2. ดึงคำเชิญที่ค้างอยู่ (สำหรับจุดแดงบน Navbar)
// handlers.go
func HandleGetPendingInvitations(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	uID := r.URL.Query().Get("user_id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	var results []map[string]interface{}
	// ดึงข้อมูลเกมและชื่อคนท้า (Host) มาโชว์
	client.From("game_invitations").Select("*, sessions:session_id(*), host:host_id(username)", "", false).Eq("guesser_id", uID).Eq("status", "pending").ExecuteTo(&results)

	json.NewEncoder(w).Encode(results)
}

func HandleInvitePlayer(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var body struct {
		GameID    string `json:"game_id"`
		GuesserID string `json:"guesser_id"` // คนที่จะเล่น (แฟน)
		HostID    string `json:"host_id"`    // เจ้าของโจทย์
	}
	json.NewDecoder(r.Body).Decode(&body)

	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	// 1. สร้าง Session การเล่นใหม่
	var session []map[string]interface{}
	client.From("game_sessions").Insert(map[string]interface{}{
		"game_id":    body.GameID,
		"guesser_id": body.GuesserID,
		"mode":       "human",
		"status":     "pending",
	}, false, "", "", "").ExecuteTo(&session)

	// 2. สร้างคำเชิญเพื่อให้จุดแดงเด้งที่ Navbar แฟน
	if len(session) > 0 {
		client.From("game_invitations").Insert(map[string]interface{}{
			"session_id": session[0]["id"],
			"host_id":    body.HostID,
			"guesser_id": body.GuesserID,
			"status":     "pending",
		}, false, "", "", "").Execute()

		// 3. ส่งแจ้งเตือน PWA/Discord
		go services.TriggerPushNotification(body.GuesserID, "🎮 มีคำท้าทายใหม่!", "แฟนของคุณท้าให้ทายคำในใจแล้ว รีบไปรับคำท้าที่ Navbar นะ!")
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "ส่งคำเชิญสำเร็จ"})
}

// ฟังก์ชันสำหรับสร้าง Session การเล่นใหม่ (เมื่อกดเริ่มเกม)
func HandleCreateGame(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var body struct {
		GameID    string `json:"game_id"`
		GuesserID string `json:"guesser_id"`
		UseBot    bool   `json:"use_bot"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	// สร้าง session การเล่น
	mode := "human"
	if body.UseBot {
		mode = "bot"
	}

	var session []map[string]interface{}
	client.From("game_sessions").Insert(map[string]interface{}{
		"game_id":    body.GameID,
		"guesser_id": body.GuesserID,
		"mode":       mode,
		"status":     "playing", // ถ้าเป็นบอทเริ่มเล่นได้ทันที
	}, false, "", "", "").ExecuteTo(&session)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(session[0])
}
