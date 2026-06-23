package feishu

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/hongshuo_wang/go-tiny-claw/internal/engine"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/larksuite/oapi-sdk-go/v3/ws"
)

type FeishuWebsocketBot struct {
	client    *lark.Client
	appID     string
	appSecret string
	engine    *engine.AgentEngine
}

func NewFeishuWebsocketBot(eng *engine.AgentEngine) *FeishuWebsocketBot {
	appID := os.Getenv("FEISHU_APP_ID")
	appSecret := os.Getenv("FEISHU_APP_SECRET")

	if appID == "" || appSecret == "" {
		log.Fatal("请设置 FEISHU_APP_ID 和 FEISHU_APP_SECRET")
	}

	// 实例化飞书官方客户端
	client := lark.NewClient(appID, appSecret)

	return &FeishuWebsocketBot{
		client:    client,
		appID:     appID,
		appSecret: appSecret,
		engine:    eng,
	}
}

// StartWebSocket 启动 WebSocket 长连接方式接收飞书事件（推荐方式）
// 优势：无需公网 IP、无需配置回调 URL、自动重连、部署简单
func (b *FeishuWebsocketBot) StartWebSocket(ctx context.Context) error {
	log.Println("🔌 正在启动 WebSocket 长连接模式...")

	// 创建事件处理器（长连接模式下 verifyToken 和 encryptKey 可以为空）
	eventHandler := b.createEventDispatcher("", "")

	// 创建 WebSocket 客户端
	wsClient := ws.NewClient(
		b.appID,
		b.appSecret,
		ws.WithEventHandler(eventHandler),
		ws.WithLogLevel(larkcore.LogLevelInfo),
		ws.WithAutoReconnect(true), // 自动重连
		ws.WithOnReady(func() {
			log.Println("✅ 已成功连接飞书服务器")
		}),
		ws.WithOnReconnected(func() {
			log.Println("✅ 已重新连接飞书服务器")
		}),
		ws.WithOnDisconnected(func() {
			log.Println("🔌 已断开与飞书服务器的连接")
		}),
	)

	log.Println("✅ WebSocket 客户端已创建，正在连接飞书服务器...")

	errChan := make(chan error, 1)
	go func() {
		errChan <- wsClient.Start(ctx)
	}()

	select {
	case <-ctx.Done():
		log.Println("收到退出信号，准备断开飞书 WebSocket 连接...")
		wsClient.Close()
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}

// createEventDispatcher 创建事件调度器（HTTP 和 WebSocket 共用）
func (b *FeishuWebsocketBot) createEventDispatcher(verifyToken, encryptKey string) *dispatcher.EventDispatcher {
	// 使用官方 SDK 构建调度器，监听 "接收消息" 事件
	handler := dispatcher.NewEventDispatcher(verifyToken, encryptKey).
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			// 由于飞书消息体是 JSON，我们需要粗略地提取其中的文本内容。
			// 这里简单处理：去掉开头结尾的特殊转义字符和引用的机器人名字。
			contentStr := *event.Event.Message.Content
			log.Printf("[Feishu] 收到原始消息: %s\n", contentStr)
			contentStr = strings.TrimPrefix(contentStr, `{"text":"`)
			contentStr = strings.TrimSuffix(contentStr, `"}`)

			chatId := *event.Event.Message.ChatId
			log.Printf("[Feishu] 收到会话 %s 消息: %s\n", chatId, contentStr)

			// 【驾驭并发】：收到消息后，绝不能阻塞回调。
			// 我们要为每个请求开启一个独立的 Goroutine 跑 Agent 任务！
			go b.handleAgentRun(chatId, contentStr)

			return nil
		}).
		OnP2MessageReadV1(func(ctx context.Context, event *larkim.P2MessageReadV1) error {
			// 消息已读事件，静默忽略（避免日志干扰）
			return nil
		})

	return handler
}

// handleAgentRun 是连接飞书与底层引擎的桥梁
func (b *FeishuWebsocketBot) handleAgentRun(chatId string, prompt string) {
	// 为当前聊天窗口实例化一个专属的 Reporter
	reporter := &FeishuReporter{
		client: b.client,
		chatId: chatId,
	}

	err := b.engine.Run(context.Background(), prompt, reporter)
	if err != nil {
		reporter.sendMsg(fmt.Sprintf("❌ Agent 运行崩溃: %v", err))
	}
}
