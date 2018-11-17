package apisession

import (
	"net/http"
	"strings"

	"github.com/go-apibox/api"
	"github.com/go-apibox/session"
	"github.com/go-apibox/utils"
)

type Session struct {
	app      *api.App
	disabled bool
	inited   bool

	store         *session.CookieStore
	sessionName   string
	sessionKey    string
	actionMatcher *utils.Matcher
}

func NewSession(app *api.App) *Session {
	app.Error.RegisterGroupErrors("session", ErrorDefines)

	sess := new(Session)
	sess.app = app

	cfg := app.Config
	disabled := cfg.GetDefaultBool("apisession.disabled", false)
	sess.disabled = disabled
	if disabled {
		return sess
	}

	sess.init()
	return sess
}

func (s *Session) init() {
	if s.inited {
		return
	}

	app := s.app
	cfg := app.Config
	authKey := cfg.GetDefaultString("apisession.auth_key", "default.authed")
	actionWhitelist := cfg.GetDefaultStringArray("apisession.actions.whitelist", []string{"*"})
	actionBlacklist := cfg.GetDefaultStringArray("apisession.actions.blacklist", []string{})

	parts := strings.SplitN(authKey, ".", 2)
	if len(parts) != 2 {
		parts = []string{"default", parts[0]}
	}
	sessionName := parts[0]
	sessionKey := parts[1]

	matcher := utils.NewMatcher()
	matcher.SetWhiteList(actionWhitelist)
	matcher.SetBlackList(actionBlacklist)

	store, err := app.SessionStore()
	if err != nil {
		app.Logger.Error("(apisession) cookie store init failed, session not enabled: %s", err.Error())
	}

	s.store = store
	s.sessionName = sessionName
	s.sessionKey = sessionKey
	s.actionMatcher = matcher
	s.inited = true
}

func (s *Session) ServeHTTP(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if s.disabled {
		next(w, r)
		return
	}

	c, err := api.NewContext(s.app, w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// check if session is enable
	if s.sessionName == "" {
		next(w, r)
		return
	}

	// check if action not required session check
	action := c.Input.GetAction()
	if !s.actionMatcher.Match(action) {
		next(w, r)
		return
	}

	// check session
	if s.store == nil {
		api.WriteResponse(c, c.Error.NewGroupError("session", errorSessionInitFailed))
		return
	}
	session, err := s.store.Get(r, s.sessionName)
	if err != nil {
		api.WriteResponse(c, c.Error.NewGroupError("session", errorSessionGetFailed))
		return
	}
	authed, ok := session.Values[s.sessionKey]
	if !ok {
		api.WriteResponse(c, c.Error.NewGroupError("session", errorSessionNotAuthed))
		return
	}
	isAuthed, ok := authed.(bool)
	if !ok || !isAuthed {
		api.WriteResponse(c, c.Error.NewGroupError("session", errorSessionNotAuthed))
		return
	}

	// next middleware
	next(w, r)
}

// Enable enable the middle ware.
func (s *Session) Enable() {
	s.disabled = false
	s.init()
}

// Disable disable the middle ware.
func (s *Session) Disable() {
	s.disabled = true
}
