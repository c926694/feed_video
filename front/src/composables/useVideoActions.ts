import { setVideoFavorite } from "@/api/favorite";
import { setFollow } from "@/api/follow";
import { setVideoLike } from "@/api/like";
import type { Video } from "@/types/domain";

// 点赞、收藏、关注三个动作的请求与本地字段写回。
// getLists 返回当前页面持有的全部视频列表，动作成功后按 ID 写回每一个列表，
// 同一个视频出现在多个列表里也不会出现状态不一致。
// onChange 用于关注变化后需要重新拉取列表的页面（关注流就是这样）
export function useVideoActions(
  getLists: () => Video[][],
  onChange?: (action: "like" | "favorite" | "follow") => void
) {
  function findVideo(videoId: number): Video | undefined {
    for (const list of getLists()) {
      const found = list.find((item) => item.id === videoId);
      if (found) return found;
    }
    return undefined;
  }

  function findAuthorVideo(authorId: number): Video | undefined {
    for (const list of getLists()) {
      const found = list.find((item) => item.author.id === authorId);
      if (found) return found;
    }
    return undefined;
  }

  function patch(videoId: number, next: Partial<Video>) {
    for (const list of getLists()) {
      list.forEach((item, index) => {
        if (item.id !== videoId) return;
        list[index] = { ...item, ...next };
      });
    }
  }

  // toggleLike 点赞或取消点赞，同时把本地的点赞数按差值改掉
  async function toggleLike(videoId: number) {
    const current = findVideo(videoId);
    if (!current) return;
    const liked = await setVideoLike(videoId, !current.liked);
    const delta = (liked ? 1 : 0) - (current.liked ? 1 : 0);
    patch(videoId, { liked, likeCount: Math.max(0, current.likeCount + delta) });
    onChange?.("like");
  }

  // toggleFavorite 收藏或取消收藏，同时把本地的收藏数按差值改掉
  async function toggleFavorite(videoId: number) {
    const current = findVideo(videoId);
    if (!current) return;
    const favorited = await setVideoFavorite(videoId, !current.favorited);
    const delta = (favorited ? 1 : 0) - (current.favorited ? 1 : 0);
    patch(videoId, { favorited, favoriteCount: Math.max(0, current.favoriteCount + delta) });
    onChange?.("favorite");
  }

  // toggleFollow 关注或取关作者，同一个作者的全部视频一起改
  async function toggleFollow(authorId: number) {
    if (!authorId) return;
    const current = findAuthorVideo(authorId);
    if (!current) return;
    const followed = await setFollow(authorId, !current.followed);
    for (const list of getLists()) {
      list.forEach((item, index) => {
        if (item.author.id !== authorId) return;
        list[index] = { ...item, followed };
      });
    }
    onChange?.("follow");
  }

  return { toggleLike, toggleFavorite, toggleFollow };
}
