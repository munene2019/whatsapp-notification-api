package routes

import (
	"net/http"

	"whatsapp-notification-api/handlers"
	"whatsapp-notification-api/utils"
)

func RegisterRoutes(
	handler *handlers.WhatsAppHandler,
) *http.ServeMux {

	mux := http.NewServeMux()

	// Immediate messaging endpoints.
	mux.HandleFunc(
		"/api/whatsapp/send",
		handler.Send,
	)

	mux.HandleFunc(
		"/api/whatsapp/bulk",
		handler.Bulk,
	)
	mux.HandleFunc(
		"/api/whatsapp/history",
		handler.GetMessageHistory,
	)
	mux.HandleFunc("/api/whatsapp/webhook", handler.Webhook)
	// Scheduling endpoints.
	mux.HandleFunc(
		"/api/whatsapp/schedule",
		handler.Schedule,
	)

	mux.HandleFunc(
		"/api/whatsapp/schedule/bulk",
		handler.ScheduleBulk,
	)

	mux.HandleFunc(
		"/api/whatsapp/scheduled",
		handler.GetScheduled,
	)

	mux.HandleFunc(
		"/api/whatsapp/scheduled/",
		handler.CancelScheduled,
	)

	// Health check.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			utils.ErrorResponse(
				w,
				http.StatusNotFound,
				"Endpoint not found",
			)
			return
		}

		utils.JSONResponse(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "WhatsApp Notification API is running",
		})
	})

	return mux
}
