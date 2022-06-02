package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	TOKEN_KEY = "DISCORD_BOT_TOKEN"
)

func main() {

	dg, err := discordgo.New("Bot " + os.Getenv(TOKEN_KEY))
	if err != nil {
		log.Println("unable to create Discord session, ", err)
		return
	}

	dg.AddHandler(handleMessage)
	dg.AddHandler(handleMemberJoin)
	dg.AddHandler(handleMessageReaction)
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsGuildMessageReactions | discordgo.IntentsGuildMembers

	err = dg.Open()
	if err != nil {
		log.Println("error opening connection, ", err)
		return
	}
	defer func() {
		log.Println("Kill signal received")
		log.Println("Closing bot down")
		dg.Close()
	}()

	log.Println("Bot started")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}

func handleMemberJoin(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	loadRolesUser(s, m.Member)
}

func handleMessageReaction(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	if c, _ := s.Channel(m.ChannelID); c.Name == "votes" {
		log.Println("Adding reaction to message")
		err := s.MessageReactionAdd(m.ChannelID, m.MessageID, m.Emoji.APIName())
		if err != nil {
			log.Println("There was an error adding the reaction, ", err)
		}
	}
}

func handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	parts := strings.Split(m.Content, " ")

	switch parts[0] {
	case "save":
		for _, user := range m.Mentions {
			member, err := s.GuildMember(m.GuildID, user.ID)
			if err != nil {
				log.Println("There was an error getting the member, ", err)
			}
			saveRolesUser(s, member)
		}
	case "load":
		for _, user := range m.Mentions {
			member, err := s.GuildMember(m.GuildID, user.ID)
			if err != nil {
				log.Println("There was an error getting the member, ", err)
			}
			loadRolesUser(s, member)
		}
	case "add":
		addRolesToUsers(s, m)
	case "clear":
		limit, err := strconv.ParseInt(parts[1], 10, 32)
		if err != nil {
			log.Println("Unable to convert limit input, ", err)
		}
		if limit > 100 {
			limit = 100
		}

		msgs, err := s.ChannelMessages(m.ChannelID, int(limit), "", "", "")
		if err != nil {
			log.Println("Unable to get messages,", err)
		}

		var msgIds []string
		for _, msg := range msgs {
			msgIds = append(msgIds, msg.ID)
		}

		err = s.ChannelMessagesBulkDelete(m.ChannelID, msgIds)
		if err != nil {
			log.Println("Unable to delete messages,", err)
		}
	case "!remind":
		guild, err := s.Guild(m.GuildID)
		if err != nil {
			log.Println("Unable to get guild,", err)
		}
		s.State.GuildAdd(guild)

		channel, err := s.Channel(m.ChannelID)
		if err != nil {
			log.Println("Unable to get channel,", err)
		}
		s.State.ChannelAdd(channel)

		perms, err := s.State.MessagePermissions(m.Message)
		if err != nil {
			log.Println("Unable to get message permissions,", err)
		}

		if perms&discordgo.PermissionKickMembers == discordgo.PermissionKickMembers {
			reminder(s, m, parts[1], parts[2:])
		} else {
			log.Println("Incorrect permissions,", perms)
		}
	}
}

func reminder(s *discordgo.Session, m *discordgo.MessageCreate, period string, content []string) {
	waitSeconds := 0
	temp := 0

	unitToSeconds := map[rune]int{
		's': 1,
		'm': 60,
		'h': 60 * 60,
		'd': 60 * 60 * 24,
	}

	for _, c := range period {
		if '0' <= c && c <= '9' {
			temp = temp * 10
			temp += int(c - '0')
		} else {
			secondsFactor, ok := unitToSeconds[c]
			if !ok {
				log.Println("Invalid time period, reminder will not be created")
				return
			}
			waitSeconds += temp * secondsFactor
			temp = 0
		}
	}

	if waitSeconds == 0 {
		return
	}

	go func() {
		<-time.After(time.Duration(waitSeconds) * time.Second)
		message := fmt.Sprintf("%s %s", m.Author.Mention(), strings.Join(content, " "))
		s.ChannelMessageSend(m.ChannelID, message)
	}()
}

func loadRolesUser(s *discordgo.Session, m *discordgo.Member) {
	memberRoles, err := getMemberRolesFromCache(m.User.ID, m.GuildID)
	if err != nil {
		log.Println("Unable to get roles from cache,", err)
	}

	guildRoles, err := s.GuildRoles(m.GuildID)
	if err != nil {
		log.Println("Unable to get roles in current guild,", err)
	}

	for _, guildRole := range guildRoles {
		for _, userRoleId := range memberRoles.RoleIds {
			if guildRole.ID == userRoleId {
				s.GuildMemberRoleAdd(m.GuildID, m.User.ID, userRoleId)
			}
		}
	}
	log.Println("Loaded roles for", m.User.Username)
}

func saveRolesUser(s *discordgo.Session, m *discordgo.Member) {
	memberRoles := MemberRoles{
		UserId:  m.User.ID,
		GuildId: m.GuildID,
		RoleIds: m.Roles,
	}

	err := saveMemberRolesToCache(memberRoles)
	if err != nil {
		log.Println("Unable to save roles for member,", err)
	} else {
		log.Println("Saved roles for", m.User.Username)
	}
}

func addRolesToUsers(s *discordgo.Session, m *discordgo.MessageCreate) {
	mentionedRoles := m.MentionRoles
	mentionedUsers := m.Mentions
	var roles []*discordgo.Role

	guildRoles, err := s.GuildRoles(m.GuildID)
	if err != nil {
		log.Println("Unable to get guild roles, ", err)
	}
	for _, guildRole := range guildRoles {
		for _, mentionedRole := range mentionedRoles {
			if mentionedRole == guildRole.ID {
				roles = append(roles, guildRole)
			}
		}
	}

	for _, user := range mentionedUsers {
		for _, role := range roles {
			err := s.GuildMemberRoleAdd(m.GuildID, user.ID, role.ID)
			if err != nil {
				log.Printf("Unable to add role %s to %s with error %s", role.Name, user.Username, err)
			} else {
				log.Printf("Added role %s to %s", role.Name, user.Username)
			}
		}
	}
}
