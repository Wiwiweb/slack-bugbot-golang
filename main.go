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
    "os/exec"
)

const botName = "bugbot"
const botSlackId = "U04BTN9D2"
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
    messageParameters.EscapeText = false
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
        if ok { // If this is a MessageEvent
            // That event doesn't contain the Username, so we can't use message.Username
            log.Printf("Message from %s in channel %s: %s\n", message.UserId, message.ChannelId, message.Text)

            matches := bugNbRegex.FindAllStringSubmatch(message.Text, -1)
            if matches != nil {
                // We only care about the first capturing group
                matchesNb := make([]string, len(matches))
                for i, _ := range matches {
                    matchesNb[i] = matches[i][1]
                }
                bugMentions(matchesNb, message)
            } else if strings.Contains(message.Text, botName) || strings.Contains(message.Text, botSlackId) {
                bugbotMention(message)
            }
        }
    }
}

func bugMentions(bugNumbers []string, message *slack.MessageEvent) {
    log.Printf("That message mentions these bugs: %s", bugNumbers)
    var messageText string

    for _, match := range bugNumbers {
        if bugNumberWasLinkedRecently(match, message.ChannelId, message.Timestamp) {
            log.Printf("Bug %s was already linked recently", match)
        } else {
            if string(match[0]) == "3" {
                messageText += formatOpenProjectBugMessage(match)
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

func formatOpenProjectBugMessage(bugNumber string) string {
    var messageText string
    bugTitle, err := fetchOpenProjectBugTitle(bugNumber)
    if err != nil && err.Error() == "This bug doesn't exist!" {
        messageText += fmt.Sprintf("Bug %s doesn't exist!", bugNumber)
    } else if bugTitle == "" {
        messageText += fmt.Sprintf("<%s|%s (Couldn't fetch title)>",
        fmt.Sprintf(openProjectBugUrl, bugNumber), bugNumber)
    } else {
        messageText += fmt.Sprintf("<%s|%s: %s>",
        fmt.Sprintf(openProjectBugUrl, bugNumber), bugNumber, bugTitle)
    }
    return messageText
}

func bugNumberWasLinkedRecently(number string, channelId string, messageTime string) bool {
    historyParameters.Latest = messageTime
    info, _ := slackApi.GetChannelHistory(channelId, historyParameters)
    // Last 10 messages (see historyParameters.Count)
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

func bugbotMention(message *slack.MessageEvent) {
    log.Printf("That message mentions bugbot")
    // Unmerged bugs
    matched, _ := regexp.MatchString(`^(?:[@/]?bugbot|<@U04BTN9D2>) unmerged`, message.Text)
    if matched {
        lines := getUnMergedBugNumbers()
        messageText := "*Issues that are unmerged to master:*\n"
        for _, bugNumber := range lines {
            log.Printf("bugNumber: %s", bugNumber)
            // Kind of a hack until I can make sure getUnMergedBugNumbers returns only bug numbers
            if bugNumber != "" && string(bugNumber[0]) == "3" {
                messageText += formatOpenProjectBugMessage(bugNumber)
                messageText += "\n"
            }
        }
        slackApi.PostMessage(message.ChannelId, messageText, messageParameters)
    }

    // Thanks
    matched, _ = regexp.MatchString(`[Tt]hanks`, message.Text)
    if matched {
        messageText := "You're welcome! :catbug:"
        slackApi.PostMessage(message.ChannelId, messageText, messageParameters)
    }
}

func getUnMergedBugNumbers() []string {
    log.Printf("Call for unmerged bug check")
    out, err := exec.Command("sh", "unmerged-bugs.sh").Output()
    if err != nil {
        log.Fatal(err)
    }
    lines := strings.Split(string(out), "\n")
    log.Printf("Unmerged bugs: %s", lines)
    return lines
}