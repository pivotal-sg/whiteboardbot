package app

import (
	"github.com/nlopes/slack"
	"strings"
	"fmt"
	. "github.com/xtreme-andleung/whiteboardbot/model"
	. "github.com/xtreme-andleung/whiteboardbot/rest"
	"time"
)

var entryMap = make(map[string]EntryType)

type SlackClient interface {
	PostMessage(channel, text string, params slack.PostMessageParameters) (string, string, error)
	GetUserInfo(user string) (*slack.User, error)
}

func ParseMessageEvent(slackClient SlackClient, restClient RestClient, clock Clock, ev *slack.MessageEvent) (username string, message string) {
	if strings.HasPrefix(strings.ToLower(ev.Text), "wb ") {
		user, err := slackClient.GetUserInfo(ev.User)
		if err != nil {
			fmt.Printf("%v, %v", ev.User, err)
			return
		}
		username = user.Name
		message = ev.Text[3:]

		entryType, ok := entryMap[username]
		if !ok {
			entryMap[username] = &Entry{}
			entryType = entryMap[username]
		}

		index := strings.Index(message, " ")
		if index == -1 {
			index = len(message)
		}

		keyword := strings.ToLower(message[:index])
		switch {
		case matches(keyword, "faces"):
			entryType = NewFace(clock, username)
			entryMap[username] = entryType
			populateEntry(message, index, entryType)
		case matches(keyword, "interestings"):
			entryType = NewInteresting(clock, username)
			entryMap[username] = entryType
			populateEntry(message, index, entryType)
		case matches(keyword, "helps"):
			entryType = NewHelp(clock, username)
			entryMap[username] = entryType
			populateEntry(message, index, entryType)
		case matches(keyword, "events"):
			entryType = NewEvent(clock, username)
			entryMap[username] = entryType
			populateEntry(message, index, entryType)
		case matches(keyword, "name"):
			fallthrough
		case matches(keyword, "title"):
			entryType.GetEntry().Title = message[index + 1:]
		case matches(keyword, "body"):
			entryType.GetEntry().Body = message[index + 1:]
		case matches(keyword, "date"):
			parsedDate, err := time.Parse("2006-01-02", message[index + 1:])
			if err != nil {
				message = entryType.String() + "\nDate not set, use YYYY-MM-DD as date format"
				slackClient.PostMessage(ev.Channel, message, slack.PostMessageParameters{})
				return
			} else {
				entryType.GetEntry().Date = parsedDate
			}
		default:
			message = fmt.Sprintf("%v no you %v", user.Name, message)
			slackClient.PostMessage(ev.Channel, message, slack.PostMessageParameters{})
			return
		}

		message = entryType.String()
		if entryType.Validate() {
			var request = createRequest(entryType, isExistingEntry(entryType.GetEntry()))
			itemId, ok := restClient.Post(request)
			if ok {
				if isExistingEntry(entryType.GetEntry()) {
					message += "\nitem updated"
				} else {
					message += "\nitem created"
					entryType.GetEntry().Id = itemId
				}
			}
		}
		fmt.Printf("Posting message: %v", message)
		slackClient.PostMessage(ev.Channel, message, slack.PostMessageParameters{})
	}
	return
}

func matches(keyword string, command string) bool {
	return len(keyword) <= len(command) && command[:len(keyword)] == keyword
}

func isExistingEntry(entry *Entry) bool {
	return entry != nil && len(entry.Id) > 0
}

func createRequest(entryType EntryType, existingEntry bool) (request WhiteboardRequest) {
	if existingEntry {
		request = entryType.MakeUpdateRequest()
	} else {
		request = entryType.MakeCreateRequest()
	}
	return
}

func populateEntry(message string, index int, entryType EntryType) {
	entryType.GetEntry().Title = strings.TrimPrefix(message[index:], " ")
}