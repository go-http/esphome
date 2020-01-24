package esphome

import "errors"

// Errors.
var (
	ErrPassword = errors.New("esphome: invalid password")
	ErrTimeout  = errors.New("esphome: timeout")
	ErrObjectID = errors.New("esphome: unknown object identifier")
	ErrEntity   = errors.New("esphome: entity not found")
)
