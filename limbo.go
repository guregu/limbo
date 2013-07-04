package main

import _ "fmt"
import "time"
import "log"

import "code.google.com/p/go.crypto/bcrypt"
import "labix.org/v2/mgo"
import "labix.org/v2/mgo/bson"
import "github.com/guregu/bbs"

var db *mgo.Database
var dbSession *mgo.Session
var config *Config

type User struct {
	ID       bson.ObjectId `bson:"_id,omitempty"`
	Name     string
	Password []byte
}

type Thread struct {
	ID      bson.ObjectId `bson:"_id,omitempty"`
	Title   string
	Creator *User
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
	return nil, &bbs.ErrorMessage{
		Command: "error",
		ReplyTo: "get",
		Error:   "Not implemented yet, sorry!",
	}
}

func (client *limbo) BoardList(cmd *bbs.ListCommand) (blm *bbs.BoardListMessage, errm *bbs.ErrorMessage) {
	return nil, &bbs.ErrorMessage{
		Command: "error",
		ReplyTo: "get",
		Error:   "Not implemented yet, sorry!",
	}
}

func (client *limbo) Reply(cmd *bbs.ReplyCommand) (okm *bbs.OKMessage, errm *bbs.ErrorMessage) {
	return nil, &bbs.ErrorMessage{
		Command: "error",
		ReplyTo: "get",
		Error:   "Not implemented yet, sorry!",
	}
}

func (client *limbo) Post(cmd *bbs.PostCommand) (okm *bbs.OKMessage, errm *bbs.ErrorMessage) {
	return nil, &bbs.ErrorMessage{
		Command: "error",
		ReplyTo: "get",
		Error:   "Not implemented yet, sorry!",
	}
}

func (client *limbo) IsLoggedIn() bool {
	return client.user != nil
}

func (client *limbo) Hello() bbs.HelloMessage {
	return bbs.HelloMessage{
		Command:         "hello",
		Name:            config.Board.Name,
		ProtocolVersion: 0,
		Description:     config.Board.Desc,
		Options:         []string{"filter", "range", "tags"},
		Access: bbs.AccessInfo{
			GuestCommands: []string{"hello", "login", "logout"},
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
