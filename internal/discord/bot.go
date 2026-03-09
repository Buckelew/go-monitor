package discord

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	session *discordgo.Session
}

func NewBot(token string) (*Bot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord session: %w", err)
	}
	return &Bot{session: session}, nil
}

func (b *Bot) Open() error {
	return b.session.Open()
}

func (b *Bot) Close() error {
	return b.session.Close()
}

func (b *Bot) SendEmbed(channelID string, embed *discordgo.MessageEmbed) error {
	_, err := b.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}
