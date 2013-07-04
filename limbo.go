package main

import "fmt"
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
	Admin    bool
}

type Thread struct {
	ID      bson.ObjectId `bson:"_id,omitempty"`
	Title   string
	Creator string
	Created time.Time
	Posts   []*Post
	Tags    []string
	Sticky  bool
	Closed  bool
}

type Post struct {
	Author string
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

func messages(thread *Thread, r *bbs.Range) []*bbs.Message {
	var msgs []*bbs.Message
	for i, v := range thread.Posts {
		if r != nil {
			if i+1 < r.Start {
				continue
			} else if i+1 > r.End {
				break
			}
		}
		msgs = append(msgs, &bbs.Message{
			ID:     fmt.Sprintf("%s:%d", thread.ID.Hex(), i+1),
			Author: v.Author,
			Date:   v.Date.String(),
			Text:   v.Text,
		})
	}
	return msgs
}

func (client *limbo) Get(cmd *bbs.GetCommand) (tm *bbs.ThreadMessage, errm *bbs.ErrorMessage) {
	// Possible improvement: use a prettier Thread ID (base64 instead of hex?)
	if !bson.IsObjectIdHex(cmd.ThreadID) {
		return nil, bbs.Error("get", "Invalid thread ID.")
	}

	id := bson.ObjectIdHex(cmd.ThreadID)
	var thread Thread
	err := db.C("threads").FindId(id).One(&thread)
	if err != nil {
		return nil, bbs.Error("get", fmt.Sprintf("No such thread: %s", cmd.ThreadID))
	}

	return &bbs.ThreadMessage{
		Command:  "msg",
		ID:       cmd.ThreadID,
		Title:    thread.Title,
		Format:   "markdown",
		Messages: messages(&thread, cmd.Range),
	}, nil
}

func listing(threads []*Thread) []*bbs.ThreadListing {
	var list []*bbs.ThreadListing
	for _, t := range threads {
		list = append(list, &bbs.ThreadListing{
			ID:        t.ID.Hex(),
			Title:     t.Title,
			Author:    t.Creator,
			Date:      t.Created.String(),
			PostCount: len(t.Posts),
			Tags:      t.Tags,
			Sticky:    t.Sticky,
			Closed:    t.Closed,
		})
	}
	return list
}

func (client *limbo) List(cmd *bbs.ListCommand) (lm *bbs.ListMessage, errm *bbs.ErrorMessage) {
	var threads []*Thread
	db.C("threads").Find(bson.M{}).Limit(50).All(&threads)
	return &bbs.ListMessage{
		Command: "list",
		Type:    "thread",
		Query:   cmd.Query,
		Threads: listing(threads),
	}, nil
}

func (client *limbo) BoardList(cmd *bbs.ListCommand) (blm *bbs.BoardListMessage, errm *bbs.ErrorMessage) {
	return nil, bbs.Error("list", "No boards!")
}

func (client *limbo) Reply(cmd *bbs.ReplyCommand) (okm *bbs.OKMessage, errm *bbs.ErrorMessage) {
	if !bson.IsObjectIdHex(cmd.To) {
		return nil, bbs.Error("reply", "Invalid thread ID.")
	}
	id := bson.ObjectIdHex(cmd.To)
	var thread Thread
	err := db.C("threads").FindId(id).One(&thread)
	if err != nil {
		return nil, bbs.Error("reply", "No such thread.")
	}

	if thread.Closed && !client.user.Admin {
		return nil, bbs.Error("reply", "Can't reply to a closed thread.")
	}

	// TODO: deal with formatting
	post := Post{
		Author: client.user.Name,
		Date:   time.Now(),
		Text:   cmd.Text,
	}

	err = db.C("threads").UpdateId(id, bson.M{"$push": bson.M{"posts": &post}})
	if err != nil {
		return nil, bbs.Error("reply", "DB error: couldn't add reply.")
	}

	return bbs.OK("reply"), nil
}

func (client *limbo) Post(cmd *bbs.PostCommand) (okm *bbs.OKMessage, errm *bbs.ErrorMessage) {
	if cmd.Title == "" {
		return nil, bbs.Error("post", "Thread title can't be blank.")
	}

	// TODO: deal with formatting, tags
	id := bson.NewObjectId()
	now := time.Now()
	thread := Thread{
		ID:      id,
		Title:   cmd.Title,
		Created: now,
		Creator: client.user.Name,
		Tags:    cmd.Tags,
		Posts: []*Post{
			&Post{
				Author: client.user.Name,
				Date:   now,
				Text:   cmd.Text,
			},
		},
	}

	err := db.C("threads").Insert(&thread)
	if err != nil {
		return &bbs.OKMessage{
			Command: "ok",
			ReplyTo: "post",
			Result:  id.Hex(),
		}, nil
	} else {
		log.Printf("New thread err: %s\n", err.Error())
		return nil, bbs.Error("post", "Couldn't post.")
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
			GuestCommands: []string{"hello", "login", "logout", "register", "get", "list"},
			UserCommands:  []string{"post", "reply", "info"},
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
