package video

import (
	"context"
	"encoding/json"
	"errors"

	"simple_tiktok/internal/mq/event"
	"simple_tiktok/internal/platform/kafka"
	"simple_tiktok/internal/platform/upload"
	"simple_tiktok/internal/svc"
)

func RegisterConsumers(sub *kafka.Subscriber, ctx *svc.ServiceContext) error {
	sub.Subscribe(kafka.TopicVideoDelete, func(handlerCtx context.Context, payload []byte) error {
		return handleDeleteVideo(handlerCtx, payload, ctx.Upload)
	})
	return nil
}

// handleDeleteVideo 消费视频删除事件，删除存储目录里的视频与封面文件
func handleDeleteVideo(ctx context.Context, payload []byte, uploader *upload.Uploader) error {
	var deleteVideoEvent event.DeleteVideoEvent
	if err := json.Unmarshal(payload, &deleteVideoEvent); err != nil {
		return kafka.Permanent(err)
	}
	if deleteVideoEvent.PlayURL == "" && deleteVideoEvent.CoverURL == "" {
		return kafka.Permanent(errors.New("删除视频事件里没有文件路径"))
	}
	if err := uploader.Delete(upload.Video, deleteVideoEvent.PlayURL); err != nil {
		return err
	}
	return uploader.Delete(upload.Cover, deleteVideoEvent.CoverURL)
}
