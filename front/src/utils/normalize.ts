import type { RawComment, RawUser, RawVideo } from "@/types/backend";
import type { Comment, User, Video } from "@/types/domain";

function toNumber(value: unknown, fallback = 0) {
  const n = Number(value);
  return Number.isFinite(n) ? n : fallback;
}

function toString(value: unknown, fallback = "") {
  return typeof value === "string" ? value : fallback;
}

export function normalizeUser(raw?: RawUser | null): User {
  const fallbackUsername = toString(raw?.username ?? raw?.nick_name ?? raw?.user_nick_name ?? raw?.nickname, "anonymous");
  return {
    id: toNumber(raw?.id ?? raw?.user_id),
    username: fallbackUsername,
    nickname: toString(raw?.nick_name ?? raw?.user_nick_name ?? raw?.nickname ?? raw?.username, "匿名用户"),
    avatar: toString(raw?.avatar ?? raw?.avatar_url ?? raw?.avatar_URL),
    bio: toString(raw?.signature ?? raw?.bio, "这个人很神秘，什么都没写。"),
    followCount: toNumber(raw?.follow_count),
    followerCount: toNumber(raw?.follower_count),
    videoCount: toNumber(raw?.video_count)
  };
}

export function normalizeVideo(raw?: RawVideo | null): Video {
  const authorFallback: RawUser = {
    user_id: toNumber(raw?.author_id),
    nick_name: toString(raw?.author_name, "匿名作者"),
    avatar_url: toString(raw?.author_avatar ?? raw?.author_avatar_url)
  };
  return {
    id: toNumber(raw?.id ?? raw?.video_id),
    title: toString(raw?.title, "未命名视频"),
    description: toString(raw?.desc ?? raw?.description),
    coverUrl: toString(raw?.cover ?? raw?.cover_url ?? raw?.coverURL),
    playUrl: toString(raw?.play ?? raw?.play_url ?? raw?.playURL),
    createdAt: toString(raw?.created_at),
    likeCount: toNumber(raw?.like_count ?? raw?.favorite_count),
    commentCount: toNumber(raw?.comment_count),
    liked: Boolean(raw?.is_liked ?? raw?.is_favorite),
    followed: Boolean(raw?.is_follow),
    status: toString(raw?.status, "published"),
    author: normalizeUser(raw?.author ?? raw?.user ?? authorFallback),
    score: raw?.score ? String(raw.score) : undefined
  };
}

export function normalizeComment(raw?: RawComment | null): Comment {
  const fallbackAuthor: RawUser = {
    user_id: typeof raw?.commenter === "number" ? raw.commenter : undefined,
    username: toString(raw?.commenter_username ?? raw?.username, "anonymous"),
    nick_name: toString(raw?.commenter_name ?? raw?.nickname, "匿名用户")
  };
  return {
    id: toNumber(raw?.id ?? raw?.comment_id),
    content: toString(raw?.content ?? raw?.text),
    likeCount: toNumber(raw?.like_count ?? raw?.favorite_count),
    liked: Boolean(raw?.is_liked ?? raw?.is_favorite),
    createdAt: toString(raw?.created_at),
    author: normalizeUser(raw?.author ?? raw?.user ?? fallbackAuthor),
    parentId: toNumber(raw?.parent_id),
    replyToUserId: toNumber(raw?.reply_to_user_id),
    replyToUserName: toString(raw?.reply_to_user_name),
    replyCount: toNumber(raw?.reply_count)
  };
}

export function unwrapData<T>(payload: unknown): T {
  if (payload && typeof payload === "object" && "data" in payload) {
    return (payload as { data: T }).data;
  }
  return payload as T;
}
