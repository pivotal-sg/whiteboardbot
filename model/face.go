package model

import (
	"fmt"
)

type Face struct{ *Entry }

func NewFace(clock Clock, author string) (face Face) {
	face = Face{NewEntry(clock, author)}
	return
}

func (face Face) String() string {
	return fmt.Sprintf("faces\n  *name: %v\n  date: %v", face.Title, face.Time.Format("2006-01-02"))
}

func (face Face) MakeCreateRequest() (request WhiteboardRequest) {
	request = face.Entry.MakeCreateRequest()
	request.Item.Kind = "New face"
	request.Commit = "Create New Face"
	return
}

func (face Face) MakeUpdateRequest() (request WhiteboardRequest) {
	request = face.Entry.MakeUpdateRequest()
	request.Item.Kind = "New face"
	request.Commit = "Update New Face"
	return
}
