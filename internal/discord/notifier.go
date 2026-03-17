package discord

import (
	"net/url"

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
	embed := buildEmbed(event)
	return n.bot.SendEmbed(channelID, embed)
}

func buildEmbed(event monitor.ItemEvent) *discordgo.MessageEmbed {
	store := storeFromURL(event.Item.URL)
	switch event.Type {
	case monitor.EventRestock:
		return RestockEmbed(event.Item.Title, store, event.Item.URL, event.Item.ImageURL)
	case monitor.EventDelisted:
		return DelistedEmbed(event.Item.Title, store, event.Item.URL)
	case monitor.EventPasswordUp:
		return PasswordUpEmbed(event.Item.URL)
	case monitor.EventPasswordDown:
		return PasswordDownEmbed(event.Item.URL)
	default:
		return NewProductEmbed(event.Item.Title, store, event.Item.URL, event.Item.ImageURL)
	}
}

func storeFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Host
}
