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
		var user []map[string]interface{}
		client.From("users").Select("username", "exact", false).Eq("id", item.UserID).ExecuteTo(&user)
		username := "แฟนของคุณ"
		if len(user) > 0 {
			username = user[0]["username"].(string)
		}

		msg := fmt.Sprintf("**%s** เพิ่มของที่อยากได้: %s\nรายละเอียด: %s", username, item.ItemName, item.Description)
		if item.ItemURL != "" {
			msg += "\n🔗 ลิงก์: " + item.ItemURL
		}

		services.SendDiscordEmbed("Wishlist Added! ✨", msg, 16753920, nil, "")
		for _, targetID := range item.VisibleTo {
			if targetID != item.UserID {
				services.TriggerPushNotification(targetID, "✨ แฟนอยากได้ของแหละ", item.ItemName)
			}
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

	go services.SendDiscordEmbed("Wish Completed! 🎉", "เย้! รายการ Wishlist สำเร็จแล้วหนึ่งอย่าง", 5763719, nil, "")
	w.WriteHeader(http.StatusOK)
}

func HandleDeleteWishlist(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	id := r.URL.Query().Get("id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	client.From("wishlists").Delete("", "").Eq("id", id).Execute()

	go services.SendDiscordEmbed("Wish Deleted", "ลบรายการ Wishlist ออกแล้ว", 16729149, nil, "")
	w.WriteHeader(http.StatusOK)
}
