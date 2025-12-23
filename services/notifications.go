package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/supabase-community/supabase-go"
)

// ก๊อปมาจาก triggerPushNotification เดิม
func TriggerPushNotification(userID string, title string, message string) {
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	var results []map[string]interface{}

	client.From("push_subscriptions").Select("subscription_json", "exact", false).Eq("user_id", userID).ExecuteTo(&results)

	for _, res := range results {
		subStr, ok := res["subscription_json"].(string)
		if !ok {
			b, _ := json.Marshal(res["subscription_json"])
			subStr = string(b)
		}

		s := &webpush.Subscription{}
		json.Unmarshal([]byte(subStr), s)

		resp, err := webpush.SendNotification([]byte(fmt.Sprintf(`{"title":"%s", "body":"%s", "url":"/"}`, title, message)), s, &webpush.Options{
			Subscriber:      os.Getenv("VAPID_EMAIL"),
			VAPIDPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
			VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
			TTL:             30,
		})
		if err == nil {
			resp.Body.Close()
		}
	}
}

// ก๊อปมาจาก sendDiscordEmbed เดิม
// services/notifications.go

func SendDiscordEmbed(title, description string, color int, fields []map[string]interface{}, imageURL string) {
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if webhookURL == "" {
		return
	}

	embed := map[string]interface{}{
		"title":       "💖 " + title,
		"description": description,
		"color":       color,
		"fields":      fields,
		"footer": map[string]string{
			"text": "Lover App • " + time.Now().Format("02 Jan 15:04"),
		},
	}

	if imageURL != "" && imageURL != "null" {
		embed["image"] = map[string]string{"url": imageURL}
	}

	payload := map[string]interface{}{
		"content": "@everyone", // แจ้งเตือนทุกคนในห้อง
		"embeds":  []interface{}{embed},
	}

	jsonData, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
}

// ก๊อปมาจาก checkAndNotify เดิม (เอาไว้รันใน background)
func CheckAndNotify() {
	client, _ := supabase.NewClient(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), nil)
	now := time.Now().UTC().Truncate(time.Minute).Format("2006-01-02T15:04:00.000Z")

	var results []map[string]interface{}
	client.From("events").Select("*", "exact", false).Eq("event_date", now).ExecuteTo(&results)

	if len(results) > 0 {
		for _, ev := range results {
			title := ev["title"].(string)
			SendDiscordEmbed("🔔 แจ้งเตือนวันสำคัญ!", title, 16761035, nil, "")
			if visibleTo, ok := ev["visible_to"].([]interface{}); ok {
				for _, uid := range visibleTo {
					go TriggerPushNotification(uid.(string), "🔔 ถึงเวลาแล้วนะ!", title)
				}
			}
		}
	}
}
