package main

import (
    "github.com/nlopes/slack"
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
    "log"
    "time"
    "fmt"
    "regexp"
    "strings"
    "os"
    "encoding/json"
    "errors"
)

const botName = "bugbot"
const botKey = "xoxb-4401757444-fDt9Tg9nroPbrlh5NxlDy4Kd"
const openProjectBugUrl = "https://openproject.activestate.com/work_packages/%s"
const bugzillaBugUrl = "https://bugs.activestate.com/show_bug.cgi?id=%s"
const bugNumberRegex = `(?:\s|^)#?([13]\d{5})\b(?:[^-]|$)`

type MysqlConfig struct {
    Host     string
    Database string
    Username string
    Password string
}

var messageParameters = slack.NewPostMessageParameters()
var historyParameters = slack.NewHistoryParameters()
var slackApi = slack.New(botKey)
var mysqlConfig = MysqlConfig{}

func main() {
    file, _ := os.Open("mysqlConfig.json")
    decoder := json.NewDecoder(file)
    decoder.Decode(&mysqlConfig)

    messageParameters.AsUser = true
    historyParameters.Count = 10

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
        message, ok := event.Data.(*slack.MessageEvent)
        if ok {
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
                        log.Printf("Bug %s was already linked recently", match)
                    } else {
                        if string(match[0]) == "3" {
                            bugTitle, err := fetchOpenProjectBugTitle(match)
                            if err != nil && err.Error() == "This bug doesn't exist!" {
                                messageText += fmt.Sprintf("Bug %s doesn't exist!", match)
                            } else if bugTitle == "" {
                                messageText += fmt.Sprintf(openProjectBugUrl, match)
                            } else {
                                messageText += fmt.Sprintf("%s: %s - %s",
                                match, bugTitle, fmt.Sprintf(openProjectBugUrl, match))
                            }
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

func fetchOpenProjectBugTitle(bugNumber string) (string, error) {
    connectionURL := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?allowOldPasswords=1",
    mysqlConfig.Username, mysqlConfig.Password, mysqlConfig.Host, mysqlConfig.Database)
    log.Printf(connectionURL)
    db, err := sql.Open("mysql", connectionURL)
    if err != nil {
        log.Printf("Mysql database is unavailable! %s", err.Error())
        return "", err
    }
    defer db.Close()

    stmtIns, err := db.Prepare("SELECT subject FROM work_packages WHERE id=?")
    if err != nil {
        log.Printf("MySQL statement preparation failed! %s", err.Error())
        return "", err
    }
    defer stmtIns.Close()

    var bugTitle string
    stmtIns.QueryRow(bugNumber).Scan(&bugTitle)
    if err != nil {
        log.Printf("MySQL statement failed! %s", err.Error())
        return "", err
    }

    if bugTitle == "" {
        return "", errors.New("This bug doesn't exist!")
    }

    log.Printf("#%s: %s", bugNumber, bugTitle)
    return bugTitle, nil
}
