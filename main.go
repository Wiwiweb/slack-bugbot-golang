package main

import (
    "github.com/nlopes/slack"
    "net/http"
    "log"
    "time"
    "strconv"
    "fmt"
)

const botName = "bugbot"
var emptyParameters = slack.PostMessageParameters{}
var slackApi = slack.New("xoxb-4401757444-fDt9Tg9nroPbrlh5NxlDy4Kd")

func main() {
    //    chReceiver := make(chan slack.SlackEvent)
    //    webSocketApi, err := slackApi.StartRTM("", "http://example.com")

    slackApi.SetDebug(true)
    emptyParameters.AsUser = true

    port := 8123
    log.Printf("Starting HTTP server on %d", port)

    mux := http.NewServeMux()
    mux.HandleFunc("/", Summon)
    mux.HandleFunc("/time", Time)

    http.ListenAndServe(":"+strconv.Itoa(port), mux)
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
            slackApi.PostMessage(channelId, "Hi!", emptyParameters)
        } else {
            log.Printf("Not in channel")
            log.Printf("%b", emptyParameters.AsUser)
            slackApi.PostMessage(channelId, fmt.Sprintf("Summon me with @%s!", botName), emptyParameters)
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