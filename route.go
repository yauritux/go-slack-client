package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

func slackInteractionHandler(c echo.Context) error {
	var payload slack.InteractionCallback
	if err := json.Unmarshal([]byte(c.FormValue("payload")), &payload); err != nil {
		log.Printf("Error parsing payload: %v\n", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	log.Printf("Received payload: %+v\n", payload)

	// only handle message shortcuts
	if payload.Type != slack.InteractionTypeMessageAction {
		return c.NoContent(http.StatusOK)
	}

	if payload.CallbackID != "summarize_thread" {
		log.Printf("Unknown Callback ID: %s\n", payload.CallbackID)
		return c.NoContent(http.StatusOK)
	}

	channelID := payload.Channel.ID
	user := payload.User.ID
	threadTs := payload.Message.ThreadTimestamp
	if threadTs == "" {
		threadTs = payload.Message.Timestamp //fallback to parent message ts
	}

	replies, _, _, err := fetchThreadReplies(api, channelID, threadTs)
	if err != nil {
		log.Printf("Failed to fetch thread replies: %v", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	slackEvent := &slackevents.AppMentionEvent{
		Channel:         channelID,
		Type:            payload.Message.Type,
		User:            user,
		Text:            payload.Message.Text,
		ThreadTimeStamp: payload.Message.ThreadTimestamp,
		TimeStamp:       payload.Message.Timestamp,
	}

	summary := summarizeThread(replies)

	if summary == "" {
		return c.NoContent(http.StatusOK)
	}

	if err = sendMessage(api, slackEvent, summary); err != nil {
		fmt.Printf("Error sending message reply: %v\n", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}

func slackCommandHandler(c echo.Context) error {
	req := c.Request()

	if !strings.Contains(req.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		return c.String(http.StatusBadRequest, "Expected form-urlencoded data")
	}

	if err := req.ParseForm(); err != nil {
		return c.String(http.StatusBadRequest, "Error parsing form data")
	}

	cmd := req.FormValue("command")
	text := req.FormValue("text")
	user := req.FormValue("user_name")
	thread := req.FormValue("thread_ts")

	log.Printf("Received command: %s %s from @%s\n", cmd, text, user)

	botID, err := getBotID(api)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	if thread != "" {
		if user != botID {
			log.Printf("Message in thread: %s\n", thread)
			return c.JSON(http.StatusOK, map[string]string{
				"text": fmt.Sprintf("Hi %s! You used `%s %s in thread %s`\n", user, cmd, text, thread),
			})
		}
		return c.NoContent(http.StatusOK)
	}

	if user != botID {
		return c.JSON(http.StatusOK, map[string]string{
			"text": fmt.Sprintf("Hi @%s! You used `%s %s`", user, cmd, text),
		})
	}

	return c.NoContent(http.StatusOK)
}

func slackEventHandler(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	sv, err := slack.NewSecretsVerifier(c.Request().Header, signingSecret)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if _, err = sv.Write(body); err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	if err = sv.Ensure(); err != nil {
		return c.NoContent(http.StatusUnauthorized)
	}

	slackEvent, err := slackevents.ParseEvent(
		json.RawMessage(body),
		slackevents.OptionVerifyToken(&slackevents.TokenComparator{
			VerificationToken: verificationToken,
		}))
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	if slackEvent.Type == slackevents.URLVerification {
		var r *slackevents.ChallengeResponse
		err = json.Unmarshal([]byte(body), &r)
		if err != nil {
			return c.NoContent(http.StatusInternalServerError)
		}
		return c.String(http.StatusOK, r.Challenge)
	}

	if slackEvent.Type == slackevents.CallbackEvent {
		botID, err := getBotID(api)
		if err != nil {
			return c.NoContent(http.StatusInternalServerError)
		}

		switch event := slackEvent.InnerEvent.Data.(type) {
		// case *slackevents.MessageEvent:
		case *slackevents.AppMentionEvent:
			fmt.Println("AppMentioned by User=", event.User, "botID=", botID, "message=", event.Text, "event.BotID=", event.BotID)

			if event.BotID != "" {
				return c.NoContent(http.StatusOK)
			}

			resp, err := getAIResponse(event.Text)

			if err != nil {
				return c.NoContent(http.StatusInternalServerError)
			}

			if err = sendMessage(api, event, resp); err != nil {
				return c.NoContent(http.StatusInternalServerError)
			}
		}
	}

	return c.NoContent(http.StatusOK)
}
