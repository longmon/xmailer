package xmailer

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net"
	"net/smtp"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

//LocalName 本地主机名
const LocalName = "localhost"

//Attachment 附件对象
type Attachment struct {
	ContentType string
	FileName    string
	BaseName    string
	Content     []byte
}

//XMailer 邮件客户端
type XMailer struct {
	Addr      string
	Host      string
	auth      smtp.Auth
	client    *smtp.Client
	dialed    bool
	tlsConfig *tls.Config
}

//Message 邮件消息体
type Message struct {
	Subject     string
	FromAddr    string
	FromName    string
	To          []string
	CC          []string
	Bcc         []string
	Text        string
	HTML        string
	Attachments []*Attachment
}

var boundary = generateBoundary()

func NewXMailer(addr, username, passwd string) (*XMailer, error) {
	pos := strings.Index(addr, ":")
	if pos == -1 || pos == 0 || pos == len(addr)-1 {
		return nil, fmt.Errorf("invalid smtp server address")
	}

	host, _, err := net.SplitHostPort(addr)

	if err != nil {
		return nil, err
	}

	return &XMailer{
		Addr:   addr,
		Host:   host,
		auth:   smtp.PlainAuth("", username, passwd, host),
		dialed: false,
	}, nil
}

func NewXMailerWithStartTLS(addr, username, passwd string, tlsConfig *tls.Config) (*XMailer, error) {
	pos := strings.Index(addr, ":")
	if pos == -1 || pos == 0 || pos == len(addr)-1 {
		return nil, fmt.Errorf("invalid smtp server address")
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	if tlsConfig == nil {
		return nil, errors.New("Must specify the TLS config")
	}

	return &XMailer{
		Addr:      addr,
		Host:      host,
		auth:      smtp.PlainAuth("", username, passwd, host),
		dialed:    false,
		tlsConfig: tlsConfig,
	}, nil
}

func (x *XMailer) Send(m *Message) error {

	if !x.dialed {
		x.Dial()
	}

	if m.FromAddr == "" {
		return fmt.Errorf("Must specify the From address")
	}

	if len(m.To) == 0 {
		return fmt.Errorf("Must specify at least one To address")
	}

	if m.Subject == "" {
		m.Subject = "无题"
	}

	if err := x.client.Mail(m.FromAddr); err != nil {
		return err
	}

	for _, t := range m.To {
		if err := x.client.Rcpt(t); err != nil {
			return err
		}
	}

	w, err := x.client.Data()
	if err != nil {
		return err
	}
	defer w.Close()

	payload, err := m.payload()
	if err != nil {
		return err
	}

	w.Write(payload)

	return nil

}

func (x *XMailer) Dial() error {
	co, err := smtp.Dial(x.Addr)
	if err != nil {
		return err
	}
	x.client = co
	if err = x.client.Hello(LocalName); err != nil {
		return err
	}

	if ok, _ := x.client.Extension("STARTTLS"); ok {
		var tlsConfig *tls.Config
		if x.tlsConfig != nil {
			tlsConfig = x.tlsConfig
		} else {
			tlsConfig = &tls.Config{ServerName: x.Host}
		}
		if err := x.client.StartTLS(tlsConfig); err != nil {
			return err
		}
	}
	if err := x.Auth(); err != nil {
		return err
	}

	x.dialed = true
	
	return nil
}

func (x *XMailer) DialWithTLS(t *tls.Config) error {
	c, err := tls.Dial("tcp", x.Addr, t)
	if err != nil {
		return err
	}

	co, err := smtp.NewClient(c, x.Addr)
	if err != nil {
		return err
	}

	x.client = co

	if err := x.client.Hello(LocalName); err != nil {
		return err
	}

	if err := x.Auth(); err != nil {
		return err
	}
	x.dialed = true
	return nil
}

func (x *XMailer) DialWithStartTLS(t *tls.Config) error {

	co, err := smtp.Dial(x.Addr)
	if err != nil {
		return err
	}

	x.client = co
	if err = x.client.Hello(LocalName); err != nil {
		return err
	}

	if ok, _ := x.client.Extension("STARTTLS"); ok {
		if err := x.client.StartTLS(t); err != nil {
			return err
		}
	}

	if err := x.Auth(); err != nil {
		return err
	}
	x.dialed = true
	return nil
}

func (x *XMailer) Auth() error {

	if ok, _ := x.client.Extension("AUTH"); ok {
		if err := x.client.Auth(x.auth); err != nil {
			return err
		}
	}

	return nil
}

func (x *XMailer) Quit() {
	x.client.Quit()
}

//================= message api =================
func NewMessage() *Message {
	return &Message{}
}

func (m *Message) SetFrom(name, from string) {
	m.FromName = name
	m.FromAddr = from
}

func (m *Message) SetSubject(subject string) {
	m.Subject = subject
}

func (m *Message) AddTo(tos ...string) {
	m.To = append(m.To, tos...)
}

func (m *Message) AddCC(c string) {
	m.CC = append(m.CC, c)
}

func (m *Message) AddBCC(c string) {
	m.Bcc = append(m.Bcc, c)
}

func (m *Message) SetText(text string) {
	m.Text = text
}

func (m *Message) SetHTML(html string) {
	m.HTML = html
}

func (m *Message) AddAttachment(attachment *Attachment) {
	m.Attachments = append(m.Attachments, attachment)
}

func (m *Message) AttachFile(fileName string) error {
	
	fileName = strings.ReplaceAll(fileName, "\\", "/")
	
	ct := ParseContentTypeWithExt(fileName)
	finfo, err := os.Stat(fileName)
	if err != nil {
		return err
	}
	if finfo.IsDir() {
		return fmt.Errorf("%s is not a file", fileName)
	}

	basename := path.Base(fileName)

	m.Attachments = append(m.Attachments, &Attachment{
		ContentType: ct,
		FileName:    fileName,
		BaseName:    basename,
		Content:     nil,
	})

	return nil
}

func (m *Message) payload() ([]byte, error) {

	messageID := generateMessageID()

	payload := strings.Builder{}
	payload.Grow(2048) //TODO: Guess a better buffer size

	payload.WriteString(fmt.Sprintf("Message-Id: %s\r\nMime-Version: 1.0\r\nDate: %s\r\n", messageID, time.Now().Format(time.RFC1123Z)))
	payload.WriteString(fmt.Sprintf("From: %s <%s>\r\n", m.FromName, m.FromAddr))
	payload.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(m.To, ", ")))

	if len(m.CC) > 0 {
		payload.WriteString(fmt.Sprintf("CC: %s\r\n", strings.Join(m.CC, ", ")))
	}
	if len(m.Bcc) > 0 {
		payload.WriteString(fmt.Sprintf("BCC: %s\r\n", strings.Join(m.Bcc, ", ")))
	}

	isMixed := len(m.Attachments) > 0
	isAlternative := len(m.Text) > 0 && len(m.HTML) > 0

	switch {
	case isMixed:
		payload.WriteString(fmt.Sprintf("Content-Type: multipart/mixed;\r\n boundary=%s\r\n", boundary))
	case isAlternative:
		payload.WriteString(fmt.Sprintf("Content-Type: multipart/alternative;\r\n boundary=%s\r\n", boundary))
	case len(m.HTML) > 0:
		payload.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		payload.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	default:
		payload.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		payload.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	}

	payload.WriteString(fmt.Sprintf("Subject: %s\r\n", m.Subject))

	if isMixed || isAlternative {
		payload.WriteString(fmt.Sprintf("\r\n--%s\r\n", boundary))
	}

	if len(m.Text) > 0 {
		if isAlternative || isMixed {
			payload.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
			payload.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
		}
		payload.WriteString(fmt.Sprintf("\r\n%s\r\n", m.Text))
		if isAlternative || isMixed {
			payload.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		}
	}

	if len(m.HTML) > 0 {
		if isAlternative || isMixed {
			payload.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
			payload.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
		}
		payload.WriteString(fmt.Sprintf("\r\n%s\r\n", m.HTML))
		if isAlternative || isMixed {
			payload.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		}
	}

	if isMixed {
		for _, attachment := range m.Attachments {
			if attachment.Content == nil {
				content, err := readFile(attachment.FileName)
				if err != nil {
					return nil, err
				}
				attachment.Content = content
			}

			payload.WriteString(fmt.Sprintf("Content-Disposition: attachment;\r\n filename=\"%s\"\r\n", attachment.BaseName))
			payload.WriteString(fmt.Sprintf("Content-Id: <%s>\r\n", attachment.BaseName))
			payload.WriteString("Content-Transfer-Encoding: base64\r\n")
			payload.WriteString(fmt.Sprintf("Content-Type: %s\r\n\r\n", attachment.ContentType))
			payload.WriteString(base64.StdEncoding.EncodeToString(attachment.Content))
			payload.WriteString(fmt.Sprintf("\r\n--%s\r\n", boundary))
		}
	}

	return []byte(payload.String()), nil
}

