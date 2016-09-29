package app

import (
	"fmt"
	"github.com/nlopes/slack"
	. "github.com/pivotal-sydney/whiteboardbot/model"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type WhiteboardApp struct {
	SlackClient SlackClient
	RestClient  RestClient
	Clock       Clock
	Store       Store
	EntryMap    map[string]EntryType
	CommandMap  map[string]func(input string, ev *slack.MessageEvent)
}

func NewWhiteboard(slackClient SlackClient, restClient RestClient, clock Clock, store Store) (whiteboard WhiteboardApp) {
	whiteboard = WhiteboardApp{SlackClient: slackClient, Clock: clock, RestClient: restClient}
	whiteboard.Store = store
	whiteboard.EntryMap = make(map[string]EntryType)
	whiteboard.CommandMap = make(map[string]func(input string, ev *slack.MessageEvent))
	whiteboard.init()
	return
}

func (whiteboard WhiteboardApp) init() {
	whiteboard.registerCommand("register", whiteboard.handleRegistrationCommand)
	whiteboard.registerCommand("?", whiteboard.handleUsageCommand)
	whiteboard.registerCommand("faces", whiteboard.handleFacesCommand)
	whiteboard.registerCommand("helps", whiteboard.handleHelpsCommand)
	whiteboard.registerCommand("interestings", whiteboard.handleInterestingsCommand)
	whiteboard.registerCommand("events", whiteboard.handleEventsCommand)
	whiteboard.registerCommand("name", whiteboard.handleUpdateNameTitleCommand)
	whiteboard.registerCommand("title", whiteboard.handleUpdateNameTitleCommand)
	whiteboard.registerCommand("body", whiteboard.handleUpdateBodyCommand)
	whiteboard.registerCommand("date", whiteboard.handleUpdateDateCommand)
	whiteboard.registerCommand("present", whiteboard.handlePresentCommand)
}

func (whiteboard WhiteboardApp) registerCommand(command string, callback func(input string, ev *slack.MessageEvent)) {
	whiteboard.CommandMap[command] = callback
}

func (whiteboard WhiteboardApp) ParseMessageEvent(ev *slack.MessageEvent) {
	input := getInputString(ev)
	input = whiteboard.replaceIdsWithNames(input)

	command, input := readNextCommand(input)
	if !matches(command, "/wb") && !matches(command, "wb") {
		return
	}
	whiteboard.HandleInput(input, ev)
}

func (whiteboard WhiteboardApp) HandleInput(input string, ev *slack.MessageEvent) {
	command, input := readNextCommand(input)
	whiteboard.handleCommand(command, input, ev)
}

func (whiteboard WhiteboardApp) handleCommand(command, input string, ev *slack.MessageEvent) {
	for key := range whiteboard.CommandMap {
		if matches(command, key) {
			callback := whiteboard.CommandMap[key]
			callback(input, ev)
			return
		}
	}
	whiteboard.handleDefault(input, ev)
}

func (whiteboard WhiteboardApp) handleFacesCommand(name string, ev *slack.MessageEvent) {
	whiteboard.handleCreateCommand(name, ev, NewFace)
}

func (whiteboard WhiteboardApp) handleHelpsCommand(title string, ev *slack.MessageEvent) {
	whiteboard.handleCreateCommand(title, ev, NewHelp)
}

func (whiteboard WhiteboardApp) handleInterestingsCommand(title string, ev *slack.MessageEvent) {
	whiteboard.handleCreateCommand(title, ev, NewInteresting)
}

func (whiteboard WhiteboardApp) handleEventsCommand(title string, ev *slack.MessageEvent) {
	whiteboard.handleCreateCommand(title, ev, NewEvent)
}

func (whiteboard WhiteboardApp) handleCreateCommand(title string, ev *slack.MessageEvent, createEntryCallback func(clock Clock, author string, title string, standup Standup) (entryType interface{})) {
	standup, slackUser, _, ok := whiteboard.getEntryDetails(ev)
	if !ok {
		return
	}
	if len(title) == 0 {
		whiteboard.handleMissingTitle(ev.Channel)
		return
	}

	entryType := createEntryCallback(whiteboard.Clock, slackUser.Author, title, standup).(EntryType)

	whiteboard.EntryMap[slackUser.Username] = entryType

	if ev.Upload {
		entryType.GetEntry().Body = fmt.Sprintf("%v\n<img src=\"%v\" style=\"max-width: 500px\">", ev.File.InitialComment.Comment, ev.File.Permalink)
	}

	whiteboard.validateAndPost(entryType, ev)
}

func (whiteboard WhiteboardApp) handleUpdateNameTitleCommand(title string, ev *slack.MessageEvent) {
	whiteboard.handleUpdateCommand(title, ev, func(entryType EntryType, title string) (finished bool) {
		if len(title) == 0 {
			whiteboard.SlackClient.PostMessage("Oi! The title/name can't be empty!", ev.Channel, THUMBS_DOWN)
			finished = true
		} else {
			entryType.GetEntry().Title = title
		}
		return
	})
}

func (whiteboard WhiteboardApp) handleUpdateBodyCommand(body string, ev *slack.MessageEvent) {
	whiteboard.handleUpdateCommand(body, ev, func(entryType EntryType, body string) (finished bool) {
		switch entryType.(type) {
		default:
			entryType.GetEntry().Body = body
		case Face:
			whiteboard.SlackClient.PostMessage("Face does not have a body! "+randomInsult(), ev.Channel, THUMBS_DOWN)
			finished = true
		}
		return
	})
}

func (whiteboard WhiteboardApp) handleUpdateDateCommand(date string, ev *slack.MessageEvent) {
	whiteboard.handleUpdateCommand(date, ev, func(entryType EntryType, input string) (finished bool) {
		if parsedDate, err := time.Parse(DATE_FORMAT, input); err == nil {
			entryType.GetEntry().Date = parsedDate.Format(DATE_FORMAT)
		} else {
			whiteboard.SlackClient.PostEntry(entryType.GetEntry(), ev.Channel, THUMBS_DOWN+"Date not set, use YYYY-MM-DD as date format\n")
			finished = true
		}
		return
	})
}

func (whiteboard WhiteboardApp) handleUpdateCommand(detail string, ev *slack.MessageEvent, updateCallback func(entryType EntryType, detail string) (finished bool)) {
	_, _, entryType, ok := whiteboard.getEntryDetails(ev)
	if !ok {
		return
	}
	if missingEntry(entryType) {
		handleMissingEntry(whiteboard.SlackClient, ev.Channel)
		return
	}

	if updateCallback(entryType, detail) {
		return
	}

	whiteboard.validateAndPost(entryType, ev)
}

func (whiteboard WhiteboardApp) handleRegistrationCommand(standupId string, ev *slack.MessageEvent) {
	standup, ok := whiteboard.RestClient.GetStandup(standupId)
	if !ok {
		handleStandupNotFound(whiteboard.SlackClient, standupId, ev.Channel)
		return
	}
	whiteboard.Store.SetStandup(ev.Channel, standup)
	whiteboard.SlackClient.PostMessage(fmt.Sprintf("Standup %v has been registered! You can now start creating Whiteboard entries!", standup.Title), ev.Channel, THUMBS_UP)
}

func (whiteboard WhiteboardApp) handleUsageCommand(_ string, ev *slack.MessageEvent) {
	whiteboard.SlackClient.PostMessageWithMarkdown(USAGE, ev.Channel, "")
}

func (whiteboard WhiteboardApp) handlePresentCommand(numDays string, ev *slack.MessageEvent) {
	standup, slackUser, _, ok := whiteboard.getEntryDetails(ev)
	if !ok {
		return
	}
	items, ok := whiteboard.RestClient.GetStandupItems(standup.Id)
	if !ok || items.Empty() {
		whiteboard.SlackClient.PostMessage("Hey, there's no entries in today's standup yet, why not add some?", ev.Channel, THUMBS_DOWN)
		return
	}

	if len(numDays) > 0 {
		numDaysInt, err := strconv.Atoi(numDays)
		if err == nil {
			items.Events = whiteboard.FilterOutOld(items.Events, numDaysInt, slackUser.TimeZone)
			items.Faces = whiteboard.FilterOutOld(items.Faces, numDaysInt, slackUser.TimeZone)
			items.Helps = whiteboard.FilterOutOld(items.Helps, numDaysInt, slackUser.TimeZone)
			items.Interestings = whiteboard.FilterOutOld(items.Interestings, numDaysInt, slackUser.TimeZone)
		}
	}
	whiteboard.SlackClient.PostMessage(items.String(), ev.Channel, "")
}

func (whiteboard WhiteboardApp) getEntryDetails(ev *slack.MessageEvent) (standup Standup, slackUser SlackUser, entryType EntryType, ok bool) {
	standup, ok = whiteboard.Store.GetStandup(ev.Channel)
	if !ok {
		handleNotRegistered(whiteboard.SlackClient, ev.Channel)
		return
	}

	slackUser = whiteboard.SlackClient.GetUserDetails(ev.User)
	entryType = whiteboard.EntryMap[slackUser.Username]
	return
}

func (whiteboard WhiteboardApp) handleDefault(_ string, ev *slack.MessageEvent) {
	_, slackUser, _, ok := whiteboard.getEntryDetails(ev)
	if !ok {
		return
	}
	_, userInput := readNextCommand(getInputString(ev))

	whiteboard.SlackClient.PostMessage(fmt.Sprintf("%v no you %v", slackUser.Username, userInput), ev.Channel, "")
}

func (whiteboard WhiteboardApp) validateAndPost(entryType EntryType, ev *slack.MessageEvent) {
	status := ""
	entry := entryType.GetEntry()
	if entryType.Validate() {
		if itemId, ok := PostEntryToWhiteboard(whiteboard.RestClient, entryType); ok {
			if len(entry.Id) == 0 {
				status = THUMBS_UP + "_Now go update the details. Need help?_ `wb ?`\n\n" + strings.ToUpper(entry.ItemKind) + "\n"
			} else {
				status = THUMBS_UP + strings.ToUpper(entry.ItemKind) + "\n"
			}
			entry.Id = itemId
		}
	}
	whiteboard.SlackClient.PostEntry(entry, ev.Channel, status)
}

func (whiteboard WhiteboardApp) handleMissingTitle(channel string) {
	whiteboard.SlackClient.PostMessageWithMarkdown("Hey, next time add a title along with your entry!\nLike this: `wb i My title`\nNeed help? Try `wb ?`", channel, THUMBS_DOWN)
}

func getInputString(ev *slack.MessageEvent) string {
	if ev.Upload {
		return ev.File.Title
	} else {
		return ev.Text
	}
}

func (whiteboard WhiteboardApp) FilterOutOld(entries []Entry, numDays int, userTimeZone string) []Entry {

	location, err := time.LoadLocation(userTimeZone)
	if err != nil {
		location = time.Local
	}

	entiriesFiltered := make([]Entry, 0)
	for _, entry := range entries {
		entryDate, err := time.Parse("2006-01-02", entry.Date)
		cutOff := whiteboard.Clock.Now().In(location).AddDate(0, 0, numDays)
		if err != nil || entryDate.In(location).Before(cutOff) || entryDate.In(location).Equal(cutOff) {
			entiriesFiltered = append(entiriesFiltered, entry)
		}
	}
	return entiriesFiltered
}

func (whiteboard WhiteboardApp) replaceIdsWithNames(input string) string {
	input = whiteboard.replaceUserIdsWithNames(input)
	input = whiteboard.replaceChannelIdsWithNames(input)
	return input
}

func (whiteboard WhiteboardApp) replaceUserIdsWithNames(input string) string {
	re := regexp.MustCompile("<@([a-zA-Z0-9]+)>")
	userIds := re.FindAllStringSubmatch(input, -1)

	for _, id := range userIds {
		userId := id[1]
		slackUser := whiteboard.SlackClient.GetUserDetails(userId)
		userName := "@" + slackUser.Username
		input = strings.Replace(input, id[0], userName, -1)
	}

	return input
}

func (whiteboard WhiteboardApp) replaceChannelIdsWithNames(input string) string {
	re := regexp.MustCompile("<#([a-zA-Z0-9]+)>")
	channelIds := re.FindAllStringSubmatch(input, -1)

	for _, id := range channelIds {
		channelId := id[1]
		slackChannel := whiteboard.SlackClient.GetChannelDetails(channelId)
		channelName := "#" + slackChannel.Name
		input = strings.Replace(input, id[0], channelName, -1)
	}

	return input
}
