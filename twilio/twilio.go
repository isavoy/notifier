package twilio

import (
	"bytes"
	"fmt"
	"net/url"
	"strconv"
	"time"

	twilio "github.com/carlosdp/twiliogo"
	"github.com/moira-alert/notifier"
	"github.com/op/go-logging"
)

type sendEventsTwilio interface {
	SendEvents(events notifier.EventsData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error
}

type twilioSender struct {
	client       *twilio.TwilioClient
	APIFromPhone string
	log          *logging.Logger
}

type twilioSenderSms struct {
	twilioSender
}

type twilioSenderVoice struct {
	twilioSender
	voiceURL      string
	appendMessage bool
}

func (smsSender *twilioSenderSms) SendEvents(events notifier.EventsData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
	var message bytes.Buffer

	state := events.GetSubjectState()
	tags := trigger.GetTags()

	message.WriteString(fmt.Sprintf("%s %s %s (%d)\n", state, trigger.Name, tags, len(events)))

	for _, event := range events {
		value := strconv.FormatFloat(event.Value, 'f', -1, 64)
		message.WriteString(fmt.Sprintf("\n%s: %s = %s (%s to %s)", time.Unix(event.Timestamp, 0).Format("15:04"), event.Metric, value, event.OldState, event.State))
		if len(event.Message) > 0 {
			message.WriteString(fmt.Sprintf(". %s", event.Message))
		}
	}

	if len(events) > 5 {
		message.WriteString(fmt.Sprintf("\n\n...and %d more events.", len(events)-5))
	}

	if throttled {
		message.WriteString("\n\nPlease, fix your system or tune this trigger to generate less events.")
	}

	smsSender.log.Debugf("Calling twilio sms api to phone %s and message body %s", contact.Value, message.String())
	twilioMessage, err := twilio.NewMessage(smsSender.client, smsSender.APIFromPhone, contact.Value, twilio.Body(message.String()))

	if err != nil {
		return fmt.Errorf("Failed to send message to contact %s: %s", contact.Value, err)
	}

	smsSender.log.Debugf(fmt.Sprintf("message send to twilio with status: %s", twilioMessage.Status))

	return nil
}

func (voiceSender *twilioSenderVoice) SendEvents(events notifier.EventsData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
	voiceURL := voiceSender.voiceURL
	if voiceSender.appendMessage {
		voiceURL += url.QueryEscape(fmt.Sprintf("Hi! This is a notification for Moira trigger %s. Please, visit Moira web interface for details.", trigger.Name))
	}

	twilioCall, err := twilio.NewCall(voiceSender.client, voiceSender.APIFromPhone, contact.Value, twilio.Callback(voiceURL))

	if err != nil {
		return fmt.Errorf("Failed to make call to contact %s: %s", contact.Value, err.Error())
	}

	voiceSender.log.Debugf("Call queued to twilio with status %s, callback url %s", twilioCall.Status, voiceURL)

	return nil
}

// Sender implements moira sender interface via twilio
type Sender struct {
	sender sendEventsTwilio
}

//Init read yaml config
func (sender *Sender) Init(senderSettings map[string]string, logger *logging.Logger) error {
	apiType := senderSettings["type"]

	apiASID := senderSettings["api_asid"]
	if apiASID == "" {
		return fmt.Errorf("Can not read [%s] api_sid param from config", apiType)
	}

	apiAuthToken := senderSettings["api_authtoken"]
	if apiAuthToken == "" {
		return fmt.Errorf("Can not read [%s] api_authtoken param from config", apiType)
	}

	apiFromPhone := senderSettings["api_fromphone"]
	if apiFromPhone == "" {
		return fmt.Errorf("Can not read [%s] api_fromphone param from config", apiType)
	}

	twilioClient := twilio.NewClient(apiASID, apiAuthToken)

	switch apiType {
	case "twilio sms":
		sender.sender = &twilioSenderSms{twilioSender{twilioClient, apiFromPhone, logger}}

	case "twilio voice":
		voiceURL := senderSettings["voiceurl"]
		if voiceURL == "" {
			return fmt.Errorf("Can not read [%s] voiceurl param from config", apiType)
		}

		appendMessage := senderSettings["append_message"] == "true"

		sender.sender = &twilioSenderVoice{
			twilioSender{twilioClient, apiFromPhone, logger},
			voiceURL,
			appendMessage,
		}

	default:
		return fmt.Errorf("Wrong twilio type: %s", apiType)
	}

	return nil
}

//SendEvents implements Sender interface Send
func (sender *Sender) SendEvents(events notifier.EventsData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
	return sender.sender.SendEvents(events, contact, trigger, throttled)
}
