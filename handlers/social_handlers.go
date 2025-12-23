package handlers

import (
	"couple-app/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/supabase-community/postgrest-go"
	"github.com/supabase-community/supabase-go"
)

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

// --- Request Handlers ---
func HandleCreateRequest(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var req struct {
		SenderID         string `json:"sender_id"`
		ReceiverUsername string `json:"receiver_username"`
		Header           string `json:"header"`
		Title            string `json:"title"`
		Duration         string `json:"duration"`
		TimeStart        string `json:"time_start"`
		TimeEnd          string `json:"time_end"`
		ImageURL         string `json:"image_url"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var users []map[string]interface{}
	client.From("users").Select("id, username", "exact", false).Eq("username", req.ReceiverUsername).ExecuteTo(&users)
	if len(users) == 0 {
		http.Error(w, "Not Found", 404)
		return
	}
	rID := users[0]["id"].(string)
	row := map[string]interface{}{"category": req.Header, "title": req.Title, "description": req.Duration, "sender_id": req.SenderID, "receiver_id": rID, "status": "pending"}
	client.From("requests").Insert(row, false, "", "", "").Execute()
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
	w.WriteHeader(http.StatusOK)
}
