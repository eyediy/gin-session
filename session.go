package ginsession

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	redis "gopkg.in/redis.v4"

	"github.com/gin-gonic/gin"
	"github.com/gokyle/uuid"
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
	// key prefix used in store
	keyPrefix string
	// redis
	client *redis.Client
}

func (manager *SessionManager) sessionKey(sid string) string {
	return manager.keyPrefix + ":" + sid
}

// Save save session to store
func (manager *SessionManager) Save(session *Session) error {
	var (
		err   error
		bytes []byte
	)
	bytes, err = json.Marshal(session.Data)
	if err != nil {
		return err
	}
	statusCmd := manager.client.Set(manager.sessionKey(session.ID), string(bytes), time.Duration(manager.maxAge)*time.Second)
	if statusCmd.Err() != nil {
		return statusCmd.Err()
	}
	return nil
}

// GetSession get or generate a new session
func (manager *SessionManager) GetSession(sid string) *Session {
	var (
		err error
	)
	session := &Session{"", make(map[string]interface{}), manager}
	strCmd := manager.client.Get(manager.sessionKey(sid))
	if strCmd.Err() == nil {
		err = json.Unmarshal([]byte(strCmd.Val()), &session.Data)
		if err == nil {
			session.ID = sid
			return session
		}
	}
	//session.ID = betterguid.New()
	session.ID, err = uuid.GenerateV4String()
	if err != nil {
		session.ID = ""
		log.Print(err)
	}
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
	sessionManager.keyPrefix = p.GetString("session.keyPrefix", sessionManager.cookieName)

	// load storage config
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
		log.Panic(err)
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
