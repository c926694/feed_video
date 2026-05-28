package modules

import (
	"simple_tiktok/internal/modulekit"
	"simple_tiktok/internal/modules/comment"
	"simple_tiktok/internal/modules/feed"
	"simple_tiktok/internal/modules/follow"
	"simple_tiktok/internal/modules/like"
	"simple_tiktok/internal/modules/user"
	"simple_tiktok/internal/modules/video"
)

func Build(ctx modulekit.Context) ([]modulekit.Module, error) {
	userModule := user.NewModule(ctx)

	videoModule, err := video.NewModule(ctx)
	if err != nil {
		return nil, err
	}
	commentModule, err := comment.NewModule(ctx)
	if err != nil {
		return nil, err
	}
	likeModule, err := like.NewModule(ctx)
	if err != nil {
		return nil, err
	}
	followModule, err := follow.NewModule(ctx)
	if err != nil {
		return nil, err
	}
	feedModule, err := feed.NewModule(ctx)
	if err != nil {
		return nil, err
	}

	return []modulekit.Module{
		userModule,
		videoModule,
		commentModule,
		likeModule,
		followModule,
		feedModule,
	}, nil
}
