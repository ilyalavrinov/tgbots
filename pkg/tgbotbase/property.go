package tgbotbase

import "context"

type PropertyValue struct {
	Value string
	User  UserID
	Chat  ChatID
}

type PropertyStorage interface {
	GetProperty(ctx context.Context, name string, user UserID, chat ChatID) (string, error)
	SetPropertyForUser(ctx context.Context, name string, user UserID, value interface{}) error
	SetPropertyForChat(ctx context.Context, name string, chat ChatID, value interface{}) error
	SetPropertyForUserInChat(ctx context.Context, name string, user UserID, chat ChatID, value interface{}) error
	GetEveryHavingProperty(ctx context.Context, name string) ([]PropertyValue, error)
}
