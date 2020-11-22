# xmailer
一个简单的邮件客户端，支付二进制附件内容，尤其适合作为Rest API接口服务

## usage

### html content
```go
import "github.com/longmon/xmailer"

x, err := NewXMailer("smtp.qq.com:587", "abc@qq.com", "xxxxxx")
if err != nil {
    panic(err)
}
m := NewMessage()
m.SetFrom("longmon", "abc@qq.com")
m.SetSubject("Awesome Subject")
m.AddTo("123456789@qq.com")

m.SetHTML("<h1>This is a test email from xmailer</h1>")

err = x.Dial()
if err != nil {
    panic(err)
}

err = x.Send(m)
if err != nil {
    panic(err)
}
```

### with attachment

```go
import "github.com/longmon/xmailer"

x, err := NewXMailer("smtp.qq.com:587", "abc@qq.com", "xxxxxx")
if err != nil {
    panic(err)
}
m := NewMessage()
m.SetFrom("longmon", "abc@qq.com")
m.SetSubject("Awesome Subject")
m.AddTo("123456789@qq.com")

m.SetHTML("<h1>This is a test email from xmailer</h1>")

//附件可以是二进制文件内容
m.AddAttachment(&xmailer.Attachment{
    ContentType: ParseContentTypeWithExt("abc.jpg"),
    BaseName: "abc.jpg",
    Content: []byte(/*file byte data*/)
})
//也可以是本地文件路径，由程序读取二进制内容
//并填充到content
m.AttachFile("/a/b/abc,jpg")

err = x.Dial()
if err != nil {
    panic(err)
}

err = x.Send(m)
if err != nil {
    panic(err)
}
```