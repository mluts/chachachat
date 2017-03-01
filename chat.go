package main

import (
	"fmt"
	"github.com/mluts/chachachat/common"
	"strings"
	"sync"
	"time"
)

const (
	// HandleSize means how many bytes connection id has
	HandleSize = 16
	// StaleConnection means when connection will be considered as stale
	StaleConnection = time.Second * 3
	// MessageQueueSize says how much messages will client buffer has
	MessageQueueSize = 10
	// ReadTimeout is the time after which giving up on Connection.read
	ReadTimeout = time.Second * 10
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
	closed     bool
}

// Chat represents a chat state
type Chat struct {
	m           *sync.Mutex
	knownUsers  []*User
	connections map[string]*Connection
}

func (c *Connection) open() {
	c.ch = make(chan string, MessageQueueSize)
	c.user.connections++
}

func (c *Connection) close() {
	c.closed = true
	close(c.ch)
	c.user.connections--
	if c.user.connections < 0 {
		panic(fmt.Errorf("user.connections can't be %d", c.user.connections))
	}
}

func (c *Connection) stale() bool {
	return !c.reading && time.Since(c.lastSeenAt) > StaleConnection
}

func (c *Connection) write(message string) error {
	if c.closed {
		return fmt.Errorf("connection closed")
	}
	c.ch <- message
	return nil
}

func (c *Connection) read(timeout time.Duration) (string, error) {
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

	var (
		err  error
		msg  string
		open bool
	)

	t := time.After(timeout)
	select {
	case msg, open = <-c.ch:
		if !open {
			err = fmt.Errorf("unexpected closed connection channel")
		}
	case <-t:
		return "", common.ErrTimeout
	}

	return msg, err
}

func (c *Connection) refresh() {
	c.lastSeenAt = time.Now()
}

func (c *Chat) openConnection(u *User) (*common.Handle, error) {
	handle, err := common.GenerateHandle(HandleSize)
	if err != nil {
		return nil, err
	}

	conn := &Connection{
		make(chan string, 1),
		time.Now(),
		u,
		false,
		false,
	}
	conn.open()

	c.connections[handle.ToString()] = conn

	return &handle, err
}

func (c *Chat) closeConnection(h *common.Handle) error {
	var (
		conn *Connection
	)
	conn, ok := c.connections[h.ToString()]
	if !ok {
		return fmt.Errorf("Don't know such connection: %s", h)
	}

	delete(c.connections, h.ToString())
	conn.close()

	return nil
}

func (c *Chat) lookupUser(username string) *User {
	c.m.Lock()
	defer c.m.Unlock()

	for _, user := range c.knownUsers {
		if user.username == username {
			return user
		}
	}
	user := &User{username, 0}
	c.knownUsers = append(c.knownUsers, user)
	return user
}

func (c *Chat) closeStaleConnections() {
	for key, conn := range c.connections {
		if conn.stale() {
			handle := common.Handle(key)
			c.Logout(handle, &common.Nothing{})
		} else {
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
		if u != user {
			c.writeTo(u, fmt.Sprintf("%s is here", user.username))
		}
	}
}

func (c *Chat) byeUser(user *User) {
	for _, u := range c.knownUsers {
		c.writeTo(u, fmt.Sprintf("%s leaved us", user.username))
	}
}

// Login returns a connection handle
func (c *Chat) Login(username string, out *common.Handle) error {
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
func (c *Chat) Logout(h common.Handle, out *common.Nothing) error {
	if conn, ok := c.connections[h.ToString()]; ok {
		if conn.user.connections == 1 {
			c.byeUser(conn.user)
		}
	}
	return c.closeConnection(&h)
}

// Write broadcasts a message to all users
func (c *Chat) Write(msg common.Message, out *common.Nothing) error {
	var conn *Connection

	conn, ok := c.connections[msg.Handle.ToString()]
	if !ok {
		return fmt.Errorf("Not authorized")
	}
	conn.refresh()

	for _, c := range c.connections {
		if c.user.username != conn.user.username {
			go c.write(fmt.Sprintf("%s: %s", conn.user.username, msg.Message))
		}
	}

	return nil
}

// Poll waits for a new message for a given connection
func (c *Chat) Poll(h common.Handle, out *string) error {
	var conn *Connection
	conn, ok := c.connections[h.ToString()]

	if !ok {
		return fmt.Errorf("%v connection closed", h)
	}

	defer conn.refresh()
	msg, err := conn.read(ReadTimeout)

	if err != nil {
		return err
	}

	*out = msg

	return nil
}
