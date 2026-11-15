package services

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"whatsapp-notification-api/models"
	"whatsapp-notification-api/repositories"
)

type SchedulerService struct {
	repository      *repositories.ScheduledRepository
	whatsappService *WhatsAppService
	interval        time.Duration
	batchSize       int
}

func NewSchedulerService(
	repository *repositories.ScheduledRepository,
	whatsappService *WhatsAppService,
) *SchedulerService {
	return &SchedulerService{
		repository:      repository,
		whatsappService: whatsappService,
		interval:        5 * time.Second,
		batchSize:       20,
	}
}

// Start runs the background scheduler until the context is cancelled.
func (s *SchedulerService) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	log.Println("WhatsApp scheduler started")

	// Process due messages immediately on startup.
	s.processDueMessages(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("WhatsApp scheduler stopped")
			return

		case <-ticker.C:
			s.processDueMessages(ctx)
		}
	}
}

func (s *SchedulerService) processDueMessages(ctx context.Context) {
	messages, err := s.repository.ClaimDue(ctx, s.batchSize)
	if err != nil {
		log.Printf("Scheduler: failed to claim messages: %v", err)
		return
	}

	for _, message := range messages {
		if ctx.Err() != nil {
			return
		}

		s.processMessage(ctx, message)
	}
}

func (s *SchedulerService) processMessage(
	ctx context.Context,
	message models.ScheduledMessage,
) {
	var request models.SendRequest

	if err := json.Unmarshal(message.Payload, &request); err != nil {
		s.markFailed(
			ctx,
			message.ID,
			"Invalid stored WhatsApp payload: "+err.Error(),
		)
		return
	}

	// Ensure the database recipient is used.
	request.To = message.Recipient

	// Send the message and retain Meta's response.
	response, err := s.whatsappService.SendMessage(ctx, request)
	if err != nil {
		s.markFailed(ctx, message.ID, err.Error())
		return
	}

	// Extract the provider message ID.
	if response == nil || len(response.Messages) == 0 {
		s.markFailed(
			ctx,
			message.ID,
			"WhatsApp API accepted the request but returned no message ID",
		)
		return
	}

	providerMessageID := response.Messages[0].ID

	if providerMessageID == "" {
		s.markFailed(
			ctx,
			message.ID,
			"WhatsApp API returned an empty provider message ID",
		)
		return
	}

	log.Printf(
		"Scheduler: Meta accepted message %d. Provider ID: %s",
		message.ID,
		providerMessageID,
	)

	// Save the provider ID so webhook notifications can find this history record.
	if err := s.repository.MarkSent(
		ctx,
		message.ID,
		providerMessageID,
	); err != nil {
		log.Printf(
			"Scheduler: message %d was accepted but status update failed: %v",
			message.ID,
			err,
		)
		return
	}

	log.Printf(
		"Scheduler: message %d accepted by Meta for %s. Provider ID: %s",
		message.ID,
		message.Recipient,
		providerMessageID,
	)
}

func (s *SchedulerService) markFailed(
	ctx context.Context,
	id int64,
	messageError string,
) {
	if err := s.repository.MarkFailed(ctx, id, messageError); err != nil {
		log.Printf(
			"Scheduler: failed to update status for message %d: %v",
			id,
			err,
		)
		return
	}

	log.Printf(
		"Scheduler: message %d failed: %s",
		id,
		messageError,
	)
}
