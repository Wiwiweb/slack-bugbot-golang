package main

import (
    "github.com/nlopes/slack"
//    "database/sql"
//    _ "github.com/go-sql-driver/mysql"
    "net/http"
    "log"
    "time"
    "fmt"
    "strconv"
    "regexp"
    "strings"
)

const botName = "bugbot"
const openProjectBugUrl = "https://openproject.activestate.com/work_packages/%s"
const bugzillaBugUrl = "https://bugs.activestate.com/show_bug.cgi?id=%s"
const bugNumberRegex = `(?:\s|^)#?([13]\d{5})\b(?:[^-]|$)`

var messageParameters = slack.NewPostMessageParameters()
var historyParameters = slack.NewHistoryParameters()
var slackApi = slack.New("xoxb-4401757444-fDt9Tg9nroPbrlh5NxlDy4Kd")

func main() {
    messageParameters.AsUser = true
    // AsUser doesn't work yet on this Go API so let's implement a workaround
    messageParameters.Username = "bugbot"
    messageParameters.IconEmoji = ":catbug_static:"
    historyParameters.Count = 10

    port := 8123
    log.Printf("Starting HTTP server on %d", port)

    mux := http.NewServeMux()
    mux.HandleFunc("/", Summon)
    go http.ListenAndServe(":" + strconv.Itoa(port), mux)

    chReceiver := make(chan slack.SlackEvent, 100)
    // Seems like the protocol is optional, and the origin can be any URL
    rtmAPI, err := slackApi.StartRTM("", "http://example.com")
    if err != nil {
        log.Printf("Error starting RTM: %s", err)
    }
    go rtmAPI.HandleIncomingEvents(chReceiver)
    go rtmAPI.Keepalive(20 * time.Second)
    log.Printf("RTM is started")

    bugNbRegex := regexp.MustCompile(bugNumberRegex)

    for {
        event := <-chReceiver
        // Seems weird to use a switch with just one case
        // but apparently that's the only way to check an interface{} for type
        switch event.Data.(type) {
            case *slack.MessageEvent:
            message := event.Data.(*slack.MessageEvent)
            // That event doesn't contain the Username, so we can't use message.Username
            log.Printf("Message from %s in channel %s: %s\n", message.UserId, message.ChannelId, message.Text)
            matches := bugNbRegex.FindAllStringSubmatch(message.Text, -1)
            if matches != nil {
                // We only care about the first capturing group
                matchesNb := make([]string, len(matches))
                for i, _ := range matches {
                    matchesNb[i] = matches[i][1]
                }
                log.Printf("That message mentions these bugs: %s", matchesNb)
                var messageText string
                for _, match := range matchesNb {
                    if bugNumberWasLinkedRecently(match, message.ChannelId, message.Timestamp) {
                        log.Printf("Bug %s was already linked drecently", match)
                    } else {
                        if string(match[0]) == "3" {
                            messageText += fmt.Sprintf(openProjectBugUrl, match)
                        } else {
                            messageText += fmt.Sprintf(bugzillaBugUrl, match)
                        }
                        messageText += "\n"
                    }
                }
                if messageText != "" {
                    slackApi.PostMessage(message.ChannelId, messageText, messageParameters)
                }
            }
        }
    }
}

func Summon(w http.ResponseWriter, r *http.Request) {
    token := r.PostFormValue("token")
    if token != "QMiGNjUdxAWwowx6RxPBDm4s" {
        log.Printf("Request from something other than the webhook")
    } else {
        incomingChannel := r.PostFormValue("channel_name")
        log.Printf("Summon request into channel: %s", incomingChannel)
        channelId := getChannelIdFromName(incomingChannel)
        if isInChannel(channelId) {
            log.Printf("Already in channel")
            slackApi.PostMessage(channelId, "Hi! <test|https://github.com>", messageParameters)
        } else {
            log.Printf("Not in channel")
            slackApi.PostMessage(channelId, fmt.Sprintf("Summon me with @%s!", botName), messageParameters)
        }
    }
}

func getChannelIdFromName(channelName string) string {
    log.Printf("getChannelIdFromName: %s", channelName)
    allChannels, _ := slackApi.GetChannels(true)
    for _, channel := range allChannels {
        if channel.Name == channelName {
            return channel.Id
        }
    }
    return ""
}

func isInChannel(channelId string) bool {
    log.Printf("isInChannel: %s", channelId)
    info, _ := slackApi.GetChannelInfo(channelId)
    return info.IsMember
}

func bugNumberWasLinkedRecently(number string, channelId string, messageTime string) bool {
    historyParameters.Latest = messageTime
    info, _ := slackApi.GetChannelHistory(channelId, historyParameters)
    for _, message := range info.Messages {
        if strings.Contains(message.Text, number) {
            return true
        }
    }
    return false
}
