package video

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	videoevent "simple_tiktok/internal/modules/video/event"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/upload"
)

// handleVideoDeleted 删除存储目录里的视频与封面文件
func handleVideoDeleted(ctx context.Context, payload []byte, uploader *upload.Uploader) error {
	var deleted videoevent.DeletedEvent
	if err := json.Unmarshal(payload, &deleted); err != nil {
		return consumer.Permanent(err)
	}
	if deleted.PlayURL == "" && deleted.CoverURL == "" {
		return consumer.Permanent(errors.New("删除视频事件里没有文件路径"))
	}
	if err := uploader.Delete(upload.Video, deleted.PlayURL); err != nil {
		slog.Error("删除视频文件失败", "play_url", deleted.PlayURL, "error", err)
		return err
	}
	if err := uploader.Delete(upload.Cover, deleted.CoverURL); err != nil {
		slog.Error("删除封面文件失败", "cover_url", deleted.CoverURL, "error", err)
		return err
	}
	return nil
}
