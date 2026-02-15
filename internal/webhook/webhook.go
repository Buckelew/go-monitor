package webhook

type Webhook interface {
	ID() string
	SendDiscord() // context? product? product will be product type specific
}
