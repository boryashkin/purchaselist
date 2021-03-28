package dialog

type MessageDto struct {
	PhotoUrls      []string
	Command        string
	Text           string
	UnknownContent bool
}
