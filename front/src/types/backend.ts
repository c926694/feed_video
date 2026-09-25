export interface ApiEnvelope<T = unknown> {
  code?: number;
  msg?: string;
  data?: T;
  [key: string]: unknown;
}

export interface RawUser {
  id?: number;
  user_id?: number;
  username?: string;
  nick_name?: string;
  user_nick_name?: string;
  nickname?: string;
  avatar?: string;
  avatar_url?: string;
  avatar_URL?: string;
  signature?: string;
  bio?: string;
  follow_count?: number;
  follower_count?: number;
  video_count?: number;
  [key: string]: unknown;
}

export interface RawVideo {
  id?: number;
  video_id?: number;
  author_id?: number;
  title?: string;
  author_avatar?: string;
  author_avatar_url?: string;
  created_at?: string;
  desc?: string;
  description?: string;
  cover?: string;
  cover_url?: string;
  coverURL?: string;
  play?: string;
  play_url?: string;
  playURL?: string;
  favorite_count?: number;
  like_count?: number;
  comment_count?: number;
  is_favorite?: boolean;
  is_liked?: boolean;
  is_follow?: boolean;
  author?: RawUser;
  user?: RawUser;
  author_name?: string;
  score?: string | number;
  [key: string]: unknown;
}

export interface RawComment {
  id?: number;
  comment_id?: number;
  commenter?: number;
  commenter_name?: string;
  commenter_username?: string;
  username?: string;
  nickname?: string;
  content?: string;
  text?: string;
  like_count?: number;
  favorite_count?: number;
  is_liked?: boolean;
  is_favorite?: boolean;
  created_at?: string;
  author?: RawUser;
  user?: RawUser;
  [key: string]: unknown;
}
