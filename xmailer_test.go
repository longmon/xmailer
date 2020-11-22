package xmailer

import (
	"testing"
)

func TestSend(t *testing.T) {
	x, e := NewXMailer("smtp.qq.com:587", "xxx@qq.com", "xxx")
	if e != nil {
		t.Error(e)
	}
	m := NewMessage()
	m.SetFrom("longmon", "xxx@qq.com")
	m.SetSubject("Awesome Subject")
	m.AddTo("123456@qq.com")

	m.SetHTML("<h1>This is a test email from xmailer</h1>")

	m.AttachFile("C:\\Users\\NING MEI\\Desktop\\go-projects\\playround\\xmailer\\README.md")

	err := x.Dial()
	if err != nil {
		t.Error(err)
	}

	err = x.Send(m)
	if err != nil {
		t.Error(err)
	}

}
