package ginsession_test

import (
	"encoding/json"
	"testing"

	ginsession "github.com/eyediy/gin-session"
)

var (
	sessionManager *ginsession.SessionManager
)

func init() {
	sessionManager = ginsession.NewSessionManager("session_test.properties")
}

type AAA struct {
	ID   int
	Name string
	a    int
	b    int
}

func jsonInterface(bytes []byte, ss *map[string]interface{}) error {
	return json.Unmarshal(bytes, ss)
}

func Test_json(t *testing.T) {
	Data := make(map[string]interface{})
	t.Log(Data)
	Data["abc"] = 1111
	Data["ccb"] = AAA{22, "aaa", 22, 33}
	Data["kele"] = "ssss"
	bytes, err := json.Marshal(Data)
	if err != nil {
		t.Error(err)
	}
	t.Log(bytes)

	var dd interface{}
	dd = &AAA{22, "aaa", 22, 33}
	t.Log(dd)
	bytes, err = json.Marshal(dd)
	if err != nil {
		t.Error(err)
	}
	t.Log(bytes)

	var ss map[string]interface{}
	t.Log(ss)
	//var ss interface{}
	err = jsonInterface(bytes, &ss)
	//err = json.Unmarshal(bytes, &ss)
	if err != nil {
		t.Error(err)
	}
	t.Log(ss)
}

func Test_sessionAccess(t *testing.T) {
	var (
		err error
	)
	session := sessionManager.GetSession("")
	if session.ID != "" {
		t.Error("session.ID should be empty")
	}
	err = session.Alloc()
	err = session.Save()
	if err != nil {
		t.Error(err)
	}
	oSession, _ := session.Copy()
	t.Log(oSession)
	session = sessionManager.GetSession(oSession.ID)
	if session.ID != oSession.ID {
		t.Error("cannot get session")
	}
	t.Log(session)
	err = session.Destroy()
	if err != nil {
		t.Error(err)
	}
	session = sessionManager.GetSession(oSession.ID)
	t.Log(session)
	if session.ID != "" {
		t.Error("session should be empty")
	}
}
