package discord

import (
	"github.com/bwmarrin/discordgo"
	"github.com/buckelew/go-monitor/internal/monitor"
)

// DiscordNotifier implements monitor.Notifier using a Discord bot.
type DiscordNotifier struct {
	bot *Bot
}

func NewNotifier(bot *Bot) *DiscordNotifier {
	return &DiscordNotifier{bot: bot}
}

func (n *DiscordNotifier) Notify(channelID string, event monitor.ItemEvent) error {
	var embed = buildEmbed(event)
	return n.bot.SendEmbed(channelID, embed)
}

func buildEmbed(event monitor.ItemEvent) *discordgo.MessageEmbed {
	switch event.Type {
	case monitor.EventRestock:
		return RestockEmbed(event.Item.Title, event.Item.URL, event.Item.ImageURL)
	case monitor.EventDelisted:
		return DelistedEmbed(event.Item.Title, event.Item.URL)
	default:
		return NewProductEmbed(event.Item.Title, event.Item.URL, event.Item.ImageURL)
	}
}
