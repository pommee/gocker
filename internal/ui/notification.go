package ui

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type NotificationType int

type Notification struct {
	message          string
	notificationType NotificationType
	duration         int
}

const (
	SUCCESS NotificationType = iota
	INFO
	WARNING
)

func createNotification() *tview.TextView {
	notificationView = tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetTextColor(tview.Styles.PrimaryTextColor)
	return notificationView
}

func NotificationError(err error) {
	ShowNotification(Notification{
		message:          err.Error(),
		notificationType: WARNING,
		duration:         5,
	})
}

func NotificationSuccess(msg string) {
	ShowNotification(Notification{
		message:          msg,
		notificationType: SUCCESS,
		duration:         3,
	})
}

func NotificationInfo(msg string) {
	ShowNotification(Notification{
		message:          msg,
		notificationType: INFO,
		duration:         3,
	})
}

func ShowNotification(notification Notification) {
	notificationView.SetBorder(true)
	notificationView.SetBorderColor(getNotificationColor(notification.notificationType))
	notificationView.SetText(notification.message)

	go func() {
		app.Draw()
		time.AfterFunc(time.Second*time.Duration(notification.duration), func() {
			clearNotification()
		})
	}()
}

func getNotificationColor(notificationType NotificationType) tcell.Color {
	switch notificationType {
	case SUCCESS:
		return tcell.ColorGreen
	case INFO:
		return tcell.ColorBlue
	case WARNING:
		return tcell.ColorDarkRed
	default:
		return tcell.ColorWhite
	}
}

func clearNotification() {
	notificationView.SetText("")
	notificationView.SetBorder(false)
	app.Draw()
}
