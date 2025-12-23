package handlers

import (
	"couple-app/services"
	"couple-app/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"couple-app/models"

	"github.com/supabase-community/postgrest-go"
	"github.com/supabase-community/supabase-go"
)

// --- Event & Calendar ---
// handlers/event_handlers.go

func HandleCreateEvent(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var ev models.Event
	json.NewDecoder(r.Body).Decode(&ev)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	// ✅ ต้องบันทึก CreatedBy และ VisibleTo ลงไปด้วย ข้อมูลถึงจะโชว์ในหน้าเว็บ
	row := map[string]interface{}{
		"event_date": ev.EventDate, "title": ev.Title, "description": ev.Description,
		"created_by": ev.CreatedBy, "visible_to": ev.VisibleTo,
		"repeat_type": ev.RepeatType, "category_type": ev.CategoryType,
		"is_special": ev.CategoryType == "special",
	}
	client.From("events").Insert(row, false, "", "", "").Execute()

	// แจ้งเตือน Discord/PWA
	go func() {
		msg := fmt.Sprintf("📅 **นัดหมายใหม่:** %s\n🗓️ **วันที่:** %s", ev.Title, ev.EventDate)
		services.SendDiscordEmbed("Calendar Added!", msg, 3447003, nil, "")
		for _, uid := range ev.VisibleTo {
			services.TriggerPushNotification(uid, "📅 นัดหมายใหม่!", ev.Title)
		}
	}()
	w.WriteHeader(http.StatusCreated)
}

func HandleDeleteEvent(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	id := r.URL.Query().Get("id")
	title := r.URL.Query().Get("title") // ✅ รับชื่อมาโชว์ใน Discord

	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	// ลบข้อมูลจากฐานข้อมูล
	client.From("events").Delete("", "").Eq("id", id).Execute()

	// ✅ ใส่แบบนี้ถูกต้องแล้วครับ ระบบจะส่งแจ้งเตือนโดยไม่รอให้การลบเสร็จ (รันเบื้องหลัง)
	// 16729149 คือรหัสสีแดงสำหรับ Discord
	go services.SendDiscordEmbed("Calendar Deleted", fmt.Sprintf("ลบนัดหมาย **'%s'** ออกจากปฏิทินแล้ว 🗑️", title), 16729149, nil, "")

	w.WriteHeader(http.StatusOK)
}

func HandleGetMyEvents(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	uID := r.URL.Query().Get("user_id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var data []map[string]interface{}

	// ✅ แก้ไข: ให้ดึงข้อมูลที่ "เราเป็นคนสร้าง" (created_by) หรือ "มีชื่อเราในคนมองเห็น" (visible_to)
	// ใช้ Or เพื่อความชัวร์ 100% ว่าเจ้าของต้องเห็นงานตัวเอง
	query := fmt.Sprintf("created_by.eq.%s,visible_to.cs.{%s}", uID, uID)
	client.From("events").Select("*", "exact", false).Or(query, "").Order("event_date", &postgrest.OrderOpts{Ascending: true}).ExecuteTo(&data)

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

// --- Notification Subscriptions ---
func SaveSubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var sub struct {
		UserID       string `json:"user_id"`
		Subscription string `json:"subscription"`
	}
	json.NewDecoder(r.Body).Decode(&sub)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
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

// ✅ ก๊อปปี้มาจากเดิม เพื่อให้ main.go เรียกใช้งานได้
func CheckAndNotify() {
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	now := time.Now().Format("2006-01-02T15:04:00.000Z")
	var results []map[string]interface{}
	client.From("events").Select("*", "exact", false).Eq("event_date", now).ExecuteTo(&results)
	if len(results) > 0 {
		for _, ev := range results {
			title := ev["title"].(string)
			services.SendDiscordEmbed("🔔 แจ้งเตือน!", title, 16761035, nil, "")
		}
	}
}
