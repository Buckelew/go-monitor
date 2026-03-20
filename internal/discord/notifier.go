package discord

import (
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
	return n.bot.SendEmbed(channelID, BuildEmbed(event))
}
