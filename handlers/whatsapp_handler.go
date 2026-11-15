package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"whatsapp-notification-api/config"
	"whatsapp-notification-api/models"
	"whatsapp-notification-api/repositories"
	"whatsapp-notification-api/services"
)

type WhatsAppHandler struct {
	Service    *services.WhatsAppService
	Repository *repositories.ScheduledRepository
	Config     *config.Config
}

func NewWhatsAppHandler(
	service *services.WhatsAppService,
	repository *repositories.ScheduledRepository,
	cfg *config.Config,
) *WhatsAppHandler {
	return &WhatsAppHandler{
		Service:    service,
		Repository: repository,
		Config:     cfg,
	}
}

// Send handles a single WhatsApp message.
func (h *WhatsAppHandler) Send(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	var request models.SendRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid JSON request: " + err.Error(),
		})
		return
	}

	request.To = strings.TrimSpace(request.To)

	payload, err := json.Marshal(request)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Failed to encode WhatsApp payload",
		})
		return
	}

	historyID, err := h.Repository.CreateImmediateHistory(
		r.Context(),
		request.To,
		request.Template.Name,
		request.Template.Language.Code,
		payload,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Failed to save message history: " + err.Error(),
		})
		return
	}

	result, err := h.Service.SendMessage(r.Context(), request)

	if err != nil {
		updateErr := h.Repository.UpdateImmediateHistory(
			r.Context(),
			historyID,
			"failed",
			"",
			err.Error(),
		)

		if updateErr != nil {
			log.Printf("Failed to update history ID %d: %v", historyID, updateErr)
		}

		writeJSON(w, http.StatusBadGateway, map[string]interface{}{
			"success":    false,
			"error":      err.Error(),
			"history_id": historyID,
		})
		return
	}

	providerMessageID := getProviderMessageID(result)

	updateErr := h.Repository.UpdateImmediateHistory(
		r.Context(),
		historyID,
		"accepted",
		providerMessageID,
		"",
	)

	response := map[string]interface{}{
		"success":    true,
		"message":    "WhatsApp message submitted successfully",
		"history_id": historyID,
		"data":       result,
	}

	if updateErr != nil {
		log.Printf("Failed to update history ID %d: %v", historyID, updateErr)
		response["history_warning"] = "Message submitted, but history status update failed"
	}

	writeJSON(w, http.StatusOK, response)
}

// Bulk handles sending WhatsApp messages to multiple recipients.
func (h *WhatsAppHandler) Bulk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	var request models.BulkRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid JSON request: " + err.Error(),
		})
		return
	}

	if len(request.Recipients) == 0 || len(request.Recipients) > 500 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Provide between 1 and 500 recipients",
		})
		return
	}

	if strings.TrimSpace(request.TemplateName) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Template name is required",
		})
		return
	}

	if strings.TrimSpace(request.Language) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Template language is required",
		})
		return
	}

	results := make([]map[string]interface{}, 0, len(request.Recipients))
	successCount := 0
	failureCount := 0

	phonePattern := regexp.MustCompile(`^[1-9][0-9]{7,14}$`)

	for _, recipient := range request.Recipients {
		recipient = strings.TrimSpace(recipient)

		sendRequest := models.SendRequest{
			MessagingProduct: "whatsapp",
			To:               recipient,
			Type:             "template",
			Template: models.Template{
				Name: strings.TrimSpace(request.TemplateName),
				Language: models.Language{
					Code: strings.TrimSpace(request.Language),
				},
			},
		}

		payload, err := json.Marshal(sendRequest)
		if err != nil {
			failureCount++

			results = append(results, map[string]interface{}{
				"to":      recipient,
				"success": false,
				"error":   "Failed to encode message payload",
			})
			continue
		}

		historyID, err := h.Repository.CreateImmediateHistory(
			r.Context(),
			recipient,
			request.TemplateName,
			request.Language,
			payload,
		)

		if err != nil {
			failureCount++

			log.Printf("Failed to save history for %s: %v", recipient, err)

			results = append(results, map[string]interface{}{
				"to":      recipient,
				"success": false,
				"error":   "Failed to save message history: " + err.Error(),
			})
			continue
		}

		if !phonePattern.MatchString(recipient) {
			failureCount++

			errMessage := "Invalid phone number. Use international format without +"

			updateErr := h.Repository.UpdateImmediateHistory(
				r.Context(),
				historyID,
				"failed",
				"",
				errMessage,
			)

			if updateErr != nil {
				log.Printf("Failed to update history ID %d: %v", historyID, updateErr)
			}

			results = append(results, map[string]interface{}{
				"to":         recipient,
				"success":    false,
				"history_id": historyID,
				"error":      errMessage,
			})
			continue
		}

		result, err := h.Service.SendMessage(r.Context(), sendRequest)

		if err != nil {
			failureCount++

			updateErr := h.Repository.UpdateImmediateHistory(
				r.Context(),
				historyID,
				"failed",
				"",
				err.Error(),
			)

			if updateErr != nil {
				log.Printf("Failed to update history ID %d: %v", historyID, updateErr)
			}

			results = append(results, map[string]interface{}{
				"to":         recipient,
				"success":    false,
				"history_id": historyID,
				"error":      err.Error(),
			})
			continue
		}

		providerMessageID := getProviderMessageID(result)

		updateErr := h.Repository.UpdateImmediateHistory(
			r.Context(),
			historyID,
			"accepted",
			providerMessageID,
			"",
		)

		successCount++

		item := map[string]interface{}{
			"to":         recipient,
			"success":    true,
			"history_id": historyID,
			"data":       result,
		}

		if updateErr != nil {
			log.Printf("Failed to update history ID %d: %v", historyID, updateErr)
			item["history_warning"] = "Message submitted, but history status update failed"
		}

		results = append(results, item)
	}

	statusCode := http.StatusOK

	if failureCount > 0 {
		statusCode = http.StatusMultiStatus
	}

	writeJSON(w, statusCode, map[string]interface{}{
		"success":    failureCount == 0,
		"total":      len(request.Recipients),
		"successful": successCount,
		"failed":     failureCount,
		"results":    results,
	})
}

