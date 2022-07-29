package httplistener

import (
	"fmt"
	"github.com/spf13/viper"
	"gopkg.in/go-playground/webhooks.v5/gogs"
	"net/http"
	"strings"

	client "github.com/gogits/go-gogs-client"
)

func (hl *HTTPListener) gogsHandler(w http.ResponseWriter, request *http.Request) {
	if request.Method != "POST" {
		http.NotFound(w, request)
		return
	}

	hook, err := gogs.New(gogs.Options.Secret(viper.GetString("http.listeners.gogs.secret")))

	if err != nil {
		return
	}

	// All valid events we want to receive need to be listed here.
	payload, err := hook.Parse(request,
		gogs.ReleaseEvent, gogs.PushEvent, gogs.IssuesEvent, gogs.IssueCommentEvent,
		gogs.PullRequestEvent)

	if err != nil {
		if err == gogs.ErrEventNotFound {
			// We've received an event we don't need to handle, return normally
			return
		}
		log.Warningf("Error parsing gogs webhook: %s", err)
		http.Error(w, "Error processing webhook", http.StatusBadRequest)
		return
	}

	msgs := []string{}
	repo := ""
	send := false

	switch payload.(type) {
	case client.ReleasePayload:
		pl := payload.(client.ReleasePayload)
		if pl.Action == "published" {
			send = true
			msgs, err = hl.renderTemplate("gogs.release", payload)
			repo = pl.Repository.Name
		}
	case client.PushPayload:
		pl := payload.(client.PushPayload)
		send = true
		msgs, err = hl.renderTemplate("gogs.push", payload)
		repo = pl.Repo.Name
	case client.IssueCommentPayload:
		pl := payload.(client.IssueCommentPayload)
		if pl.Action == "created" {
			send = true
			msgs, err = hl.renderTemplate("gogs.issuecomment", payload)
			repo = pl.Repository.Name
		}
	case client.PullRequestPayload:
		pl := payload.(client.PullRequestPayload)
		if interestingIssueAction(string(pl.Action)) {
			send = true
			msgs, err = hl.renderTemplate("gogs.pullrequest", payload)
			repo = pl.Repository.Name
		}
	}

	if err != nil {
		log.Errorf("Error rendering Gogs event template: %s", err)
		return
	}

	if send {
		repo = strings.ToLower(repo)
		channel := viper.GetString(fmt.Sprintf("http.listeners.gogs.repositories.%s", repo))
		if channel == "" {
			channel = viper.GetString("http.listeners.gogs.default_channel")
		}

		if channel == "" {
			log.Infof("%s Gogs event for unrecognised repository %s", request.RemoteAddr, repo)
			return
		}

		log.Infof("%s [%s -> %s] Gogs event received", request.RemoteAddr, repo, channel)
		for _, msg := range msgs {
			hl.irc.Privmsg(channel, msg)
		}
	}
}
