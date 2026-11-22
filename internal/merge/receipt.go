package merge

// ReceiptRef points at the receipt a prompt proposal wrote.
type ReceiptRef struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}