// Schedule handles scheduling one WhatsApp message.
func (h *WhatsAppHandler) Schedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	var request models.ScheduleRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid JSON request: " + err.Error(),
		})
		return
	}

	if err := validateSchedule(
		request.To,
		request.Template,
		request.ScheduledAt,
	); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	sendRequest := request.SendRequest
	sendRequest.To = strings.TrimSpace(sendRequest.To)

	if sendRequest.MessagingProduct == "" {
		sendRequest.MessagingProduct = "whatsapp"
	}

	if sendRequest.Type == "" {
		sendRequest.Type = "template"
	}

	payload, err := json.Marshal(sendRequest)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Failed to encode WhatsApp payload",
		})
		return
	}

	message := models.ScheduledMessage{
		Recipient:    sendRequest.To,
		TemplateName: request.Template.Name,
		LanguageCode: request.Template.Language.Code,
		Payload:      payload,
		ScheduledAt:  request.ScheduledAt.UTC(),
	}

	saved, err := h.Repository.Create(r.Context(), message)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"message": "WhatsApp message scheduled successfully",
		"data":    saved,
	})
}

// ScheduleBulk handles scheduling messages for multiple recipients.
func (h *WhatsAppHandler) ScheduleBulk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	var request models.BulkScheduleRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid JSON request: " + err.Error(),
		})
		return
	}

	if len(request.Recipients) == 0 || len(request.Recipients) > 500 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Provide between 1 and 500 recipients",
		})
		return
	}

	if strings.TrimSpace(request.Template.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Template name is required",
		})
		return
	}

	if strings.TrimSpace(request.Template.Language.Code) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Template language is required",
		})
		return
	}

	now := time.Now()

	if request.ScheduledAt.IsZero() || !request.ScheduledAt.After(now) {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "scheduled_at must be a future date and time",
		})
		return
	}

	log.Printf(
		"SCHEDULE DEBUG: received=%s, utc=%s, server_now=%s, seconds_until=%.2f",
		request.ScheduledAt.Format(time.RFC3339Nano),
		request.ScheduledAt.UTC().Format(time.RFC3339Nano),
		now.Format(time.RFC3339Nano),
		request.ScheduledAt.Sub(now).Seconds(),
	)

	phonePattern := regexp.MustCompile(`^[1-9][0-9]{7,14}$`)
	seen := make(map[string]bool)

	for i, recipient := range request.Recipients {
		recipient = strings.TrimSpace(recipient)
		request.Recipients[i] = recipient

		if !phonePattern.MatchString(recipient) {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{
				"success": false,
				"error":   "Invalid recipient phone number: " + recipient,
			})
			return
		}

		if seen[recipient] {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{
				"success": false,
				"error":   "Duplicate recipient: " + recipient,
			})
			return
		}

		seen[recipient] = true
	}

	messages := make([]models.ScheduledMessage, 0, len(request.Recipients))

	for _, recipient := range request.Recipients {
		sendRequest := models.SendRequest{
			MessagingProduct: "whatsapp",
			To:               recipient,
			Type:             "template",
			Template:         request.Template,
		}

		payload, err := json.Marshal(sendRequest)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"success": false,
				"error":   "Failed to encode WhatsApp payload",
			})
			return
		}

		messages = append(messages, models.ScheduledMessage{
			Recipient:    recipient,
			TemplateName: request.Template.Name,
			LanguageCode: request.Template.Language.Code,
			Payload:      payload,
			ScheduledAt:  request.ScheduledAt.UTC(),
		})
	}

	saved, err := h.Repository.CreateBulk(r.Context(), messages)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	for _, message := range saved {
		log.Printf(
			"SCHEDULE SAVED: id=%d, recipient=%s, scheduled_at=%s, status=%s",
			message.ID,
			message.Recipient,
			message.ScheduledAt.Format(time.RFC3339Nano),
			message.Status,
		)
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"message": "Bulk WhatsApp messages scheduled successfully",
		"total":   len(saved),
		"data":    saved,
	})
}

