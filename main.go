package main

import (
    "github.com/nlopes/slack"
    "net/http"
    "log"
    "time"
    "fmt"
    "strconv"
    "regexp"
)

const botName = "bugbot"
const openProjectBugUrl = "https://openproject.activestate.com/work_packages/%s"
const bugzillaBugUrl = "https://bugs.activestate.com/show_bug.cgi?id=%s"
const bugNumberRegex = `[13]\d{5}`

var defaultParameters = slack.PostMessageParameters{}
var slackApi = slack.New("xoxb-4401757444-fDt9Tg9nroPbrlh5NxlDy4Kd")

func main() {
    //    chReceiver := make(chan slack.SlackEvent)
    //    webSocketApi, err := slackApi.StartRTM("", "http://example.com")

    slackApi.SetDebug(true)
    defaultParameters.AsUser = true
    // AsUser doesn't work yet on this Go API so let's implement a workaround
    defaultParameters.Username = "bugbot"
    defaultParameters.IconEmoji = ":catbug_static:"

    port := 8123
    log.Printf("Starting HTTP server on %d", port)

    mux := http.NewServeMux()
    mux.HandleFunc("/", Summon)
    mux.HandleFunc("/time", Time)

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
            if (message.SubType != "bot_message") {
                matches := bugNbRegex.FindAllString(message.Text, -1)
                if (matches != nil) {
                    log.Printf("That message mentions these bugs: %s", matches)
                    var messageText string
                    for _, match := range matches {
                        if (string(match[0]) == "3") {
                            messageText += fmt.Sprintf(openProjectBugUrl, match)
                        } else {
                            messageText += fmt.Sprintf(bugzillaBugUrl, match)
                        }
                        messageText += "\n"
                    }
                    slackApi.PostMessage(message.ChannelId, messageText, defaultParameters)
                }
            }
        }
    }
}

func Summon(w http.ResponseWriter, r *http.Request) {
    token := r.PostFormValue("token")
    if (token != "QMiGNjUdxAWwowx6RxPBDm4s") {
        log.Printf("Request from something other than the webhook")
    } else {
        incomingChannel := r.PostFormValue("channel_name")
        log.Printf("Summon request into channel: %s", incomingChannel)
        channelId := getChannelIdFromName(incomingChannel)
        if (isInChannel(channelId)) {
            log.Printf("Already in channel")
            slackApi.PostMessage(channelId, "Hi!", defaultParameters)
        } else {
            log.Printf("Not in channel")
            slackApi.PostMessage(channelId, fmt.Sprintf("Summon me with @%s!", botName), defaultParameters)
        }
    }
}

const layout = "Jan 2, 2006 at 3:04pm (MST)"
func Time(w http.ResponseWriter, r *http.Request) {
    log.Printf("Time request")
    tm := time.Now().Format(layout)
    w.Write([]byte("The time is: " + tm))
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