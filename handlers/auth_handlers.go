package handlers

import (
	"couple-app/utils"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"couple-app/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/supabase-community/supabase-go"
	"golang.org/x/crypto/bcrypt"
)

// HandleRegister จัดการการลงทะเบียนผู้ใช้ใหม่
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

// HandleLogin จัดการการเข้าสู่ระบบและสร้าง JWT
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

// HandleGetAllUsers ดึงรายชื่อผู้ใช้ทั้งหมด
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

// HandleUpdateProfile อัปเดตข้อมูลส่วนตัว
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
