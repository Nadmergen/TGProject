package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
)

// BatchMarkAsDelivered marks multiple messages as delivered in one query
func BatchMarkAsDelivered(db *sql.DB, messageIDs []int64, recipientID int64) error {
	if len(messageIDs) == 0 {
		return nil
	}

	query := `UPDATE messages SET delivered_at = NOW() 
	         WHERE id = ANY($1) AND recipient_id = $2 AND delivered_at IS NULL`

	result, err := db.ExecContext(context.Background(), query, messageIDs, recipientID)
	if err != nil {
		slog.Error("batch mark as delivered failed", slog.Any("error", err))
		return fmt.Errorf("database error: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	slog.Debug("messages marked as delivered", slog.Int64("count", rowsAffected))

	return nil
}

// SearchMessagesWithFilters searches messages with additional filters
func SearchMessagesWithFilters(db *sql.DB, userID int64, query string, messageType string, limit int) error {
	sqlQuery := `SELECT id, sender_id, recipient_id, content, type, file_url, file_name, created_at
			FROM messages
			WHERE (sender_id = $1 OR recipient_id = $1)
			AND content ILIKE $2`

	if messageType != "" {
		sqlQuery += ` AND type = $3`
	}

	sqlQuery += ` ORDER BY created_at DESC LIMIT $4`

	rows, err := db.QueryContext(context.Background(), sqlQuery, userID, "%"+query+"%", messageType, limit)
	if err != nil {
		slog.Error("search messages failed", slog.Any("error", err))
		return err
	}
	defer rows.Close()

	// Process rows...
	return nil
}

// DeleteMessageWithAttachment deletes message and its attachments
func DeleteMessageWithAttachment(db *sql.DB, messageID int64, userID int64) error {
	var fileURL sql.NullString

	err := db.QueryRowContext(context.Background(),
		"SELECT file_url FROM messages WHERE id = $1 AND sender_id = $2",
		messageID, userID).Scan(&fileURL)

	if err == sql.ErrNoRows {
		return fmt.Errorf("message not found")
	}

	if err != nil {
		slog.Error("delete message lookup failed", slog.Any("error", err))
		return err
	}

	// Delete message
	result, err := db.ExecContext(context.Background(),
		"DELETE FROM messages WHERE id = $1 AND sender_id = $2",
		messageID, userID)

	if err != nil {
		slog.Error("delete message failed", slog.Any("error", err))
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("message not found")
	}

	// Delete attachment if exists
	if fileURL.Valid && fileURL.String != "" {
		slog.Debug("attachment marked for deletion", slog.String("url", fileURL.String))
		// Delete from S3/storage here
	}

	return nil
}