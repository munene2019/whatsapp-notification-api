package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"whatsapp-notification-api/models"
)

// Create an immediate message history record before sending.
func (r *ScheduledRepository) CreateImmediateHistory(
	ctx context.Context,
	recipient string,
	templateName string,
	languageCode string,
	payload json.RawMessage,
) (int64, error) {

	if len(payload) == 0 || !json.Valid(payload) {
		return 0, fmt.Errorf("invalid WhatsApp payload")
	}

	result, err := r.DB.ExecContext(
		ctx,
		`INSERT INTO message_history
		(
			recipient,
			template_name,
			language_code,
			message_type,
			delivery_mode,
			payload,
			status
		)
		VALUES (?, ?, ?, 'template', 'immediate', ?, 'pending')`,
		recipient,
		templateName,
		languageCode,
		string(payload),
	)

	if err != nil {
		return 0, fmt.Errorf("failed to save message history: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get history ID: %w", err)
	}

	return id, nil
}

// Update an immediate message after the WhatsApp API responds.
func (r *ScheduledRepository) UpdateImmediateHistory(
	ctx context.Context,
	id int64,
	status string,
	providerMessageID string,
	messageError string,
) error {

	if status != "accepted" && status != "failed" {
		return fmt.Errorf("invalid immediate message status: %s", status)
	}

	result, err := r.DB.ExecContext(
		ctx,
		`UPDATE message_history
		SET status = ?,
			provider_message_id = NULLIF(?, ''),
			error_message = NULLIF(?, ''),
			submitted_at = CASE
				WHEN ? = 'accepted' THEN UTC_TIMESTAMP(6)
				ELSE submitted_at
			END
		WHERE id = ?`,
		status,
		providerMessageID,
		messageError,
		status,
		id,
	)

	if err != nil {
		return fmt.Errorf("failed to update message history: %w", err)
	}

	return checkAffectedRows(result)
}

// Update message delivery status using Meta webhook notifications.
// This method handles both immediate and scheduled messages.
func (r *ScheduledRepository) UpdateWebhookStatus(
	ctx context.Context,
	providerMessageID string,
	status string,
	eventTime time.Time,
	messageError string,
) error {

	if providerMessageID == "" {
		return fmt.Errorf("provider message ID is required")
	}

	switch status {
	case "sent", "delivered", "read", "failed":
	default:
		return fmt.Errorf("unsupported webhook status: %s", status)
	}

	if eventTime.IsZero() {
		return fmt.Errorf("webhook event timestamp is required")
	}

	eventTime = eventTime.UTC()

	result, err := r.DB.ExecContext(
		ctx,
		`UPDATE message_history
		SET
			status = CASE
				WHEN status IN ('delivered', 'read') THEN status
				WHEN status = 'failed' THEN status
				WHEN ? = 'failed' THEN 'failed'
				WHEN ? = 'read' THEN 'read'
				WHEN ? = 'delivered' THEN 'delivered'
				WHEN ? = 'sent' THEN 'sent'
				ELSE status
			END,

			sent_at = CASE
				WHEN ? = 'sent' THEN COALESCE(sent_at, ?)
				ELSE sent_at
			END,

			delivered_at = CASE
				WHEN ? = 'delivered' THEN COALESCE(delivered_at, ?)
				ELSE delivered_at
			END,

			read_at = CASE
				WHEN ? = 'read' THEN COALESCE(read_at, ?)
				ELSE read_at
			END,

			error_message = CASE
				WHEN ? = 'failed' THEN NULLIF(?, '')
				ELSE error_message
			END

		WHERE provider_message_id = ?`,
		status,
		status,
		status,
		status,
		status,
		eventTime,
		status,
		eventTime,
		status,
		eventTime,
		status,
		messageError,
		providerMessageID,
	)

	if err != nil {
		return fmt.Errorf("failed to update webhook message status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check webhook update: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf(
			"no message history found for provider message ID: %s",
			providerMessageID,
		)
	}

	return nil
}

// Retrieve message history.
func (r *ScheduledRepository) GetMessageHistory(
	ctx context.Context,
) ([]models.MessageHistory, error) {

	rows, err := r.DB.QueryContext(
		ctx,
		`SELECT
			id,
			recipient,
			template_name,
			language_code,
			message_type,
			delivery_mode,
			payload,
			scheduled_message_id,
			scheduled_at,
			submitted_at,
			sent_at,
			delivered_at,
			read_at,
			provider_message_id,
			status,
			error_message,
			created_at,
			updated_at
		FROM message_history
		ORDER BY created_at DESC`,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve message history: %w", err)
	}
	defer rows.Close()

	var messages []models.MessageHistory

	for rows.Next() {
		var message models.MessageHistory
		var templateName sql.NullString
		var languageCode sql.NullString
		var payload []byte
		var scheduledMessageID sql.NullInt64
		var scheduledAt sql.NullTime
		var submittedAt sql.NullTime
		var sentAt sql.NullTime
		var deliveredAt sql.NullTime
		var readAt sql.NullTime
		var providerMessageID sql.NullString
		var errorMessage sql.NullString

		err := rows.Scan(
			&message.ID,
			&message.Recipient,
			&templateName,
			&languageCode,
			&message.MessageType,
			&message.DeliveryMode,
			&payload,
			&scheduledMessageID,
			&scheduledAt,
			&submittedAt,
			&sentAt,
			&deliveredAt,
			&readAt,
			&providerMessageID,
			&message.Status,
			&errorMessage,
			&message.CreatedAt,
			&message.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan message history: %w", err)
		}

		if templateName.Valid {
			message.TemplateName = &templateName.String
		}
		if languageCode.Valid {
			message.LanguageCode = &languageCode.String
		}
		if len(payload) > 0 {
			message.Payload = json.RawMessage(payload)
		}
		if scheduledMessageID.Valid {
			message.ScheduledMessageID = &scheduledMessageID.Int64
		}
		if scheduledAt.Valid {
			message.ScheduledAt = &scheduledAt.Time
		}
		if submittedAt.Valid {
			message.SubmittedAt = &submittedAt.Time
		}
		if sentAt.Valid {
			message.SentAt = &sentAt.Time
		}
		if deliveredAt.Valid {
			message.DeliveredAt = &deliveredAt.Time
		}
		if readAt.Valid {
			message.ReadAt = &readAt.Time
		}
		if providerMessageID.Valid {
			message.ProviderMessageID = &providerMessageID.String
		}
		if errorMessage.Valid {
			message.ErrorMessage = &errorMessage.String
		}

		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}