func (m *Message) Reset() {
	m.FromAddr = ""
	m.FromName = ""
	m.Subject = ""
	m.CC = nil
	m.Bcc = nil
	m.To = nil
	m.Attachments = nil
	m.HTML = ""
	m.Text = ""
}

//=============== helper func ===============

func generateMessageID() string {
	t := time.Now().UnixNano()
	h, err := os.Hostname()
	if err != nil {
		h = "localdomain"
	}
	pid := os.Getpid()
	return fmt.Sprintf("%d.%d@%s", pid, t, h)
}

func generateBoundary() string {
	buf := bytes.NewBuffer(make([]byte, 70))
	w := multipart.NewWriter(buf)
	return w.Boundary()
}

//ParseContentTypeWithExt
func ParseContentTypeWithExt(fileNameWithExt string) string {
	if fileNameWithExt == "" {
		return "application/octet-stream"
	}

	ext := filepath.Ext(fileNameWithExt)
	if ext == "" {
		return "application/octet-stream"
	}

	ct := mime.TypeByExtension(ext)
	if ct == "" {
		return "application/octet-stream"
	}

	return ct
}

func readFile(fileWithFullPath string) ([]byte, error) {
	finfo, err := os.Stat(fileWithFullPath)
	if err != nil {
		return nil, err
	}

	size := finfo.Size()
	buf := make([]byte, size)

	fp, err := os.OpenFile(fileWithFullPath, os.O_RDONLY, 6)
	if err != nil {
		return nil, err
	}

	_, err = fp.Read(buf)

	return buf, err
}
