package like

import (
	"context"
	"fmt"
	"strconv"

	"simple_tiktok/internal/modules/like/event"
	"simple_tiktok/internal/modules/like/repo"
	"simple_tiktok/internal/platform/httpx"
	"simple_tiktok/internal/platform/kafka/producer"
	"simple_tiktok/internal/platform/kafka/topic"
)

// Model 点赞模块的业务逻辑
type Model struct {
	repo     *repo.Repo
	producer *producer.Producer
}
func (m *Model) SwitchVideoLike(ctx context.Context, videoID uint64, userID uint64) (bool, error) {
	return m.switchLike(ctx, event.TargetVideo, videoID, userID)
}

func (m *Model) SwitchCommentLike(ctx context.Context, commentID uint64, userID uint64) (bool, error) {
	return m.switchLike(ctx, event.TargetComment, commentID, userID)
}

func (m *Model) switchLike(ctx context.Context, target string, targetID uint64, userID uint64) (bool, error) {
	liked, err := m.repo.Switch(ctx, target, targetID, userID)
	if err != nil {
		return false, err
	}

	err = m.producer.Publish(ctx, topic.LikeSwitched, strconv.FormatUint(targetID, 10), event.SwitchedEvent{
		Target:   target,
		TargetID: targetID,
		Liked:    liked,
		Operator: userID,
	})
	if err != nil {
		if rollbackErr := m.repo.Reset(ctx, target, targetID, userID, !liked); rollbackErr != nil {
			return false, httpx.New(httpx.CodeInternal, fmt.Sprintf("点赞状态回滚失败: %v", rollbackErr))
		}
		return false, err
	}
	return liked, nil
}
