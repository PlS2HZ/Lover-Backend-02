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
		var user []map[string]interface{}
		client.From("users").Select("username", "exact", false).Eq("id", m.UserID).ExecuteTo(&user)
		username := "แฟนของคุณ"
		if len(user) > 0 {
			username = user[0]["username"].(string)
		}

		msg := fmt.Sprintf("**%s** บันทึก Moment ใหม่: %s", username, m.Caption)
		services.SendDiscordEmbed("New Moment! 📸", msg, 3447003, nil, m.ImageURL)

		for _, targetID := range m.VisibleTo {
			if targetID != m.UserID {
				services.TriggerPushNotification(targetID, "📸 แฟนลงรูปใหม่ล่ะ!", m.Caption)
			}
		}
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

	go services.SendDiscordEmbed("Moment Deleted", "ลบรูปภาพ Moment ออกไปแล้ว", 16729149, nil, "")
	w.WriteHeader(http.StatusOK)
}
