package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

func summarizeThread(messages []slack.Message) string {
	summary := "*Here is a summary of the thread:*\n```"

	textMessages := make([]string, len(messages))

	for _, msg := range messages {
		user := msg.Username
		if user == "" {
			user = msg.User
		}
		textMessages = append(textMessages, user+": "+msg.Text)
	}

	prompt := `
	You are given a conversation from Slack where each line starts with the sender's name followed by their message. 
	Your task is to generate a concise and clear summary of the entire conversation, capturing the main points and outcomes in natural, plain language.
	Do not list every message individually.
	Focus on the key ideas, decisions, questions, and answers.
	Mention participants only if necessary for clarity.
	Keep your summary under 5 sentences unless the conversation is especially long or complex.
	Here is the conversation:
	{messages}	
	`

	prompt = strings.ReplaceAll(prompt, "{messages}", strings.Join(textMessages, "\n"))

	fmt.Println("prompt=", prompt)

	resp, err := getAIResponse(prompt)
	if err != nil {
		log.Printf("Error getting AI response: %v\n", err)
		return ""
	}

	summary += resp

	return summary + "```"
}

func getAIResponse(msg string) (string, error) {
	aiClient := openai.NewClient(openaiKey)
	ctx := context.Background()
	resp, err := aiClient.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: msg,
				},
			},
		},
	)
	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return "", err
	}

	if len(resp.Choices) > 0 {
		return resp.Choices[0].Message.Content, nil
	}

	return "No response from AI", nil
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
