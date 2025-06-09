package database

import "errors"

var (
	ErrUserNotFound        = errors.New("user not found")
	ErrChatSessionNotFound = errors.New("chat session not found")
	ErrMessageNotFound     = errors.New("message not found")
)
