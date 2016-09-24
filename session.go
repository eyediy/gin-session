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

// SessionData .
type SessionData struct {
	LastUpdate int64
	Value      map[string]interface{}
}

// Session defines common session info
type Session struct {
	ID      string
	Data    SessionData
	manager *SessionManager
}

// Copy copy a session instance
// return *Session if succeed
// return nil otherwise
func (session *Session) Copy() *Session {
	newSession := new(Session)
	// ID
	newSession.ID = session.ID
	// Data
	bytes, err := json.Marshal(&session.Data)
	if err != nil {
		return nil
	}
	err = json.Unmarshal(bytes, &newSession.Data)
	if err != nil {
		return nil
	}
	// manager
	newSession.manager = session.manager
	return newSession
}

// Save save session to store
func (session *Session) Save() error {
	return session.manager.Save(session)
}

// Update force the update the session state
func (session *Session) Update() {
	session.Data.LastUpdate = time.Now().UnixNano()
}

// ShouldUpdate .
// check session state
// return true if session.Data.LastUpdate had been modified
// return true if elapsed time is as long as maxAge-maxAgeTolerance
func (session *Session) ShouldUpdate(oldSession *Session) bool {
	if session.Data.LastUpdate != oldSession.Data.LastUpdate {
		return true
	}
	return session.manager.shouldUpdate(session)
}

// SessionManager use to manage sessions
type SessionManager struct {
	// cookie
	cookieName      string
	maxAge          int
	maxAgeTolerance int
	path            string
	domain          string
	secure          bool
	httpOnly        bool
	// key prefix used in store
	keyPrefix string
	// redis
	client *redis.Client
}

// sessionKey
func (manager *SessionManager) sessionKey(sid string) string {
	return manager.keyPrefix + ":" + sid
}

// shouldUpdate .
// return true if time.Now().UnixNano() - session.Data.LastUpdate > maxAge
func (manager *SessionManager) shouldUpdate(session *Session) bool {
	tElapsed := time.Now().UnixNano() - session.Data.LastUpdate
	maxAgeTolerance := time.Duration(manager.maxAge-manager.maxAgeTolerance) * time.Second
	if tElapsed > int64(maxAgeTolerance) {
		return true
	}
	return false
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
	session.Update()
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
	session := &Session{
		"",
		SessionData{
			time.Now().UnixNano(),
			make(map[string]interface{}),
		},
		manager}
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
	sessionManager.maxAgeTolerance = p.GetInt("session.maxAgeTolerance", 0)
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

		// create session
		sid, err := c.Cookie(sessionManager.cookieName)
		if err != nil {
			log.Println(err)
		}
		session := sessionManager.GetSession(sid)
		if session != nil {
			c.Set("session", session)
		}

		// save a copy of session
		oldSession := session.Copy()
		if oldSession == nil {
			log.Println("[gin-session]: session.Copy() failed")
		}

		c.Next()

		// save session
		if session.ShouldUpdate(oldSession) {
			session.Save()
		}
	}
}
