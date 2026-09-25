package follow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"simple_tiktok/internal/modules/follow/event"
	"simple_tiktok/internal/modules/follow/repo"
	"simple_tiktok/internal/platform/httpx"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/producer"
	"simple_tiktok/internal/platform/kafka/topic"
)

// Model 关注模块的业务逻辑。关注关系先写 Redis，关系表由订阅方维护。
type Model struct {
	repo     *repo.Repo
	producer *producer.Producer
}

func (m *Model) SwitchFollow(ctx context.Context, targetUserID uint64, currentUserID uint64) (bool, error) {
	if targetUserID == currentUserID {
		return false, httpx.New(httpx.CodeBadRequest, "不能关注自己")
	}
	followed, err := m.repo.Switch(ctx, currentUserID, targetUserID)
	if err != nil {
		return false, err
	}

	err = m.producer.Publish(ctx, topic.FollowSwitched, strconv.FormatUint(targetUserID, 10), event.SwitchedEvent{
		Follower:  currentUserID,
		Following: targetUserID,
		Followed:  followed,
	})
	if err != nil {
		if rollbackErr := m.repo.Reset(ctx, currentUserID, targetUserID, !followed); rollbackErr != nil {
			return false, httpx.New(httpx.CodeInternal, fmt.Sprintf("关注状态回滚失败: %v", rollbackErr))
		}
		return false, err
	}
	return followed, nil
}

// HandleSwitched 订阅自己的事件，维护 follow 表
func (m *Model) HandleSwitched(ctx context.Context, payload []byte) error {
	var switched event.SwitchedEvent
	if err := json.Unmarshal(payload, &switched); err != nil {
		return consumer.Permanent(err)
	}
	if switched.Follower == 0 || switched.Following == 0 {
		return consumer.Permanent(errors.New("关注事件里缺少用户 ID"))
	}

	var err error
	if switched.Followed {
		err = m.repo.Create(ctx, switched.Follower, switched.Following)
	} else {
		err = m.repo.Delete(ctx, switched.Follower, switched.Following)
	}
	if err != nil {
		slog.Error("维护关注关系失败", "follower", switched.Follower, "following", switched.Following, "error", err)
		return err
	}
	return nil
}
