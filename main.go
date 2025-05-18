package main

import (
	"os"

	_ "github.com/joho/godotenv/autoload"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/slack-go/slack"
)

var signingSecret string
var verificationToken string
var openaiKey string
var api *slack.Client

func main() {
	slackToken := os.Getenv("SLACK_TOKEN")
	slackAppToken := os.Getenv("SLACK_APP_TOKEN")
	verificationToken = os.Getenv("SLACK_VERIFICATION_TOKEN")
	signingSecret = os.Getenv("SLACK_SIGNING_SECRET")
	openaiKey = os.Getenv("OPEN_AI_KEY")
	port := ":" + os.Getenv("SERVER_PORT")
	if slackToken == "" {
		panic("missing SLACK_TOKEN")
	}
	if slackAppToken == "" {
		panic("missing SLACK_APP_TOKEN")
	}
	if verificationToken == "" {
		panic("missing SLACK_VERIFICATIONT_TOKEN")
	}
	if signingSecret == "" {
		panic("missing SLACK_SIGNING_SECRET")
	}
	if openaiKey == "" {
		panic("missing OPEN_AI_KEY")
	}
	if port == "" {
		port = ":7001"
	}

	api = slack.New(slackToken, slack.OptionAppLevelToken(slackAppToken))

	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.POST("/slack/events", slackEventHandler)
	e.POST("/slack/commands", slackCommandHandler)
	e.POST("/slack/interactions", slackInteractionHandler)
	e.Logger.Fatal(e.Start(port))
}
