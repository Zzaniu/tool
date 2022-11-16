package mq_mail

import (
	"encoding/base64"
	"encoding/json"
	"github.com/Zzaniu/tool/mq/rabbit"
	"golang.org/x/xerrors"
	"io/ioutil"
	"path"
)

type (
	MailInfo struct {
		Mail Mail `json:"mail"`
	}

	Mail struct {
		To         string      `json:"to"`
		Cc         string      `json:"cc"`
		Bcc        string      `json:"bcc"`
		Title      string      `json:"title"`
		Body       string      `json:"body"`
		Attachment [][2]string `json:"attachment"`
		Foreign    bool        `json:"foreign"`
	}

	Opt struct {
		Cc       string
		Bcc      string
		filePath []string
		Foreign  bool
		FileInfo map[string][]byte
	}

	Option func(opt *Opt)
)

// WithCc 抄送
func WithCc(cc string) Option {
	return func(opt *Opt) {
		opt.Cc = cc
	}
}

// WithBcc 密送
func WithBcc(bcc string) Option {
	return func(opt *Opt) {
		opt.Bcc = bcc
	}
}

// WithForeign 签名, false 时使用对内签名
func WithForeign(foreign bool) Option {
	return func(opt *Opt) {
		opt.Foreign = foreign
	}
}

// WithFilePath 根据路径发送附件
func WithFilePath(filePath []string) Option {
	return func(opt *Opt) {
		opt.filePath = filePath
	}
}

// WithFileInfo 根据二进制数据发送邮件
func WithFileInfo(fileInfo map[string][]byte) Option {
	return func(opt *Opt) {
		opt.FileInfo = fileInfo
	}
}

// SendMail 基于路径发送邮件附件
func SendMail(product *rabbit.RbMqClient, to, title, body string, opts ...Option) (bool, error) {
	opt := Opt{}

	for _, o := range opts {
		o(&opt)
	}

	var attachment [][2]string
	for _, fileName := range opt.filePath {
		file, err := ioutil.ReadFile(fileName)
		if err != nil {
			return false, xerrors.Errorf("ioutil.ReadFile error, err = %w", err)
		}
		fileStr := base64.StdEncoding.EncodeToString(file)
		attachment = append(attachment, [2]string{path.Base(fileName), fileStr})
	}
	for fineName, fileContent := range opt.FileInfo {
		fileStr := base64.StdEncoding.EncodeToString(fileContent)
		attachment = append(attachment, [2]string{fineName, fileStr})
	}
	m := MailInfo{Mail: Mail{To: to, Cc: opt.Cc, Bcc: opt.Bcc, Title: title, Body: body, Attachment: attachment, Foreign: opt.Foreign}}
	mStr, err := json.Marshal(m)
	if err != nil {
		return false, xerrors.Errorf("json.Marshal error, err = %w", err)
	}
	if product.Publish(mStr) {
		return true, nil
	} else {
		return false, nil
	}
}
