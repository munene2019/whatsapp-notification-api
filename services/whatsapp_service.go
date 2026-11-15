package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"whatsapp-notification-api/config"
	"whatsapp-notification-api/models"
)

type WhatsAppService struct {
	config     *config.Config
	httpClient *http.Client
}

func NewWhatsAppService(cfg *config.Config) *WhatsAppService {
	return &WhatsAppService{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *WhatsAppService) SendMessage(
	ctx context.Context,
	request models.SendRequest,
) (*models.WhatsAppResponse, error) {

	// Validate recipient phone number.
	to := strings.TrimSpace(request.To)

	validPhone := regexp.MustCompile(`^[1-9][0-9]{7,14}$`)
	if !validPhone.MatchString(to) {
		return nil, fmt.Errorf(
			"invalid phone number: use international format without +, e.g. 254712345678",
		)
	}

	// Read template details from nested request structure.
	templateName := strings.TrimSpace(request.Template.Name)
	if templateName == "" {
		return nil, fmt.Errorf("template name is required")
	}

	language := strings.TrimSpace(request.Template.Language.Code)
	if language == "" {
		return nil, fmt.Errorf("template language code is required")
	}

	// Validate configuration.
	if strings.TrimSpace(s.config.WhatsAppToken) == "" {
		return nil, fmt.Errorf("WhatsApp access token is missing")
	}

	if strings.TrimSpace(s.config.PhoneNumberID) == "" {
		return nil, fmt.Errorf("WhatsApp phone number ID is missing")
	}

	apiVersion := strings.TrimSpace(s.config.WhatsAppAPIVersion)
	if apiVersion == "" {
		apiVersion = "v25.0"
	}

	// Meta WhatsApp Cloud API endpoint.
	endpoint := fmt.Sprintf(
		"https://graph.facebook.com/%s/%s/messages",
		apiVersion,
		s.config.PhoneNumberID,
	)

	// Construct Meta WhatsApp payload.
	payload := models.WhatsAppPayload{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               to,
		Type:             "template",
		Template: models.Template{
			Name: templateName,
			Language: models.Language{
				Code: language,
			},
		},
	}

	// Convert payload to JSON.
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode WhatsApp payload: %w", err)
	}

	// Create HTTP request with context.
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewBuffer(jsonPayload),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create WhatsApp request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.config.WhatsAppToken)
	req.Header.Set("Content-Type", "application/json")

	// Send request to Meta.
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to WhatsApp API: %w", err)
	}
	defer resp.Body.Close()

	// Read Meta response.
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read WhatsApp response: %w", err)
	}

	// Handle unsuccessful responses.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf(
			"WhatsApp API returned HTTP %d: %s",
			resp.StatusCode,
			string(responseBody),
		)
	}

	// Decode successful response.
	var result models.WhatsAppResponse

	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode WhatsApp response: %w", err)
	}

	return &result, nil
}
