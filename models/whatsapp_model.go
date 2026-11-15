package models

import (
	"encoding/json"
	"time"
)

type SendRequest struct {
	MessagingProduct string   `json:"messaging_product"`
	To               string   `json:"to"`
	Type             string   `json:"type"`
	Template         Template `json:"template"`
}

type BulkRequest struct {
	Recipients   []string `json:"recipients"`
	TemplateName string   `json:"template_name"`
	Language     string   `json:"language"`
	Parameters   []string `json:"parameters,omitempty"`
}

// Single scheduled message request.
type ScheduleRequest struct {
	SendRequest
	ScheduledAt time.Time `json:"scheduled_at"`
}

// Bulk scheduled message request.
type BulkScheduleRequest struct {
	Recipients  []string  `json:"recipients"`
	Template    Template  `json:"template"`
	ScheduledAt time.Time `json:"scheduled_at"`
}

type WhatsAppPayload struct {
	MessagingProduct string   `json:"messaging_product"`
	RecipientType    string   `json:"recipient_type"`
	To               string   `json:"to"`
	Type             string   `json:"type"`
	Template         Template `json:"template"`
}

type Template struct {
	Name       string      `json:"name"`
	Language   Language    `json:"language"`
	Components []Component `json:"components,omitempty"`
}

type Language struct {
	Code string `json:"code"`
}

type Component struct {
	Type       string      `json:"type"`
	Parameters []Parameter `json:"parameters"`
}

type Parameter struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type WhatsAppResponse struct {
	MessagingProduct string `json:"messaging_product"`

	Contacts []struct {
		Input string `json:"input"`
		WaID  string `json:"wa_id"`
	} `json:"contacts"`

	Messages []struct {
		ID string `json:"id"`
	} `json:"messages"`
}

type SendResult struct {
	Phone   string            `json:"phone"`
	Success bool              `json:"success"`
	Data    *WhatsAppResponse `json:"data,omitempty"`
	Error   string            `json:"error,omitempty"`
}

type BulkResponse struct {
	Success bool         `json:"success"`
	Total   int          `json:"total"`
	Results []SendResult `json:"results"`
}

// Database representation of a scheduled message.
type ScheduledMessage struct {
	ID                int64           `json:"id"`
	Recipient         string          `json:"recipient"`
	TemplateName      string          `json:"template_name"`
	LanguageCode      string          `json:"language"`
	Payload           json.RawMessage `json:"payload"`
	ScheduledAt       time.Time       `json:"scheduled_at"`
	Status            string          `json:"status"`
	Attempts          int             `json:"attempts"`
	ErrorMessage      *string         `json:"error_message,omitempty"`
	ProviderMessageID *string         `json:"provider_message_id,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// Response after scheduling messages.
type ScheduleResponse struct {
	Success bool               `json:"success"`
	Message string             `json:"message"`
	Total   int                `json:"total"`
	Results []ScheduledMessage `json:"results,omitempty"`
}

// Database representation of a message in message_history.
type MessageHistory struct {
	ID                 int64           `json:"id"`
	Recipient          string          `json:"recipient"`
	TemplateName       *string         `json:"template_name,omitempty"`
	LanguageCode       *string         `json:"language_code,omitempty"`
	MessageType        string          `json:"message_type"`
	DeliveryMode       string          `json:"delivery_mode"`
	Payload            json.RawMessage `json:"payload,omitempty"`
	ScheduledMessageID *int64          `json:"scheduled_message_id,omitempty"`
	ScheduledAt        *time.Time      `json:"scheduled_at,omitempty"`
	SubmittedAt        *time.Time      `json:"submitted_at,omitempty"`
	SentAt             *time.Time      `json:"sent_at,omitempty"`
	DeliveredAt        *time.Time      `json:"delivered_at,omitempty"`
	ReadAt             *time.Time      `json:"read_at,omitempty"`
	ProviderMessageID  *string         `json:"provider_message_id,omitempty"`
	Status             string          `json:"status"`
	ErrorMessage       *string         `json:"error_message,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// Meta WhatsApp webhook notification payload.
type WhatsAppWebhookPayload struct {
	Object string `json:"object"`
	Entry  []struct {
		ID      string `json:"id"`
		Changes []struct {
			Field string `json:"field"`
			Value struct {
				MessagingProduct string `json:"messaging_product"`

				Metadata struct {
					DisplayPhoneNumber string `json:"display_phone_number"`
					PhoneNumberID      string `json:"phone_number_id"`
				} `json:"metadata"`

				Statuses []WhatsAppWebhookStatus `json:"statuses"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

// Individual delivery status notification from Meta.
type WhatsAppWebhookStatus struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Timestamp   string `json:"timestamp"`
	RecipientID string `json:"recipient_id"`

	Errors []struct {
		Code    int    `json:"code"`
		Title   string `json:"title"`
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

// Response returned by the webhook verification endpoint.
type WebhookVerificationResponse struct {
	Mode        string `json:"hub.mode"`
	VerifyToken string `json:"hub.verify_token"`
	Challenge   string `json:"hub.challenge"`
}
