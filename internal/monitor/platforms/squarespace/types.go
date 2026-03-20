package squarespace

type sqsPagination struct {
	NextPage       bool `json:"nextPage"`
	NextPageOffset int  `json:"nextPageOffset"`
}

type squarespaceItem struct {
	ID                string               `json:"id"`
	Title             string               `json:"title"`
	URLId             string               `json:"urlId"`
	FullURL           string               `json:"fullUrl"`
	AssetURL          string               `json:"assetUrl"`
	StructuredContent *sqsStructuredContent `json:"structuredContent"`
}

type sqsStructuredContent struct {
	Variants []sqsVariant `json:"variants"`
}

type sqsVariant struct {
	ID         string `json:"id"`
	QtyInStock int    `json:"qtyInStock"`
	Unlimited  bool   `json:"unlimited"`
}
