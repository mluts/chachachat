package main

import (
	"crypto/rand"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// HandleSize means how many bytes connection id has
	HandleSize = 16
	// StaleConnection means when connection will be considered as stale
	StaleConnection = time.Second * 10
	// MessageQueueSize says how much messages will client buffer have
	MessageQueueSize = 10
)

// User represents a single chat client
type User struct {
	username    string
	connections int
}

// Connection represents a single client connection
type Connection struct {
	ch         chan string
	lastSeenAt time.Time
	user       *User
	reading    bool
}

// Message represents a single user message
type Message struct {
	Handle  Handle
	Message string
}

// Chat represents a chat state
type Chat struct {
	sync.Mutex
	knownUsers  []*User
	connections map[string]*Connection
}

// Nothing represents an empty argument/response
type Nothing struct{}

// Handle is a connection id
type Handle []byte

func (c *Connection) open() {
	c.ch = make(chan string, MessageQueueSize)
	c.user.connections++
}

func (c *Connection) close() {
	close(c.ch)
	c.user.connections--
	if c.user.connections < 0 {
		panic(fmt.Errorf("user.connections can't be %d", c.user.connections))
	}
}

func (c *Connection) stale() bool {
	return !c.reading && time.Since(c.lastSeenAt) > StaleConnection
}

func (c *Connection) write(message string) {
	c.ch <- message
}

func (c *Connection) read() (string, error) {
	if c.reading {
		return "", fmt.Errorf("multiple reads detected")
	}
	c.reading = true
	defer func() { c.reading = false }()

	if len(c.ch) > 0 {
		out := make([]string, 0)

		for len(c.ch) > 0 {
			msg := <-c.ch
			out = append(out, msg)
		}

		return strings.Join(out, "\n"), nil
	}

	var err error

	msg, closed := <-c.ch

	if closed {
		err = fmt.Errorf("unexpected closed connection channel")
	}

	return msg, err
}

func (c *Connection) refresh() {
	c.lastSeenAt = time.Now()
}

func (h *Handle) toString() string {
	return string(*h)
}

func generateHandle() (Handle, error) {
	h := make([]byte, HandleSize)
	_, err := rand.Read(h)
	return h, err
}

func (c *Chat) openConnection(u *User) (*Handle, error) {
	handle, err := generateHandle()
	if err != nil {
		return nil, err
	}

	conn := &Connection{
		make(chan string, 1),
		time.Now(),
		u,
		false,
	}
	conn.open()

	c.Lock()
	c.connections[handle.toString()] = conn
	c.Unlock()

	return &handle, err
}

func (c *Chat) closeConnection(h *Handle) error {
	var (
		conn *Connection
	)
	conn, ok := c.connections[h.toString()]
	if !ok {
		return fmt.Errorf("Don't know such connection: %s", h)
	}

	c.Lock()
	delete(c.connections, h.toString())
	conn.close()
	c.Unlock()

	return nil
}

func (c *Chat) lookupUser(username string) *User {
	for _, user := range c.knownUsers {
		if user.username == username {
			return user
		}
	}
	user := &User{username, 0}
	c.Lock()
	c.knownUsers = append(c.knownUsers, user)
	c.Unlock()
	return user
}

func (c *Chat) closeStaleConnections() {
	for key, conn := range c.connections {
		if conn.stale() {
			handle := Handle(key)
			c.Logout(handle, &Nothing{})
		}
	}
}

func (c *Chat) writeTo(user *User, msg string) error {
	if user.connections == 0 {
		return fmt.Errorf("User %s is not connected", user.username)
	}

	for _, conn := range c.connections {
		if conn.user == user {
			go conn.write(msg)
		}
	}

	return nil
}

func (c *Chat) welcomeUser(user *User) {
	for _, u := range c.knownUsers {
		if u == user {
			c.writeTo(u, fmt.Sprintf("Hello %s", user.username))
		} else {
			c.writeTo(u, fmt.Sprintf("Say hello to %s", user.username))
		}
	}
}

func (c *Chat) byeUser(user *User) {
	for _, u := range c.knownUsers {
		c.writeTo(u, fmt.Sprintf("%s leaved us", user.username))
	}
}

// Login returns a connection handle
func (c *Chat) Login(username string, out *Handle) error {
	user := c.lookupUser(username)
	handle, err := c.openConnection(user)

	if err != nil {
		return err
	}

	*out = *handle

	if user.connections == 1 {
		c.welcomeUser(user)
	}

	return nil
}

// Logout frees a connection for a given handle
func (c *Chat) Logout(h Handle, out *Nothing) error {
	if conn, ok := c.connections[h.toString()]; ok {
		if conn.user.connections == 1 {
			c.byeUser(conn.user)
		}
	}
	return c.closeConnection(&h)
}

// Write broadcasts a message to all users
func (c *Chat) Write(msg Message, out *Nothing) error {
	var conn *Connection
	conn, ok := c.connections[msg.Handle.toString()]
	if !ok {
		return fmt.Errorf("Not authorized")
	}
	conn.refresh()

	for _, conn := range c.connections {
		go conn.write(msg.Message)
	}

	return nil
}

// Poll waits for a new message for a given connection
func (c *Chat) Poll(h Handle, out *string) error {
	var conn *Connection
	conn, ok := c.connections[h.toString()]

	if !ok {
		return fmt.Errorf("%v connection closed", h)
	}

	defer conn.refresh()
	msg, err := conn.read()

	if err != nil {
		return err
	}

	*out = msg

	return nil
}
