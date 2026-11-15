package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"whatsapp-notification-api/models"
)

type ScheduledRepository struct {
	DB *sql.DB
}

func NewScheduledRepository(db *sql.DB) *ScheduledRepository {
	return &ScheduledRepository{DB: db}
}

// Insert the matching message_history record inside the same transaction.
func insertScheduledHistory(
	ctx context.Context,
	tx *sql.Tx,
	scheduledMessageID int64,
	message models.ScheduledMessage,
) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO message_history
		(
			recipient,
			template_name,
			language_code,
			message_type,
			delivery_mode,
			payload,
			scheduled_message_id,
			scheduled_at,
			status
		)
		VALUES (?, ?, ?, 'template', 'scheduled', ?, ?, ?, 'pending')`,
		message.Recipient,
		message.TemplateName,
		message.LanguageCode,
		string(message.Payload),
		scheduledMessageID,
		message.ScheduledAt.UTC(),
	)

	if err != nil {
		return fmt.Errorf("failed to create scheduled message history: %w", err)
	}

	return nil
}

// Save a single scheduled message and its history record.
func (r *ScheduledRepository) Create(
	ctx context.Context,
	message models.ScheduledMessage,
) (*models.ScheduledMessage, error) {

	if len(message.Payload) == 0 || !json.Valid(message.Payload) {
		return nil, fmt.Errorf("invalid WhatsApp payload")
	}

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(
		ctx,
		`INSERT INTO scheduled_messages
		(recipient, template_name, language_code, payload, scheduled_at, status)
		VALUES (?, ?, ?, ?, ?, 'pending')`,
		message.Recipient,
		message.TemplateName,
		message.LanguageCode,
		string(message.Payload),
		message.ScheduledAt.UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to save scheduled message: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled message ID: %w", err)
	}

	message.ID = id
	message.Status = "pending"

	if err := insertScheduledHistory(ctx, tx, id, message); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit scheduled message: %w", err)
	}

	return &message, nil
}

// Save multiple scheduled messages and their history records in one transaction.
func (r *ScheduledRepository) CreateBulk(
	ctx context.Context,
	messages []models.ScheduledMessage,
) ([]models.ScheduledMessage, error) {

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	for i := range messages {
		if len(messages[i].Payload) == 0 ||
			!json.Valid(messages[i].Payload) {
			return nil, fmt.Errorf(
				"invalid payload for recipient %s",
				messages[i].Recipient,
			)
		}

		result, err := tx.ExecContext(
			ctx,
			`INSERT INTO scheduled_messages
			(recipient, template_name, language_code, payload, scheduled_at, status)
			VALUES (?, ?, ?, ?, ?, 'pending')`,
			messages[i].Recipient,
			messages[i].TemplateName,
			messages[i].LanguageCode,
			string(messages[i].Payload),
			messages[i].ScheduledAt.UTC(),
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to save scheduled message for %s: %w",
				messages[i].Recipient,
				err,
			)
		}

		id, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("failed to get inserted ID: %w", err)
		}

		messages[i].ID = id
		messages[i].Status = "pending"

		if err := insertScheduledHistory(ctx, tx, id, messages[i]); err != nil {
			return nil, fmt.Errorf(
				"failed to save history for %s: %w",
				messages[i].Recipient,
				err,
			)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit scheduled messages: %w", err)
	}

	return messages, nil
}

// Retrieve scheduled messages.
func (r *ScheduledRepository) GetAll(
	ctx context.Context,
) ([]models.ScheduledMessage, error) {

	rows, err := r.DB.QueryContext(
		ctx,
		`SELECT id, recipient, template_name, language_code,
		payload, scheduled_at, status, attempts,
		error_message, provider_message_id, created_at, updated_at
		FROM scheduled_messages
		ORDER BY scheduled_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve scheduled messages: %w", err)
	}
	defer rows.Close()

	var messages []models.ScheduledMessage

	for rows.Next() {
		message, err := scanScheduledMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}

// Retrieve one scheduled message by ID.
func (r *ScheduledRepository) GetByID(
	ctx context.Context,
	id int64,
) (*models.ScheduledMessage, error) {

	row := r.DB.QueryRowContext(
		ctx,
		`SELECT id, recipient, template_name, language_code,
		payload, scheduled_at, status, attempts,
		error_message, provider_message_id, created_at, updated_at
		FROM scheduled_messages
		WHERE id = ?`,
		id,
	)

	message, err := scanScheduledMessage(row)
	if err != nil {
		return nil, err
	}

	return &message, nil
}

