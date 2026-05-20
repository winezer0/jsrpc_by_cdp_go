package cdp

import "github.com/chromedp/cdproto/target"

type browserVersionInfo struct {
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type pageTargetInfo struct {
	ID                   target.ID `json:"id"`
	Type                 string    `json:"type"`
	URL                  string    `json:"url"`
	WebSocketDebuggerURL string    `json:"webSocketDebuggerUrl"`
}
