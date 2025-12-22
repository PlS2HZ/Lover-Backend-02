package models

import "time"

// รวม Struct ทั้งหมดจาก main.go เดิม
type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	AvatarURL   string `json:"avatar_url"`
	Description string `json:"description"`
	Gender      string `json:"gender"`
}

type RequestBody struct {
	ID               string `json:"id,omitempty"`
	Header           string `json:"header"`
	Title            string `json:"title"`
	Duration         string `json:"duration"`
	SenderID         string `json:"sender_id"`
	ReceiverUsername string `json:"receiver_username"`
	TimeStart        string `json:"time_start"`
	TimeEnd          string `json:"time_end"`
	ImageURL         string `json:"image_url"`
}

type Event struct {
	ID           string   `json:"id,omitempty"`
	EventDate    string   `json:"event_date"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	CreatedBy    string   `json:"created_by"`
	VisibleTo    []string `json:"visible_to"`
	RepeatType   string   `json:"repeat_type"`
	IsSpecial    bool     `json:"is_special"`
	CategoryType string   `json:"category_type"`
}

type PushSubscription struct {
	UserID       string      `json:"user_id"`
	Subscription interface{} `json:"subscription"`
}

type DailyMood struct {
	UserID    string `json:"user_id"`
	MoodEmoji string `json:"mood_emoji"`
	MoodText  string `json:"mood_text"`
}

type WishlistItem struct {
	ID          string `json:"id,omitempty"`
	UserID      string `json:"user_id"`
	ItemName    string `json:"item_name"`
	Description string `json:"item_description"`
	ItemURL     string `json:"item_url"`
	IsReceived  bool   `json:"is_received"`
}

type Moment struct {
	ID       string `json:"id,omitempty"`
	UserID   string `json:"user_id"`
	ImageURL string `json:"image_url"`
	Caption  string `json:"caption"`
}

type HomeConfig struct {
	ID         string `json:"id,omitempty"`
	ConfigType string `json:"config_type"`
	Data       string `json:"data"`
}

// โครงสร้างเกมหลัก
type HeartGame struct {
	ID         string     `json:"id,omitempty"`
	HostID     string     `json:"host_id"`
	GuesserID  string     `json:"guesser_id"`
	SecretWord string     `json:"secret_word"`
	Status     string     `json:"status"` // waiting, playing, finished
	StartTime  *time.Time `json:"start_time,omitempty"`
	EndTime    *time.Time `json:"end_time,omitempty"`
	UseBot     bool       `json:"use_bot"`
	CreatedAt  time.Time  `json:"created_at"`
}

// โครงสร้างข้อความถาม-ตอบ
type GameMessage struct {
	ID        string    `json:"id,omitempty"`
	GameID    string    `json:"game_id"`
	SenderID  string    `json:"sender_id"`
	Message   string    `json:"message"`
	Answer    string    `json:"answer"` // "ใช่", "ไม่ใช่", "ถูกต้อง"
	CreatedAt time.Time `json:"created_at"`
}
