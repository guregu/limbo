package main

import _ "fmt"
import "time"
import "log"
import "strings"

import "code.google.com/p/go.crypto/bcrypt"
import "labix.org/v2/mgo"
import "labix.org/v2/mgo/bson"
import "github.com/guregu/bbs"

var db *mgo.Database
var dbSession *mgo.Session
var config *Config

var usernameLengthLimit = 32

type User struct {
	ID       string `bson:"_id,omitempty"` // username lower-cased
	Name     string
	Password []byte
	Regdate  time.Time
}

type Thread struct {
	ID      bson.ObjectId `bson:"_id,omitempty"`
	Title   string
	Creator string
	Created time.Time
	Posts   []*Post
	Tags    []string
}

type Post struct {
	Name   string
	Author *User
	Date   time.Time
	Text   string
}

type limbo struct {
	user *User
}

// TODO: add options for no spaces, etc
func validateUsername(username string) bool {
	length := len(username)
	if length == 0 || length > usernameLengthLimit {
		return false
	}
	return true
}

func (client *limbo) Register(cmd *bbs.RegisterCommand) (wm *bbs.OKMessage, errm *bbs.ErrorMessage) {
	if !validateUsername(cmd.Username) {
		return nil, bbs.Error("register", "Invalid username.")
	}

	// see if we have a user already
	ct, err := db.C("users").FindId(strings.ToLower(cmd.Username)).Count()
	if err != nil || ct == 0 {
		pw, crypt_err := bcrypt.GenerateFromPassword([]byte(cmd.Password), bcrypt.DefaultCost)
		if crypt_err != nil {
			// seems like bcrypt freaks out if the password is less than 3 characters
			return nil, bbs.Error("register", "Password too short.")
		}

		usr := User{
			ID:       strings.ToLower(cmd.Username),
			Name:     cmd.Username,
			Password: pw,
			Regdate:  time.Now(),
		}

		db.C("users").Insert(&usr)
		return bbs.OK("register"), nil
	} else {
		return nil, bbs.Error("register", "Username is already taken.")
	}
}

// TODO: change bbs package so this can return an error message
func (client *limbo) LogIn(cmd *bbs.LoginCommand) bool {
	var user User
	err := db.C("users").Find(bson.M{"name": cmd.Username}).One(&user)
	if err != nil {
		return false
	}

	err = bcrypt.CompareHashAndPassword(user.Password, []byte(cmd.Password))
	if err != nil {
		return false
	} else {
		client.user = &user
		return true
	}
}

func (client *limbo) LogOut(cmd *bbs.LogoutCommand) *bbs.OKMessage {
	// TODO: session handling
	client.user = nil
	return &bbs.OKMessage{
		Command: "ok",
	}
}

func (client *limbo) Get(cmd *bbs.GetCommand) (tm *bbs.ThreadMessage, errm *bbs.ErrorMessage) {
	return nil, &bbs.ErrorMessage{
		Command: "error",
		ReplyTo: "get",
		Error:   "Not implemented yet, sorry!",
	}
}

func (client *limbo) List(cmd *bbs.ListCommand) (lm *bbs.ListMessage, errm *bbs.ErrorMessage) {
	var threads []*Thread
	db.C("threads").Find(bson.M{}).Limit(50).All(&threads)
	return &bbs.ListMessage{
		Command: "list",
		Type:    "thread",
		Query:   cmd.Query,
		Threads: threads,
	}, nil
}

func (client *limbo) BoardList(cmd *bbs.ListCommand) (blm *bbs.BoardListMessage, errm *bbs.ErrorMessage) {
	return nil, &bbs.ErrorMessage{
		Command: "error",
		ReplyTo: "list",
		Error:   "No boards!",
	}
}

func (client *limbo) Reply(cmd *bbs.ReplyCommand) (okm *bbs.OKMessage, errm *bbs.ErrorMessage) {
	return nil, &bbs.ErrorMessage{
		Command: "error",
		ReplyTo: "reply",
		Error:   "Not implemented yet, sorry!",
	}
}

func (client *limbo) Post(cmd *bbs.PostCommand) (okm *bbs.OKMessage, errm *bbs.ErrorMessage) {
	return nil, &bbs.ErrorMessage{
		Command: "error",
		ReplyTo: "post",
		Error:   "Not implemented yet, sorry!",
	}
}

func (client *limbo) IsLoggedIn() bool {
	return client.user != nil
}

func (client *limbo) Hello() bbs.HelloMessage {
	return bbs.HelloMessage{
		Command:         "hello",
		Name:            config.BBS.Name,
		ProtocolVersion: 0,
		Description:     config.BBS.Desc,
		Options:         []string{"filter", "range", "tags"},
		Access: bbs.AccessInfo{
			GuestCommands: []string{"hello", "login", "logout", "register"},
			UserCommands:  []string{"get", "list", "post", "reply", "info"},
		},
		Formats:       []string{"html", "text"},
		Lists:         []string{"thread"},
		ServerVersion: "limbo 0.1",
		IconURL:       "/static/icon.png",
		DefaultRange:  &bbs.Range{1, 50},
	}
}

func main() {
	var cfg = readConfig()
	config = &cfg

	dbSesh, err := mgo.Dial(config.DB.Addr)
	if err != nil {
		log.Fatalf("Couldn't connect to DB (%s): %s\n", config.DB.Addr, err.Error())
	}
	dbSession = dbSesh
	db = dbSession.DB(config.DB.Name)

	bbs.Serve(config.Server.Bind, cfg.Server.Path, func() bbs.BBS {
		return new(limbo)
	})
}
