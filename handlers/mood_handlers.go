package handlers

import (
	"couple-app/services"
	"couple-app/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/supabase-community/postgrest-go"
	"github.com/supabase-community/supabase-go"
)

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

	// บันทึกลง DB
	client.From("daily_moods").Insert(m, false, "", "", "").Execute()

	// ✅ แจ้งเตือน Discord
	go func() {
		var user []map[string]interface{}
		client.From("users").Select("username", "exact", false).Eq("id", m.UserID).ExecuteTo(&user)
		username := "แฟนของคุณ"
		if len(user) > 0 {
			username = user[0]["username"].(string)
		}

		msg := fmt.Sprintf("**%s** บอกความรู้สึก: %s\n> %s", username, m.MoodEmoji, m.MoodText)
		services.SendDiscordEmbed("อัปเดตอารมณ์ความรู้สึก 💖", msg, 16738740, nil, "")

		for _, targetID := range m.VisibleTo {
			if targetID != m.UserID {
				services.TriggerPushNotification(targetID, "💖 แฟนส่งความรู้สึกมานะ", m.MoodEmoji+" "+m.MoodText)
			}
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
	json.NewEncoder(w).Encode(results)
}

func HandleDeleteMood(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	id := r.URL.Query().Get("id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("daily_moods").Delete("", "").Eq("id", id).Execute()

	go services.SendDiscordEmbed("Mood Deleted", "ลบความทรงจำความรู้สึกออกไปแล้ว 🗑️", 16729149, nil, "")
	w.WriteHeader(http.StatusOK)
}
