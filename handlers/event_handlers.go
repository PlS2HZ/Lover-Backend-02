package handlers

import (
	"couple-app/services"
	"couple-app/utils"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"couple-app/models"

	"github.com/supabase-community/postgrest-go"
	"github.com/supabase-community/supabase-go"
)

// --- Event & Calendar ---
func HandleCreateEvent(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var ev models.Event
	json.NewDecoder(r.Body).Decode(&ev)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	row := map[string]interface{}{"event_date": ev.EventDate, "title": ev.Title, "is_special": true, "category_type": ev.CategoryType, "visible_to": ev.VisibleTo}
	client.From("events").Insert(row, false, "", "", "").Execute()
	w.WriteHeader(http.StatusCreated)
}

func HandleGetMyEvents(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	uID := r.URL.Query().Get("user_id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var data []map[string]interface{}
	client.From("events").Select("*", "exact", false).Filter("visible_to", "cs", "{"+uID+"}").Order("event_date", &postgrest.OrderOpts{Ascending: true}).ExecuteTo(&data)
	json.NewEncoder(w).Encode(data)
}

func HandleDeleteEvent(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	id := r.URL.Query().Get("id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("events").Delete("", "").Eq("id", id).Execute()
	w.WriteHeader(http.StatusOK)
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
