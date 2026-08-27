package model

type NotifyMode string

const (
	NotifyNone   NotifyMode = "none"
	NotifySilent NotifyMode = "silent"
	NotifyPopup  NotifyMode = "popup"
)
