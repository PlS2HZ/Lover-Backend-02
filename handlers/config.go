package handlers

import (
	"couple-app/models"
	"couple-app/utils"
	"encoding/json"
	"net/http"
	"os"

	"github.com/supabase-community/supabase-go"
)

// ✅ ต้องมี jwtKey ประกาศที่นี่ตัวเดียวพอ เพื่อให้ auth_handlers เรียกใช้ได้
var jwtKey = []byte("your_secret_key_2025")

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
