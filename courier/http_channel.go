// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package courier

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/ory/kratos/courier/template"
	"github.com/ory/kratos/x"
	"github.com/ory/x/jsonnetsecure"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"

	dysmsapi20170525 "github.com/alibabacloud-go/dysmsapi-20170525/v5/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
)

type (
	httpChannel struct {
		id            string
		requestConfig json.RawMessage
		d             channelDependencies
	}
	channelDependencies interface {
		x.TracingProvider
		x.LoggingProvider
		x.HTTPClientProvider
		jsonnetsecure.VMProvider
		ConfigProvider
	}
)

var _ Channel = new(httpChannel)

func newHttpChannel(id string, requestConfig json.RawMessage, d channelDependencies) *httpChannel {
	return &httpChannel{
		id:            id,
		requestConfig: requestConfig,
		d:             d,
	}
}

func (c *httpChannel) ID() string {
	return c.id
}

// type httpDataModel struct {
//  Recipient    string                `json:"recipient"`
//  Subject      string                `json:"subject"`
//  Body         string                `json:"body"`
//  TemplateType template.TemplateType `json:"template_type"`
//  TemplateData Template              `json:"template_data"`
//  MessageType  string                `json:"message_type"`
// }

// func (c *httpChannel) Dispatch(ctx context.Context, msg Message) (err error) {
//  ctx, span := c.d.Tracer(ctx).Tracer().Start(ctx, "courier.httpChannel.Dispatch")
//  defer otelx.End(span, &err)

//  builder, err := request.NewBuilder(ctx, c.requestConfig, c.d, nil)
//  if err != nil {
//      return errors.WithStack(err)
//  }

//  tmpl, err := newTemplate(c.d, msg)
//  if err != nil {
//      return errors.WithStack(err)
//  }

//  td := httpDataModel{
//      Recipient:    msg.Recipient,
//      Subject:      msg.Subject,
//      Body:         msg.Body,
//      TemplateType: msg.TemplateType,
//      TemplateData: tmpl,
//      MessageType:  msg.Type.String(),
//  }

//  req, err := builder.BuildRequest(ctx, td)
//  if err != nil {
//      return errors.WithStack(err)
//  }
//  req = req.WithContext(ctx)

//  res, err := c.d.HTTPClient(ctx).Do(req)
//  if err != nil {
//      return errors.WithStack(err)
//  }

//  logger := c.d.Logger().
//      WithField("http_server", gjson.GetBytes(c.requestConfig, "url").String()).
//      WithField("message_id", msg.ID).
//      WithField("message_nid", msg.NID).
//      WithField("message_type", msg.Type).
//      WithField("message_template_type", msg.TemplateType).
//      WithField("message_subject", msg.Subject)

//  if res.StatusCode >= 200 && res.StatusCode < 300 {
//      logger.Debug("Courier sent out mailer.")
//      return nil
//  }

//  err = errors.Errorf(
//      "unable to dispatch mail delivery because upstream server replied with status code %d",
//      res.StatusCode,
//  )
//  logger.
//      WithError(err).
//      Error("sending mail via HTTP failed.")
//  return errors.WithStack(err)
// }

func newTemplate(d template.Dependencies, msg Message) (Template, error) {
	switch msg.Type {
	case MessageTypeEmail:
		return NewEmailTemplateFromMessage(d, msg)
	case MessageTypeSMS:
		return NewSMSTemplateFromMessage(d, msg)
	default:
		return nil, fmt.Errorf("received unexpected message type: %s", msg.Type)
	}
}

func (c *httpChannel) Dispatch(ctx context.Context, msg Message) (err error) {
	// ctx, span := c.d.Tracer(ctx).Tracer().Start(ctx, "courier.httpChannel.Dispatch")
	// defer otelx.End(span, &err)

	tmpl, err := newTemplate(c.d, msg)
	fmt.Println(tmpl)
	if err != nil {
		return errors.WithStack(err)
	}
	code := strings.TrimSpace(msg.Body)
	phoneNumber := strings.TrimSpace(strings.TrimPrefix(msg.Recipient, "+86"))
	res, err := SendAliyunSmsCode(code, phoneNumber)
	fmt.Println(res)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func CreateClient() (_result *dysmsapi20170525.Client, _err error) {
	config := &openapi.Config{
		// 必填，请确保代码运行环境设置了环境变量 ALIBABA_CLOUD_ACCESS_KEY_ID。
		AccessKeyId: tea.String(os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_ID")),
		// 必填，请确保代码运行环境设置了环境变量 ALIBABA_CLOUD_ACCESS_KEY_SECRET。
		AccessKeySecret: tea.String(os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET")),
	}
	config.Endpoint = tea.String("dysmsapi.aliyuncs.com")
	_result = &dysmsapi20170525.Client{}
	_result, _err = dysmsapi20170525.NewClient(config)
	return _result, _err
}

//  需要在import中添加  util "github.com/alibabacloud-go/tea-utils/v2/service"

func SendAliyunSmsCode(code string, phoneNumber string) (_result *dysmsapi20170525.SendSmsResponse, _err error) {
	client, _err := CreateClient()
	if _err != nil {
		return nil, _err
	}
	content := fmt.Sprintf(`{"code":"%s"}`, code)
	templateCode := os.Getenv("ALIBABA_TEMPLATE_CODE")
	signName := os.Getenv("ALIBABA_SIGN_NAME")
	// 创建SendSmsRequest结构体实例
	sendSmsRequest := &dysmsapi20170525.SendSmsRequest{
		// 需替换成为您的短信模板code
		TemplateCode: tea.String(templateCode),
		// 需替换成为您的短信模板变量对应的实际值，示例值：{"code":"1234"}
		// TemplateParam: tea.String("123456"),
		TemplateParam: tea.String(content),
		// 需替换成为您的接收手机号码
		PhoneNumbers: tea.String(phoneNumber),
		// 需替换成为您的短信签名
		SignName: tea.String(signName),
	}
	// 创建运行时参数结构体实例
	runtime := &util.RuntimeOptions{}
	response, _err := client.SendSmsWithOptions(sendSmsRequest, runtime)
	if _err != nil {
		return nil, _err
	}
	return response, nil
}
