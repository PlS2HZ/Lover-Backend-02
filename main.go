package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"couple-app/handlers" // ใช้ชื่อจาก go.mod

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	// รันระบบแจ้งเตือนอัตโนมัติใน Background
	// handlers.StartSpecialDayReminder()
	ticker := time.NewTicker(time.Minute)
	go func() {
		for range ticker.C {
			handlers.CheckAndNotify()
		}
	}()

	// --- Routes Mapping ---
	http.HandleFunc("/api/register", handlers.HandleRegister)
	http.HandleFunc("/api/login", handlers.HandleLogin)
	http.HandleFunc("/api/users", handlers.HandleGetAllUsers)
	http.HandleFunc("/api/users/update", handlers.HandleUpdateProfile)

	http.HandleFunc("/api/save-mood", handlers.HandleSaveMood)
	http.HandleFunc("/api/get-moods", handlers.HandleGetMoods)
	http.HandleFunc("/api/mood/delete", handlers.HandleDeleteMood)

	http.HandleFunc("/api/wishlist/save", handlers.HandleSaveWishlist)
	http.HandleFunc("/api/wishlist/get", handlers.HandleGetWishlist)
	http.HandleFunc("/api/wishlist/complete", handlers.HandleCompleteWish)
	http.HandleFunc("/api/wishlist/delete", handlers.HandleDeleteWishlist)

	http.HandleFunc("/api/moment/save", handlers.HandleSaveMoment)
	http.HandleFunc("/api/moment/get", handlers.HandleGetMoments)
	http.HandleFunc("/api/moment/delete", handlers.HandleDeleteMoment)

	http.HandleFunc("/api/request", handlers.HandleCreateRequest)
	http.HandleFunc("/api/my-requests", handlers.HandleGetMyRequests)
	http.HandleFunc("/api/update-status", handlers.HandleUpdateStatus)

	http.HandleFunc("/api/events", handlers.HandleGetMyEvents)
	http.HandleFunc("/api/events/create", handlers.HandleCreateEvent)
	http.HandleFunc("/api/events/delete", handlers.HandleDeleteEvent)
	http.HandleFunc("/api/highlights", handlers.HandleGetHighlights)

	http.HandleFunc("/api/save-subscription", handlers.SaveSubscriptionHandler)
	http.HandleFunc("/api/unsubscribe", handlers.HandleUnsubscribe)
	http.HandleFunc("/api/check-subscription", handlers.HandleCheckSubscription)

	http.HandleFunc("/api/home-config/get", handlers.HandleGetHomeConfig)
	http.HandleFunc("/api/home-config/update", handlers.HandleUpdateHomeConfig)

	http.HandleFunc("/api/game/create", handlers.HandleCreateHeartGame)
	http.HandleFunc("/api/game/start", handlers.HandleStartHeartGame)
	http.HandleFunc("/api/game/ask", handlers.HandleAskQuestion)

	http.HandleFunc("/api/game/answer", handlers.HandleAnswerQuestion)

	http.HandleFunc("/api/game/create", handlers.HandleCreateGame)

	http.HandleFunc("/api/game/invite", handlers.HandleInvitePlayer)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("🚀 Server live on %s", port)
	http.ListenAndServe(":"+port, nil)
}
