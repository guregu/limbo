package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/guregu/bbs"
	"gopkg.in/mgo.v2/bson"
)

type User struct {
	ID       string `bson:"_id"` // username lower-cased
	Name     string
	Password []byte
	Regdate  time.Time
	Admin    bool
}

type Thread struct {
	ID       bson.ObjectId `bson:"_id,omitempty"`
	Title    string
	Creator  string
	Created  time.Time
	LastPost time.Time
	Posts    []*Post
	Tags     []string
	Sticky   bool
	Closed   bool
}

func (thread Thread) toBBS(r bbs.Range) bbs.ThreadMessage {
	if r.End == 0 {
		r = defaultRange
	}
	if r.End > len(thread.Posts) {
		r.End = len(thread.Posts)
	}
	msg := bbs.ThreadMessage{
		Command:  "msg",
		ID:       thread.threadID(),
		Title:    thread.Title,
		Format:   "markdown",
		Tags:     thread.Tags,
		Closed:   thread.Closed,
		Range:    r,
		Messages: thread.messages(r),
	}
	if thread.more(r) {
		msg.More = true
		msg.NextToken = thread.nextToken(r)
	}
	return msg
}

func (thread Thread) more(r bbs.Range) bool {
	return r.End < len(thread.Posts)
}

func (thread Thread) nextToken(r bbs.Range) string {
	return fmt.Sprintf("%d-", r.End+1)
}

func (thread Thread) threadID() string {
	//return base64.StdEncoding.EncodeToString([]byte(thread.ID))
	return thread.ID.Hex()
}

func (thread Thread) listing() bbs.ThreadListing {
	return bbs.ThreadListing{
		ID:        thread.threadID(),
		Title:     thread.Title,
		Author:    thread.Creator,
		Date:      thread.LastPost.Format(time.RFC3339),
		PostCount: len(thread.Posts),
		Tags:      thread.Tags,
		Sticky:    thread.Sticky,
		Closed:    thread.Closed,
	}
}

func (thread Thread) messages(r bbs.Range) []bbs.Message {
	var msgs []bbs.Message
	for i, v := range thread.Posts {
		if r != (bbs.Range{}) {
			if i+1 < r.Start {
				continue
			} else if i+1 > r.End {
				break
			}
		}
		msgs = append(msgs, bbs.Message{
			ID:     fmt.Sprintf("%s:%d", thread.threadID(), i+1),
			Author: v.Author,
			Date:   v.Date.Format(time.RFC3339),
			Text:   v.Text,
		})
	}
	return msgs
}

func (thread Thread) parseNextToken(token string) bbs.Range {
	split := strings.Split(token, "-")
	fmt.Printf("%#v", split)
	if len(split) != 2 {
		return defaultRange
	}
	start, err := strconv.Atoi(split[0])
	if err != nil {
		return defaultRange
	}
	if split[1] == "" {
		postcount := len(thread.Posts)
		if defaultRange.End+start > postcount {
			return bbs.Range{start, len(thread.Posts)}
		} else {
			return bbs.Range{start, start + defaultRange.End}
		}
	}
	end, err := strconv.Atoi(split[1])
	if err != nil {
		return defaultRange
	}
	return bbs.Range{start, end}
}

type Threads []*Thread

func (threads Threads) listing() []bbs.ThreadListing {
	list := make([]bbs.ThreadListing, 0, len(threads))
	for _, t := range threads {
		list = append(list, t.listing())
	}
	return list
}

type Post struct {
	Author string
	Date   time.Time
	Text   string
}

type query struct {
	include []string
	exclude []string
}

func parseTagExpr(expr string) query {
	last := ' '
	str := ""
	q := query{}
	for _, r := range expr {
		if r == '+' || r == '-' {
			switch last {
			case ' ':
				fallthrough
			case '+':
				q.include = append(q.include, str)
			case '-':
				q.exclude = append(q.exclude, str)
			}
			last = r
			str = ""
		} else {
			str = str + string(r)
		}
	}
	if str != "" {
		switch last {
		case ' ':
			fallthrough
		case '+':
			q.include = append(q.include, str)
		case '-':
			q.exclude = append(q.exclude, str)
		}
	}
	return q
}
