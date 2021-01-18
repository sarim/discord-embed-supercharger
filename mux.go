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
	"gopkg.in/xmlpath.v2"
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

	fbLinkRe := regexp.MustCompile(`(https:\/\/.*\.facebook\.com\/[^ ?]+)[?]?[^ ]*`)

	if postLink := fbLinkRe.FindStringSubmatch(mc.Content); postLink != nil {
		log.Println(mc.Content, c.ID, mc.Message.ID)

		AuthorName := mc.Member.Nick
		if AuthorName == "" {
			AuthorName = mc.Author.Username
		}

		msg, err := parseFacebookPost(postLink[1])
		if err != nil {
			log.Println(err.Error())
		}
		if len(msg) > 200 {
			msg = msg[0:200]
		}
		msgRaw := string(msg)
		msgRaw = "**" + mc.Author.Username + "** Says: \n" + fbLinkRe.ReplaceAllString(mc.Content, "<"+postLink[1]+"> \n> "+msgRaw+"\n")
		_, err = ds.ChannelMessageSend(c.ID, msgRaw)
		if err != nil {
			log.Println(err.Error())
			fmt.Println(msg)
		}

		err = ds.ChannelMessageDelete(c.ID, mc.Message.ID)
		if err != nil {
			log.Println(err.Error())
		}
	}
}

func parseFacebookPost(postLink string) ([]rune, error) {
	path := xmlpath.MustCompile("//div[@id='m_story_permalink_view']")

	req, err := http.NewRequest("GET", postLink, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 6.0.1; Moto G (4)) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.0.4324.87 Mobile Safari/537.36 Edg/88.0.705.45")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return []rune(resp.Status), errors.New(resp.Status)
	}
	node, err := xmlpath.ParseHTML(resp.Body)
	if err != nil {
		return nil, err
	}

	for iter := path.Iter(node); iter.Next(); {
		return []rune(iter.Node().String()), nil
	}

	return nil, nil
}
