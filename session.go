package ginsession

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	redis "gopkg.in/redis.v4"

	"github.com/gin-gonic/gin"
	"github.com/gokyle/uuid"
	"github.com/magiconair/properties"
)

const (
	// SessionName .
	SessionName = "session"
	// 网络延迟,会话保活时间应该加上网络延迟
	sessionDelay = 5
)

// SessionData .
type SessionData struct {
	LastUpdate int                    `json:"u"`
	Value      map[string]interface{} `json:"v"`
}

// Session defines common session info
type Session struct {
	ID      string
	Data    SessionData
	Update  bool
	manager *SessionManager
}

// Copy copy a session instance
// return *Session if succeed
// return nil otherwise
func (session *Session) Copy() (*Session, error) {
	newSession := new(Session)
	bytes, err := json.Marshal(&session)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytes, &newSession)
	if err != nil {
		return nil, err
	}
	// unexported members
	newSession.manager = session.manager
	return newSession, nil
}

// Alloc generate new session ID
func (session *Session) Alloc() error {
	session.Update = true
	if session.ID == "" {
		var err error
		session.ID, err = uuid.GenerateV4String()
		if err != nil {
			return err
		}
	}
	return nil
}

// Expired 检查是否过期
func (session *Session) Expired() bool {
	return session.manager.expired(session)
}

// SaveCookie set cookie
func (session *Session) SaveCookie(c *gin.Context) {
	session.manager.SaveCookie(session, c)
}

// Save save session to store
func (session *Session) Save() error {
	return session.manager.Save(session)
}

// Destroy delete session
func (session *Session) Destroy() error {
	return session.manager.destroy(session)
}

// CheckTTL 检查会话存活时间
// return:
//	true - 需要更新
//	false - 不需要
func (session *Session) CheckTTL() bool {
	if session.Update {
		return true
	}
	return session.manager.checkTTL(session)
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
	// store
	ttl int
	// redis
	client *redis.Client
}

func (manager *SessionManager) getSessionDuration() int {
	if manager.ttl > 0 {
		return manager.ttl + sessionDelay
	}
	return manager.ttl
}

func (manager *SessionManager) expired(session *Session) bool {
	if manager.maxAge > 0 {
		tElapsed := time.Now().Unix() - int64(session.Data.LastUpdate)
		if tElapsed >= int64(manager.maxAge) {
			return true
		}
	}
	return false
}

func (manager *SessionManager) sessionKey(sid string) string {
	return manager.keyPrefix + ":" + sid
}

func (manager *SessionManager) checkTTL(session *Session) bool {
	if manager.ttl > 0 {
		tElapsed := time.Now().Unix() - int64(session.Data.LastUpdate)
		if tElapsed >= int64(manager.ttl) {
			return true
		}
	}
	return false
}

func (manager *SessionManager) destroy(session *Session) error {
	cmd := manager.client.Del(manager.sessionKey(session.ID))
	return cmd.Err()
}

// SaveCookie save cookie to client
func (manager *SessionManager) SaveCookie(session *Session, c *gin.Context) {
	err := session.Alloc()
	if err != nil {
		log.Print(err)
	}
	cookie := &http.Cookie{
		Name:     manager.cookieName,
		Value:    url.QueryEscape(session.ID),
		MaxAge:   0,
		Path:     manager.path,
		Domain:   manager.domain,
		Secure:   manager.secure,
		HttpOnly: manager.httpOnly,
	}
	http.SetCookie(c.Writer, cookie)
}

// Save save session to store
func (manager *SessionManager) Save(session *Session) error {
	var (
		bytes []byte
		err   error
	)
	session.Data.LastUpdate = int(time.Now().Unix())
	bytes, err = json.Marshal(&session.Data)
	if err != nil {
		return err
	}
	statusCmd := manager.client.Set(manager.sessionKey(session.ID),
		string(bytes),
		time.Duration(manager.getSessionDuration())*time.Second)
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
			int(time.Now().Unix()),
			make(map[string]interface{}),
		},
		false,
		manager}
	strCmd := manager.client.Get(manager.sessionKey(sid))
	if strCmd.Err() == nil {
		err = json.Unmarshal([]byte(strCmd.Val()), &session.Data)
		if err == nil {
			session.ID = sid
			return session
		}
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

	// TTL
	sessionManager.ttl = p.GetInt("session.store.ttl", 0)

	// load storage config

	host := p.GetString("session.store.host", "")
	port := p.GetInt("session.store.port", 6379)
	pass := p.GetString("session.store.pass", "")
	db := p.GetInt("session.store.db", 0)

	// create redis client
	addr := fmt.Sprintf("%s:%d", host, port)
	//log.Println(addr)
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

// GetSession return a pointer to ginsession.Session
func GetSession(c *gin.Context) *Session {
	customField, _ := c.Get(SessionName)
	return customField.(*Session)
}

// SessionMiddleware return a gin handler
// propfile - properties file used for customized configuration
func SessionMiddleware(propfile string) gin.HandlerFunc {

	sessionManager := NewSessionManager(propfile)

	return func(c *gin.Context) {

		// get or create session
		sid, _ := c.Cookie(sessionManager.cookieName)
		session := sessionManager.GetSession(sid)
		c.Set(SessionName, session)

		c.Next()

		// save session
		if session.Expired() {
			session.Destroy()
		} else {
			if session.CheckTTL() {
				err := session.Save()
				if err != nil {
					log.Print(err)
				}
			}
		}
	}
}
