package common

import (
	"crypto/rand"
	"errors"
)

// Message represents a single user message
type Message struct {
	Handle  Handle
	Message string
}

// Nothing represents an empty argument/response
type Nothing struct{}

// Handle is a connection id
type Handle []byte

// ToString returns the handle as a string
func (h *Handle) ToString() string {
	return string(*h)
}

// GenerateHandle returns an unique handle
func GenerateHandle(size int) (Handle, error) {
	h := make([]byte, size)
	_, err := rand.Read(h)
	return h, err
}

// ErrTimeout means that there were no activity on given connection
var ErrTimeout = errors.New("connection timeout")
