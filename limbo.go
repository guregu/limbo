package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"code.google.com/p/go.crypto/bcrypt"
	"github.com/guregu/bbs"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var db *mgo.Database
var dbSession *mgo.Session
var config Config
var server *bbs.Server

var usernameLengthLimit = 32
var defaultRange = bbs.Range{1, 50}
var listThreadLimit = 50

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

func (client *limbo) Register(cmd bbs.RegisterCommand) (wm bbs.OKMessage, err error) {
	if !validateUsername(cmd.Username) {
		return bbs.OKMessage{}, errors.New("Invalid username.")
	}

	// see if we have a user already
	ct, err := db.C("users").FindId(strings.ToLower(cmd.Username)).Count()
	if err != nil || ct == 0 {
		pw, crypt_err := bcrypt.GenerateFromPassword([]byte(cmd.Password), bcrypt.DefaultCost)
		if crypt_err != nil {
			// seems like bcrypt freaks out if the password is less than 3 characters
			return bbs.OKMessage{}, errors.New("Password too short.")
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
		return bbs.OKMessage{}, errors.New("Username is already taken.")
	}
}

// TODO: change bbs package so this can return an error message
func (client *limbo) LogIn(cmd bbs.LoginCommand) bool {
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

func (client *limbo) LogOut(cmd bbs.LogoutCommand) bbs.OKMessage {
	client.user = nil
	return bbs.OK("logout")
}

func (client *limbo) Get(cmd bbs.GetCommand) (tm bbs.ThreadMessage, err error) {
	if !bson.IsObjectIdHex(cmd.ThreadID) {
		return bbs.ThreadMessage{}, errors.New("Invalid thread ID.")
	}

	id := bson.ObjectIdHex(cmd.ThreadID)
	var thread Thread
	err = db.C("threads").FindId(id).One(&thread)
	if err != nil {
		return bbs.ThreadMessage{}, errors.New(fmt.Sprintf("No such thread: %s", cmd.ThreadID))
	}

	if cmd.Range == (bbs.Range{}) && cmd.Token != "" {
		cmd.Range = thread.parseNextToken(cmd.Token)
	}

	return thread.toBBS(cmd.Range), nil
}

func (client *limbo) List(cmd bbs.ListCommand) (lm bbs.ListMessage, err error) {
	var date = time.Now().Add(time.Second * 5)
	if cmd.Token != "" {
		d, err := time.Parse(time.RFC3339, cmd.Token)
		if err == nil {
			date = d
		}
	}

	// TODO: use $slice with cmd.Range
	var threads Threads
	if cmd.Query == "" {
		db.C("threads").Find(bson.M{
			"lastpost": bson.M{
				"$lt": date,
			}}).Sort("-lastpost").Limit(listThreadLimit).All(&threads)
	} else {
		tags := parseTagExpr(cmd.Query)
		db.C("threads").Find(bson.M{
			"lastpost": bson.M{
				"$lt": date,
			},
			"tags": bson.M{
				"$in":  tags.include,
				"$nin": tags.exclude,
			}}).Sort("-lastpost").Limit(listThreadLimit).All(&threads)
	}
	msg := bbs.ListMessage{
		Command: "list",
		Type:    "thread",
		Query:   cmd.Query,
		Threads: threads.listing(),
	}
	// are there more threads? TODO: make sure there really are more?
	if len(threads) == listThreadLimit {
		msg.NextToken = threads[listThreadLimit-1].LastPost.Format(time.RFC3339)
	}
	return msg, nil
}

func (client *limbo) Reply(cmd bbs.ReplyCommand) (okm bbs.OKMessage, err error) {
	if !bson.IsObjectIdHex(cmd.To) {
		return bbs.OKMessage{}, errors.New("Invalid thread ID.")
	}
	id := bson.ObjectIdHex(cmd.To)
	var thread Thread
	err = db.C("threads").FindId(id).One(&thread)
	if err != nil {
		return bbs.OKMessage{}, errors.New("No such thread.")
	}

	if thread.Closed && !client.user.Admin {
		return bbs.OKMessage{}, errors.New("Can't reply to a closed thread.")
	}

	// TODO: deal with formatting
	post := Post{
		Author: client.user.Name,
		Date:   time.Now(),
		Text:   cmd.Text,
	}

	err = db.C("threads").UpdateId(id, bson.M{
		"$push": bson.M{"posts": &post},
		"$set":  bson.M{"lastpost": time.Now()}})
	if err != nil {
		return bbs.OKMessage{}, errors.New("DB error: couldn't add reply.")
	}

	return bbs.OK("reply"), nil
}

func (client *limbo) Post(cmd bbs.PostCommand) (okm bbs.OKMessage, err error) {
	if cmd.Title == "" {
		return bbs.OKMessage{}, errors.New("Thread title can't be blank.")
	}

	// TODO: deal with formatting, tags
	id := bson.NewObjectId()
	now := time.Now()
	thread := Thread{
		ID:       id,
		Title:    cmd.Title,
		Created:  now,
		LastPost: now,
		Creator:  client.user.Name,
		Tags:     cmd.Tags,
		Posts: []*Post{
			&Post{
				Author: client.user.Name,
				Date:   now,
				Text:   cmd.Text,
			},
		},
	}

	err = db.C("threads").Insert(&thread)
	if err == nil {
		return bbs.OKMessage{
			Command: "ok",
			ReplyTo: "post",
			Result:  id.Hex(),
		}, nil
	} else {
		log.Printf("New thread err: %s\n", err.Error())
		return bbs.OKMessage{}, errors.New("Couldn't post.")
	}
}

func (client *limbo) IsLoggedIn() bool {
	return client.user != nil
}

func (client *limbo) Hello() bbs.HelloMessage {
	msg := bbs.HelloMessage{
		Command:         "hello",
		Name:            config.BBS.Name,
		ProtocolVersion: 0,
		Description:     config.BBS.Desc,
		Options:         []string{"filter", "range", "tags"},
		Access: bbs.AccessInfo{
			GuestCommands: []string{"hello", "login", "logout", "register", "get", "list"},
			UserCommands:  []string{"post", "reply", "info"},
		},
		Formats:       []string{"html", "text"},
		Lists:         []string{"thread"},
		ServerVersion: "limbo 0.1",
		IconURL:       "/static/icon.png",
		DefaultRange:  defaultRange,
	}
	if config.Server.WS != "" {
		msg.Options = append(msg.Options, "realtime")
		msg.RealtimeURL = "ws://" + config.Server.Bind + config.Server.WS
	}
	return msg
}

func New() bbs.BBS {
	return new(limbo)
}

func main() {
	config = readConfig()

	dbSesh, err := mgo.Dial(config.DB.Addr)
	if err != nil {
		log.Fatalf("Couldn't connect to DB (%s): %s\n", config.DB.Addr, err.Error())
	}
	dbSession = dbSesh
	db = dbSession.DB(config.DB.Name)
	lpIndex := mgo.Index{
		Key:        []string{"-lastpost"},
		Unique:     false,
		DropDups:   false,
		Background: true,
		Sparse:     true,
	}
	err = db.C("threads").EnsureIndex(lpIndex)
	if err != nil {
		log.Fatalf("Couldn't make index: %s\n", err.Error())
	}

	log.Printf("Starting limbo (%s) at %s%s\n", config.BBS.Name, config.Server.Bind, config.Server.Path)

	server = bbs.NewServer(New)
	if config.WebClient.Index != "" {
		http.Handle("/", http.FileServer(http.Dir(config.WebClient.Index)))
		log.Printf("\t/ \t\t%s", config.WebClient.Index)
	}
	http.Handle(config.Server.Path, server)
	if config.Server.WS != "" {
		http.Handle(config.Server.WS, server.WS)
	}
	err = http.ListenAndServe(config.Server.Bind, nil)
	if err != nil {
		panic(err)
	}
}
