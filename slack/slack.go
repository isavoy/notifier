package slack

import (
	"fmt"
	"strconv"
	"time"

	"github.com/moira-alert/notifier"

	"github.com/nlopes/slack"
	"github.com/op/go-logging"
)

var log *logging.Logger

// Sender implements moira sender interface via slack
type Sender struct {
	APIToken string
	FrontURI string
}

//Init read yaml config
func (sender *Sender) Init(senderSettings map[string]string, logger *logging.Logger) error {
	sender.APIToken = senderSettings["api_token"]
	if sender.APIToken == "" {
		return fmt.Errorf("Can not read slack api_token from config")
	}
	log = logger
	sender.FrontURI = senderSettings["front_uri"]
	return nil
}

//SendEvents implements Sender interface Send
func (sender *Sender) SendEvents(events []notifier.EventData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
	api := slack.New(sender.APIToken)

	var message string
	if len(events) == 1 {
		message = fmt.Sprintf("*%s* ", events[0].State)
	} else {
		currentValue := make(map[string]int)
		for _, event := range events {
			currentValue[event.State]++
		}
		allStates := [...]string{"OK", "WARN", "ERROR", "NODATA", "TEST"}
		for _, state := range allStates {
			if currentValue[state] > 0 {
				message = fmt.Sprintf("%s *%s*", message, state)
			}
		}
	}

	for _, tag := range trigger.Tags {
		message += "[" + tag + "]"
	}
	message += fmt.Sprintf(" <%s/#/events/%s|%s>\n```", sender.FrontURI, events[0].TriggerID, trigger.Name)

	icon := fmt.Sprintf("%s/public/fav72_ok.png", sender.FrontURI)
	for _, event := range events {
		if event.State != "OK" {
			icon = fmt.Sprintf("%s/public/fav72_error.png", sender.FrontURI)
		}
		value := strconv.FormatFloat(event.Value, 'f', -1, 64)
		message += fmt.Sprintf("\n%s: %s = %s (%s to %s)", time.Unix(event.Timestamp, 0).Format("15:04"), event.Metric, value, event.OldState, event.State)
	}

	message += "```"

	if throttled {
		message += "\nPlease, *fix your system or tune this trigger* to generate less events."
	}

	log.Debug("Calling slack with message body %s", message)

	params := slack.PostMessageParameters{
		Username: "Moira",
		IconURL:  icon,
	}

	channelID, _, err := api.PostMessage(contact.Value, message, params)
	if err != nil {
		return fmt.Errorf("Failed to send message to slack channel %s: %s", channelID, err.Error())
	}
	return nil
}
