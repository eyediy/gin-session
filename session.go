package ginsession

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	redis "gopkg.in/redis.v4"

	guid "github.com/bsm/go-guid"
	"github.com/gin-gonic/gin"
	"github.com/magiconair/properties"
)

// Session defines common session info
type Session struct {
	ID      string
	Data    map[string]interface{}
	manager *SessionManager
}

// Save save session to store
func (session *Session) Save() error {
	return session.manager.Save(session)
}

// SessionManager use to manage sessions
type SessionManager struct {
	// cookie
	cookieName string
	maxAge     int
	path       string
	domain     string
	secure     bool
	httpOnly   bool
	// redis
	client *redis.Client
}

// Save save session to store
func (manager *SessionManager) Save(session *Session) error {

	return errors.New("Failed to save session")
}

// GetSession get or generate a new session
func (manager *SessionManager) GetSession(sid string) *Session {
	session := &Session{"", make(map[string]interface{}), manager}
	strCmd := manager.client.Get(sid)
	log.Fatal(strCmd)
	if strCmd.Err() == nil {
		err := json.Unmarshal([]byte(strCmd.Val()), &session.Data)
		if err != nil {
			session.ID = sid
			return session
		}
	}
	g2 := guid.New128()
	session.ID = hex.EncodeToString(g2.Bytes())
	return session
}

// NewSessionManager create an instance of SessionManager
func NewSessionManager(propfile string) *SessionManager {
	sessionManager := new(SessionManager)

	p := properties.MustLoadFile(propfile, properties.UTF8)

	// load session config
	sessionManager.cookieName = p.GetString("session.cookieName", "gin-session")
	sessionManager.maxAge = p.GetInt("session.maxAge", 0)
	sessionManager.path = p.GetString("session.path", "/")
	sessionManager.domain = p.GetString("session.domain", "")
	sessionManager.secure = p.GetBool("session.secure", false)
	sessionManager.httpOnly = p.GetBool("session.httpOnly", false)

	// load redis config
	host := p.GetString("session.store.host", "")
	port := p.GetInt("session.store.port", 6379)
	pass := p.GetString("session.store.pass", "")
	db := p.GetInt("session.store.db", 0)

	// create redis client
	addr := fmt.Sprintf("%s:%d", host, port)
	log.Println(addr)
	sessionManager.client = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: pass,
		DB:       db,
	})
	_, err := sessionManager.client.Ping().Result()
	if err != nil {
		panic(err)
	}

	return sessionManager
}

// SessionMiddleware return a gin handler
// propfile - properties file used for customized configuration
func SessionMiddleware(propfile string) gin.HandlerFunc {

	sessionManager := NewSessionManager(propfile)

	return func(c *gin.Context) {
		t := time.Now()

		// Set example variable
		c.Set("session", "12345")

		// before request
		sid, err := c.Cookie(sessionManager.cookieName)
		if err != nil {
			log.Println(err)
		}
		session := sessionManager.GetSession(sid)
		if session != nil {
			c.Set("session", session)
		}

		c.Next()

		// after request
		latency := time.Since(t)
		log.Print(latency)

		// access the status we are sending
		status := c.Writer.Status()
		log.Println(status)
	}
}
