package bigcartel

type bigcartelProduct struct {
	ID        int               `json:"id"`
	Name      string            `json:"name"`
	Permalink string            `json:"permalink"`
	Status    string            `json:"status"`
	Options   []bigcartelOption `json:"options"`
	Images    []bigcartelImage  `json:"images"`
}

type bigcartelOption struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	SoldOut bool   `json:"sold_out"`
}

type bigcartelImage struct {
	URL       string `json:"url"`
	SecureURL string `json:"secure_url"`
}
