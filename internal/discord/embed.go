package discord

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/buckelew/go-monitor/internal/monitor"
)

const (
	colorBlue   = 0x0099ff
	colorGreen  = 0x2ecc71
	colorRed    = 0xe74c3c
	embedFooter = "@buckelew"
)

type eventStyle struct {
	Color  int
	Prefix string
}

var eventStyles = map[monitor.EventType]eventStyle{
	monitor.EventNewProduct:   {colorBlue, "New Product: "},
	monitor.EventRestock:      {colorGreen, "Restocked: "},
	monitor.EventDelisted:     {colorRed, "Removed: "},
	monitor.EventPasswordUp:   {colorRed, "Password Up"},
	monitor.EventPasswordDown: {colorGreen, "Password Down"},
}

// BuildEmbed creates a uniform Discord embed for any item event.
func BuildEmbed(event monitor.ItemEvent) *discordgo.MessageEmbed {
	style := eventStyles[event.Type]
	store := storeFromURL(event.Item.URL)

	// Password events are store-level, not product-level.
	if event.Type == monitor.EventPasswordUp || event.Type == monitor.EventPasswordDown {
		return &discordgo.MessageEmbed{
			Title:     style.Prefix,
			URL:       event.Item.URL,
			Color:     style.Color,
			Author:    &discordgo.MessageEmbedAuthor{Name: store},
			Footer:    &discordgo.MessageEmbedFooter{Text: embedFooter},
			Timestamp: time.Now().Format(time.RFC3339),
		}
	}

	embed := &discordgo.MessageEmbed{
		Title:     style.Prefix + event.Item.Title,
		URL:       event.Item.URL,
		Color:     style.Color,
		Author:    &discordgo.MessageEmbedAuthor{Name: store},
		Footer:    &discordgo.MessageEmbedFooter{Text: embedFooter},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if event.Item.ImageURL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: event.Item.ImageURL}
	}

	if price := priceFromData(event.Item.Data); price != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Price",
			Value:  price,
			Inline: true,
		})
	}

	// Stock status for new/restock only (redundant for "Removed:").
	if event.Type == monitor.EventNewProduct || event.Type == monitor.EventRestock {
		status := "Out of Stock"
		if event.Item.InStock {
			status = "In Stock"
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Status",
			Value:  status,
			Inline: true,
		})
	}

	return embed
}

// priceFromData extracts the first available price from item data JSON.
// Currently only Shopify stores variant prices in the data column.
func priceFromData(data json.RawMessage) string {
	if len(data) == 0 {
		return ""
	}
	var d struct {
		Variants []struct {
			Price string `json:"price"`
		} `json:"variants"`
	}
	if json.Unmarshal(data, &d) == nil && len(d.Variants) > 0 && d.Variants[0].Price != "" {
		return fmt.Sprintf("$%s", d.Variants[0].Price)
	}
	return ""
}

func storeFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Host
}
