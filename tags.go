package main

import "labix.org/v2/mgo/bson"
import "fmt"
import "strings"

type Tag struct {
	ID       string `bson:"_id"` //name lowercased
	Name     string
	Desc     string
	Children []string
	/*
		Access 	   map[string]int
		AccessList map[string]int
	*/

	parents []*Tag
}

type tagMaster struct {
	tags map[string]*Tag
}

func loadTags() (tags []*Tag, tagMap map[string]*Tag) {
	tagMap = make(map[string]*Tag)
	db.C("tags").Find(bson.M{}).All(&tags)
	for _, t := range tags {
		tagMap[t.ID] = t
	}
	return
}

func children(tagMap map[string]*Tag, t string) []string {
	var nodes []string
	nodes = append(nodes, strings.ToLower(t))
	tag, ok := tagMap[strings.ToLower(t)]
	if ok {
		childs := tag.Children
		for _, child := range childs {
			kids := children(tagMap, strings.ToLower(child))
			for _, c := range kids {
				nodes = append(nodes, c)
			}
		}
	} else {
		fmt.Printf("invalid child tag: %s\n", t)
	}
	return nodes
}
