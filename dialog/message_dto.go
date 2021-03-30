package dialog

type MessageDto struct {
	ID             int
	ChatID         int64
	PhotoUrls      []string
	Command        string
	Text           string
	UnknownContent bool
}