// GetScheduled retrieves scheduled messages.
func (h *WhatsAppHandler) GetScheduled(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	messages, err := h.Repository.GetAll(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"total":   len(messages),
		"data":    messages,
	})
}

// CancelScheduled cancels a pending scheduled message.
func (h *WhatsAppHandler) CancelScheduled(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	idText := strings.TrimPrefix(
		r.URL.Path,
		"/api/whatsapp/scheduled/",
	)

	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid scheduled message ID",
		})
		return
	}

	if err := h.Repository.Cancel(r.Context(), id); err != nil {
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Scheduled message cancelled successfully",
		"id":      id,
	})
}

// Webhook handles Meta webhook verification and delivery-status notifications.
func (h *WhatsAppHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	log.Println("========== WEBHOOK ROUTE HIT ==========")
	log.Printf("Method: %s", r.Method)
	log.Printf("Path: %s", r.URL.Path)
	log.Printf("Remote Address: %s", r.RemoteAddr)

	switch r.Method {
	case http.MethodGet:
		log.Println("Processing GET webhook verification")
		h.verifyWebhook(w, r)

	case http.MethodPost:
		log.Println("Processing POST webhook notification")
		h.receiveWebhook(w, r)

	default:
		log.Printf("Unsupported webhook method: %s", r.Method)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
	}
}

// verifyWebhook handles Meta's initial webhook subscription verification.
func (h *WhatsAppHandler) verifyWebhook(w http.ResponseWriter, r *http.Request) {
	log.Println("========== WEBHOOK VERIFICATION ==========")

	if h.Config == nil || h.Config.WebhookVerifyToken == "" {
		log.Println("ERROR: Webhook verification token is not configured")
		http.Error(w, "Webhook verification is not configured", http.StatusInternalServerError)
		return
	}

	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	log.Printf("Verification mode: %s", mode)
	log.Printf("Challenge received: %t", challenge != "")

	if mode != "subscribe" ||
		!hmac.Equal([]byte(token), []byte(h.Config.WebhookVerifyToken)) ||
		challenge == "" {
		log.Println("ERROR: Webhook verification failed")
		http.Error(w, "Webhook verification failed", http.StatusForbidden)
		return
	}

	log.Println("Webhook verification successful")

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte(challenge)); err != nil {
		log.Printf("Failed to write webhook challenge: %v", err)
	}
}

