// Package mux provides a simple Discord message route multiplexer that
// parses messages and then executes a matching registered handler, if found.
// mux can be used with both Disgord and the DiscordGo library.
package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/jaytaylor/html2text"
)

// Context holds a bit of extra data we pass along to route handlers
// This way processing some of this only needs to happen once.
type Context struct {
	Fields          []string
	Content         string
	IsDirected      bool
	IsPrivate       bool
	HasPrefix       bool
	HasMention      bool
	HasMentionFirst bool
}

// HandlerFunc is the function signature required for a message route handler.
type HandlerFunc func(*discordgo.Session, *discordgo.Message, *Context)

// Mux is the main struct for all mux methods.
type Mux struct {
}

// OnMessageCreate is a DiscordGo Event Handler function.  This must be
// registered using the DiscordGo.Session.AddHandler function.  This function
// will receive all Discord messages and parse them for matches to registered
// routes.
func (m *Mux) OnMessageCreate(ds *discordgo.Session, mc *discordgo.MessageCreate) {

	var err error

	// Ignore all messages created by the Bot account itself
	if mc.Author.ID == ds.State.User.ID {
		return
	}

	// Create Context struct that we can put various infos into
	ctx := &Context{
		Content: strings.TrimSpace(mc.Content),
	}

	// Fetch the channel for this Message
	var c *discordgo.Channel
	c, err = ds.State.Channel(mc.ChannelID)
	if err != nil {
		// Try fetching via REST API
		c, err = ds.Channel(mc.ChannelID)
		if err != nil {
			log.Printf("unable to fetch Channel for Message, %s", err)
		} else {
			// Attempt to add this channel into our State
			err = ds.State.ChannelAdd(c)
			if err != nil {
				log.Printf("error updating State with Channel, %s", err)
			}
		}
	}
	// Add Channel info into Context (if we successfully got the channel)
	if c != nil {
		if c.Type == discordgo.ChannelTypeDM {
			ctx.IsPrivate, ctx.IsDirected = true, true
		}
	}

	fbLinkRe := regexp.MustCompile(`(:?https:\/\/.*)?facebook\.com\/[^ ]+`)

	if postLink := fbLinkRe.FindString(mc.Content); postLink != "" {
		fmt.Println(mc.Content, c.ID, mc.Message.ID)
		err := ds.ChannelMessageDelete(c.ID, mc.Message.ID)
		if err != nil {
			fmt.Println(err.Error())
		}
		msg, err := parseFacebookPost(postLink)
		if err != nil {
			fmt.Println(err.Error())
		}
		_, err = ds.ChannelMessageSend(c.ID, msg)
		if err != nil {
			fmt.Println(err.Error())
		}
	}
}

func parseFacebookPost(postLink string) (string, error) {
	resp, err := http.Get("https://www.facebook.com/plugins/post.php?href=" + postLink + "&width=500&show_text=true&appId=508863169151565&height=497")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return resp.Status, errors.New(resp.Status)
	}
	text, err := html2text.FromReader(resp.Body, html2text.Options{OmitLinks: true})
	if err != nil {
		return "", err
	}

	return text, nil
}
