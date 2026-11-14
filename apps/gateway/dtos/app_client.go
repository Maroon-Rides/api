package dtos

// AppClient is what a request says about the app that sent it. Every field comes from the caller and is untrusted.
type AppClient struct {
	Platform  string
	Version   string
	UserAgent string
}
