package tgbotbase

type Config struct {
	TGBot struct {
		Token            string
		SkipConnect      bool
		Verbose          bool
		RedirectMsgToLog bool
	}

	Proxy_SOCKS5 struct {
		Server string
		User   string
		Pass   string
	}
}