// receiveWebhook validates the signature and processes status notifications.
func (h *WhatsAppHandler) receiveWebhook(w http.ResponseWriter, r *http.Request) {
	log.Println("========== WHATSAPP WEBHOOK RECEIVED ==========")
	log.Printf("Method: %s", r.Method)
	log.Printf("Remote Address: %s", r.RemoteAddr)

	if h.Config == nil || h.Config.WhatsAppAppSecret == "" {
		log.Println("ERROR: Webhook App Secret is not configured")
		http.Error(w, "Webhook App Secret is not configured", http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 2*1024*1024))
	if err != nil {
		log.Printf("ERROR: Failed to read webhook request: %v", err)
		http.Error(w, "Failed to read webhook request", http.StatusBadRequest)
		return
	}

	log.Printf("Webhook request body received: %d bytes", len(body))

	signature := r.Header.Get("X-Hub-Signature-256")

	if !h.verifyWebhookSignature(body, signature) {
		log.Println("ERROR: Invalid webhook signature")
		http.Error(w, "Invalid webhook signature", http.StatusUnauthorized)
		return
	}

	log.Println("Webhook signature verified successfully")

	var payload models.WhatsAppWebhookPayload

	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("ERROR: Invalid webhook JSON: %v", err)
		http.Error(w, "Invalid webhook JSON", http.StatusBadRequest)
		return
	}

	log.Printf("Webhook object: %s", payload.Object)

	statusCount := 0

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			log.Printf("Webhook field: %s", change.Field)

			for _, status := range change.Value.Statuses {
				statusCount++

				log.Println("----------------------------------------")
				log.Printf("MESSAGE ID: %s", status.ID)
				log.Printf("RECIPIENT: %s", status.RecipientID)
				log.Printf("STATUS RECEIVED: %s", status.Status)
				log.Printf("TIMESTAMP: %s", status.Timestamp)

				timestamp, err := strconv.ParseInt(status.Timestamp, 10, 64)
				if err != nil {
					log.Printf(
						"ERROR: Invalid timestamp for message %s: %v",
						status.ID,
						err,
					)
					continue
				}

				eventTime := time.Unix(timestamp, 0).UTC()
				errorMessage := webhookErrorMessage(status.Errors)

				if errorMessage != "" {
					log.Printf("WHATSAPP ERROR: %s", errorMessage)
				}

				if h.Repository == nil {
					log.Println("ERROR: Scheduled repository is not configured")
					continue
				}

				err = h.Repository.UpdateWebhookStatus(
					r.Context(),
					status.ID,
					status.Status,
					eventTime,
					errorMessage,
				)

				if err != nil {
					log.Printf(
						"DATABASE UPDATE FAILED | Message ID: %s | Status: %s | Error: %v",
						status.ID,
						status.Status,
						err,
					)
					continue
				}

				log.Printf(
					"DATABASE UPDATED SUCCESSFULLY | Message ID: %s | Recipient: %s | Status: %s",
					status.ID,
					status.RecipientID,
					status.Status,
				)
			}
		}
	}

	if statusCount == 0 {
		log.Println("Webhook received successfully, but no message status events were included")
	}

	log.Println("========== WEBHOOK PROCESSING COMPLETED ==========")

	w.WriteHeader(http.StatusOK)
}

// verifyWebhookSignature validates Meta's X-Hub-Signature-256 header.
func (h *WhatsAppHandler) verifyWebhookSignature(body []byte, signature string) bool {
	if h.Config == nil || h.Config.WhatsAppAppSecret == "" {
		return false
	}

	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	providedSignature, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(h.Config.WhatsAppAppSecret))
	_, _ = mac.Write(body)

	expectedSignature := mac.Sum(nil)

	return hmac.Equal(providedSignature, expectedSignature)
}

// webhookErrorMessage extracts error details from a failed status notification.
func webhookErrorMessage(errors []struct {
	Code    int    `json:"code"`
	Title   string `json:"title"`
	Message string `json:"message"`
}) string {
	if len(errors) == 0 {
		return ""
	}

	messages := make([]string, 0, len(errors))

	for _, item := range errors {
		parts := make([]string, 0, 3)

		if item.Code != 0 {
			parts = append(parts, fmt.Sprintf("Code: %d", item.Code))
		}

		if item.Title != "" {
			parts = append(parts, item.Title)
		}

		if item.Message != "" {
			parts = append(parts, item.Message)
		}

		if len(parts) > 0 {
			messages = append(messages, strings.Join(parts, " - "))
		}
	}

	return strings.Join(messages, "; ")
}

// getProviderMessageID extracts the Meta message ID.
func getProviderMessageID(response *models.WhatsAppResponse) string {
	if response == nil || len(response.Messages) == 0 {
		return ""
	}

	return response.Messages[0].ID
}

// Validate scheduled message details.
func validateSchedule(
	recipient string,
	template models.Template,
	scheduledAt time.Time,
) error {
	phonePattern := regexp.MustCompile(`^[1-9][0-9]{7,14}$`)

	if !phonePattern.MatchString(strings.TrimSpace(recipient)) {
		return fmt.Errorf("invalid recipient phone number")
	}

	if strings.TrimSpace(template.Name) == "" {
		return fmt.Errorf("template name is required")
	}

	if strings.TrimSpace(template.Language.Code) == "" {
		return fmt.Errorf("template language code is required")
	}

	if scheduledAt.IsZero() || !scheduledAt.After(time.Now()) {
		return fmt.Errorf("scheduled_at must be a future date and time")
	}

	return nil
}

// Write JSON responses.
func writeJSON(
	w http.ResponseWriter,
	statusCode int,
	data interface{},
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to write JSON response: %v", err)
	}
}

// GetMessageHistory retrieves message history.
func (h *WhatsAppHandler) GetMessageHistory(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"success": false,
			"message": "Method not allowed",
		})
		return
	}

	history, err := h.Repository.GetMessageHistory(r.Context())
	if err != nil {
		log.Printf("Failed to retrieve message history: %v", err)

		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to retrieve message history",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"count":   len(history),
		"data":    history,
	})
}
