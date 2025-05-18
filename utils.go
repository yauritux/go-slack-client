package main

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

func summarizeThread(messages []slack.Message) string {
	summary := "*Here is a summary of the thread:*\n\n"

	textMessages := make([]string, len(messages))

	for _, msg := range messages {
		textMessages = append(textMessages, msg.Text)
	}

	summary += strings.Join(textMessages, "\n")

	return summary
}

func fetchThreadReplies(api *slack.Client, channelID, threadTs string) (msgs []slack.Message, hasMore bool, nextCursor string, err error) {
	params := &slack.GetConversationRepliesParameters{
		ChannelID: channelID,
		Timestamp: threadTs,
		Limit:     100,
	}

	return api.GetConversationReplies(params)
}

func getBotID(api *slack.Client) (string, error) {
	resp, err := api.AuthTest()
	if err != nil {
		log.Fatal(err)
		return "", err
	}

	return resp.BotID, nil
}

func sendMessage(api *slack.Client, event *slackevents.MessageEvent, text string) error {
	fmt.Println("trying to send message reply...")
	fmt.Println("User:", event.User, "event BotID:", event.BotID)
	if event.BotID != "" {
		return errors.New("bot message received, skip it")
	}
	if event.User == event.BotID {
		return errors.New("bot message received, skip it")
	}

	if event.ThreadTimeStamp != "" {
		_, timestamp, err := api.PostMessage(event.Channel,
			slack.MsgOptionText(text, false), slack.MsgOptionTS(event.ThreadTimeStamp))
		if err != nil {
			log.Fatalf("Error sending message reply: %s\n", err)
			return err
		}
		fmt.Printf("Message reply sent at %s\n", timestamp)
		return nil
	}

	_, timestamp, err := api.PostMessage(event.Channel, slack.MsgOptionText(text, false))
	if err != nil {
		log.Fatalf("Error sending message reply: %s\n", err)
		return err
	}

	fmt.Printf("Message reply sent at %s\n", timestamp)
	return nil
}