// Claim due messages for processing.
func (r *ScheduledRepository) ClaimDue(
	ctx context.Context,
	limit int,
) ([]models.ScheduledMessage, error) {

	if limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than zero")
	}

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(
		ctx,
		`SELECT id, recipient, template_name, language_code,
		payload, scheduled_at, status, attempts,
		error_message, provider_message_id, created_at, updated_at
		FROM scheduled_messages
		WHERE status = 'pending'
		AND scheduled_at <= UTC_TIMESTAMP(6)
		ORDER BY scheduled_at ASC
		LIMIT ?
		FOR UPDATE`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find due messages: %w", err)
	}

	var messages []models.ScheduledMessage

	for rows.Next() {
		message, err := scanScheduledMessage(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}

	if err := rows.Close(); err != nil {
		return nil, err
	}

	for _, message := range messages {
		_, err := tx.ExecContext(
			ctx,
			`UPDATE scheduled_messages
			SET status = 'processing', attempts = attempts + 1
			WHERE id = ? AND status = 'pending'`,
			message.ID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to claim message %d: %w", message.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to claim messages: %w", err)
	}

	for i := range messages {
		messages[i].Status = "processing"
		messages[i].Attempts++
	}

	return messages, nil
}

// Mark a message as accepted by the WhatsApp API and save its provider ID.
func (r *ScheduledRepository) MarkSent(
	ctx context.Context,
	id int64,
	providerMessageID string,
) error {

	if providerMessageID == "" {
		return fmt.Errorf("provider message ID is required")
	}

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(
		ctx,
		`UPDATE scheduled_messages
		SET status = 'sent',
			provider_message_id = ?,
			error_message = NULL
		WHERE id = ? AND status = 'processing'`,
		providerMessageID,
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to mark scheduled message as sent: %w", err)
	}

	if err := checkAffectedRows(result); err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`UPDATE message_history
		SET provider_message_id = ?,
			status = CASE
				WHEN status IN ('delivered', 'read', 'failed') THEN status
				ELSE 'accepted'
			END,
			submitted_at = COALESCE(submitted_at, UTC_TIMESTAMP(6))
		WHERE scheduled_message_id = ?`,
		providerMessageID,
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to update scheduled message history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit sent message update: %w", err)
	}

	return nil
}

// Mark a message as failed.
func (r *ScheduledRepository) MarkFailed(
	ctx context.Context,
	id int64,
	messageError string,
) error {

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(
		ctx,
		`UPDATE scheduled_messages
		SET status = 'failed',
			error_message = ?
		WHERE id = ? AND status = 'processing'`,
		messageError,
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to mark scheduled message as failed: %w", err)
	}

	if err := checkAffectedRows(result); err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`UPDATE message_history
		SET status = 'failed',
			error_message = ?
		WHERE scheduled_message_id = ?`,
		messageError,
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to update failed message history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit failed message update: %w", err)
	}

	return nil
}

// Cancel a pending scheduled message.
func (r *ScheduledRepository) Cancel(
	ctx context.Context,
	id int64,
) error {

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(
		ctx,
		`UPDATE scheduled_messages
		SET status = 'cancelled'
		WHERE id = ? AND status = 'pending'`,
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to cancel scheduled message: %w", err)
	}

	if err := checkAffectedRows(result); err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`UPDATE message_history
		SET status = 'cancelled'
		WHERE scheduled_message_id = ? AND status = 'pending'`,
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to update cancelled message history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit cancellation: %w", err)
	}

	return nil
}

// Scan a database row into a ScheduledMessage.
func scanScheduledMessage(scanner interface {
	Scan(dest ...any) error
}) (models.ScheduledMessage, error) {

	var message models.ScheduledMessage
	var payload []byte
	var errorMessage sql.NullString
	var providerMessageID sql.NullString

	err := scanner.Scan(
		&message.ID,
		&message.Recipient,
		&message.TemplateName,
		&message.LanguageCode,
		&payload,
		&message.ScheduledAt,
		&message.Status,
		&message.Attempts,
		&errorMessage,
		&providerMessageID,
		&message.CreatedAt,
		&message.UpdatedAt,
	)
	if err != nil {
		return message, err
	}

	message.Payload = json.RawMessage(payload)

	if errorMessage.Valid {
		message.ErrorMessage = &errorMessage.String
	}
	if providerMessageID.Valid {
		message.ProviderMessageID = &providerMessageID.String
	}

	return message, nil
}

func checkAffectedRows(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return fmt.Errorf("message not found or no longer in the required status")
	}

	return nil
}

func IsValidStatus(status string) bool {
	switch strings.ToLower(status) {
	case "pending", "processing", "sent", "failed", "cancelled":
		return true
	default:
		return false
	}
}
