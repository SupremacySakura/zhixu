package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"zhixu-backend/internal/log"
	"zhixu-backend/internal/model"
)

// OrganizeProcessor 执行 organize 类型的 AI 任务：
// 解析模型能力 → 读知识正文 → 调 AI 服务 /organize → 把结构化草稿写回任务 payload。
// 它只写 ai_tasks.payload，绝不写 knowledge_items 的正式字段：AI 产出必须经用户确认（W3.3）。
// 处理器只依赖最小接口，便于用 fake 覆盖各分支（*ChatService / *KnowledgeService / *TagService 均满足）。
type organizeChatResolver interface {
	ResolveChatModel(ctx context.Context, user uint) (*ChatCapability, error)
}

type organizeKnowledgeSource interface {
	Get(ctx context.Context, userID, id uint) (*model.KnowledgeItem, error)
}

type organizeTagLister interface {
	List(ctx context.Context, userID uint) ([]TagView, error)
}

type OrganizeProcessor struct {
	Chat      organizeChatResolver
	Knowledge organizeKnowledgeSource
	// Tags 可选：为 nil 时不带已有标签，模型自由推荐。
	Tags   organizeTagLister
	Client *OrganizeClient
}

func (p *OrganizeProcessor) TaskType() string { return model.AiTaskTypeOrganize }

func (p *OrganizeProcessor) Process(ctx context.Context, task *model.AiTask) (json.RawMessage, error) {
	if p == nil || p.Chat == nil || p.Knowledge == nil {
		return nil, TerminalTaskError("AI 整理未配置")
	}
	capability, err := p.Chat.ResolveChatModel(ctx, task.UserID)
	if err != nil {
		// 模型未配置属于部署状态问题：重试同一任务不会自愈，用户在模型设置里配好后再手动重新生成。
		log.Warn().Err(err).Uint("user_id", task.UserID).Msg("AI 整理无法解析对话模型")
		return nil, TerminalTaskError("未配置可用的对话模型，请前往模型设置")
	}
	item, err := p.Knowledge.Get(ctx, task.UserID, task.TargetID)
	if err != nil {
		if errors.Is(err, ErrKnowledgeNotFound) {
			return nil, TerminalTaskError("知识不存在或已删除")
		}
		return nil, err
	}
	if item.Status != model.KnowledgeReady || strings.TrimSpace(item.Content) == "" {
		return nil, TerminalTaskError("知识尚未就绪，无法生成整理结果")
	}
	draft, err := p.Client.Organize(ctx, capability, task.ID, item.ID, item.Title, item.Content, p.existingTagNames(ctx, task.UserID))
	if err != nil {
		// 契约类错误（模型输出不合规、请求被拒）重试同一个模型没有意义，直接判失败。
		if errors.Is(err, ErrOrganizeInvalidOutput) || errors.Is(err, ErrOrganizeRejected) {
			return nil, TerminalTaskError(err.Error())
		}
		return nil, err
	}
	// 记录生成草稿时的正文版本：详情页据此判断草稿是否已被后续编辑改旧。
	payload, err := json.Marshal(OrganizePayload{OrganizeDraft: *draft, BaseVersion: item.Version})
	if err != nil {
		return nil, TerminalTaskError("整理结果序列化失败")
	}
	return payload, nil
}

// existingTagNames 取用户已有标签名（上限 50），让模型优先复用既有标签体系。
// 标签读取失败不影响整理本身，只是推荐会退化为全新词。
func (p *OrganizeProcessor) existingTagNames(ctx context.Context, userID uint) []string {
	if p.Tags == nil {
		return nil
	}
	views, err := p.Tags.List(ctx, userID)
	if err != nil {
		log.Warn().Err(err).Uint("user_id", userID).Msg("AI 整理读取已有标签失败")
		return nil
	}
	names := make([]string, 0, len(views))
	for _, view := range views {
		names = append(names, view.Name)
		if len(names) == OrganizeExistingTagMax {
			break
		}
	}
	return names
}
