package spec

import (
	"github.com/nlopes/slack"
	"time"
	"github.com/pivotal-sydney/whiteboardbot/model"
)

type MockSlackClient struct {
	PostMessageCalled bool
}

func (client *MockSlackClient) PostMessage(channel, text string, params slack.PostMessageParameters) (string, string, error) {
	client.PostMessageCalled = true
	return "channel", "timestamp", nil
}

func (client *MockSlackClient) GetUserInfo(user string) (*slack.User, error) {
	User := slack.User{}
	User.Name = "aleung"
	return &User, nil
}

type MockClock struct{}

func (clock MockClock) Now() time.Time {
	return time.Date(2015, 1, 2, 0, 0, 0, 0, time.UTC)
}

type MockRestClient struct {
	PostCalledCount int
	Request         model.WhiteboardRequest
}

func (client *MockRestClient) Post(request model.WhiteboardRequest) (itemId string, ok bool) {
	client.PostCalledCount++
	client.Request = request
	ok = true
	itemId = "1"
	return
}