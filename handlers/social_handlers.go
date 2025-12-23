package handlers

import (
	"couple-app/services"
	"couple-app/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/supabase-community/postgrest-go"
	"github.com/supabase-community/supabase-go"
)

// --- Request Handlers ---
func HandleCreateRequest(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var req struct {
		SenderID         string `json:"sender_id"`
		ReceiverUsername string `json:"receiver_username"` // ชื่อคนรับที่พิมพ์มา
		Header           string `json:"header"`
		Title            string `json:"title"`
		Duration         string `json:"duration"`
		ImageURL         string `json:"image_url"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	// ✅ 1. หาข้อมูลคนรับ (เพื่อเอา ID)
	var targetUser []map[string]interface{}
	client.From("users").Select("id", "exact", false).Eq("username", req.ReceiverUsername).ExecuteTo(&targetUser)
	if len(targetUser) == 0 {
		http.Error(w, "Receiver Not Found", 404)
		return
	}
	rID := targetUser[0]["id"].(string)

	// ✅ 2. หาชื่อคนส่ง (เพื่อบันทึกลง sender_name ตามกฎ NOT NULL)
	var senderUser []map[string]interface{}
	client.From("users").Select("username", "exact", false).Eq("id", req.SenderID).ExecuteTo(&senderUser)
	sName := "Unknown"
	if len(senderUser) > 0 {
		sName = senderUser[0]["username"].(string)
	}

	// ✅ 3. บันทึกข้อมูลให้ครบทุกฟิลด์ที่ DB ต้องการ
	row := map[string]interface{}{
		"category":      req.Header,
		"title":         req.Title,
		"description":   req.Duration,
		"sender_id":     req.SenderID,
		"sender_name":   sName, // ✅ ห้ามว่าง
		"receiver_id":   rID,
		"receiver_name": req.ReceiverUsername, // ✅ ห้ามว่าง
		"status":        "pending",
		"image_url":     req.ImageURL,
	}

	// ตรวจสอบ Error จากการ Insert
	_, _, err := client.From("requests").Insert(row, false, "", "", "").Execute()
	if err != nil {
		fmt.Println("DB Insert Error:", err)
		http.Error(w, "Database Error", 500)
		return
	}

	// แจ้งเตือน Discord
	go func() {
		msg := fmt.Sprintf("💌 มีคำขอใหม่: %s\nจาก: %s", req.Title, sName) // ใช้ชื่อแทน ID
		services.SendDiscordEmbed("💖 มีคำขอใหม่รอการอนุมัติ!", msg, 16738740, nil, req.ImageURL)
		services.TriggerPushNotification(rID, "💌 มีคำขอใหม่!", msg)
	}()

	w.WriteHeader(http.StatusCreated)
}

// social_handlers.go
func HandleGetMyRequests(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	uID := r.URL.Query().Get("user_id")
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var data []map[string]interface{}

	// ✅ แก้ไข: กรองข้อมูลโดยใช้ Or ให้ชัดเจน และดึงข้อมูลทั้งหมด
	// ดึงรายการที่ Sender เป็นเรา หรือ Receiver เป็นเรา
	query := fmt.Sprintf("sender_id.eq.%s,receiver_id.eq.%s", uID, uID)
	client.From("requests").Select("*", "exact", false).Or(query, "").Order("created_at", &postgrest.OrderOpts{Ascending: false}).ExecuteTo(&data)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func HandleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if utils.EnableCORS(&w, r) {
		return
	}
	var body struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Comment string `json:"comment"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)

	// ✅ ดึงข้อมูลเพื่อส่งแจ้งเตือนกลับหาคนส่ง
	var reqData []map[string]interface{}
	client.From("requests").Select("sender_id, title", "", false).Eq("id", body.ID).ExecuteTo(&reqData)

	client.From("requests").Update(map[string]interface{}{
		"status": body.Status, "comment": body.Comment, "processed_at": time.Now(),
	}, "", "").Eq("id", body.ID).Execute()

	// ✅ ส่งแจ้งเตือนเมื่อ อนุมัติ/ปฏิเสธ
	if len(reqData) > 0 {
		senderID := reqData[0]["sender_id"].(string)
		title := reqData[0]["title"].(string)
		statusTxt := "ได้รับอนุมัติแล้ว ✨"
		color := 5763719 // สีเขียว
		if body.Status == "rejected" {
			statusTxt = "ถูกปฏิเสธ ❌"
			color = 16729149 // สีแดง
		}

		go func() {
			msg := fmt.Sprintf("📢 คำขอ '%s' ของคุณ %s", title, statusTxt)
			services.SendDiscordEmbed("🔔 อัปเดตสถานะคำขอ", msg, color, nil, "")
			services.TriggerPushNotification(senderID, "📢 สถานะคำขอ", msg)
		}()
	}
	w.WriteHeader(http.StatusOK)
}
