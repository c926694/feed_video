export interface User {
  id: number;
  username: string;
  nickname: string;
  avatar: string;
  bio: string;
  followCount: number;
  followerCount: number;
  videoCount: number;
  // followed 表示当前登录用户是否关注了他，取自己的资料时恒为 false
  followed: boolean;
}

export interface TokenPair {
  accessToken: string;
  refreshToken: string;
}

export interface Video {
  id: number;
  title: string;
  description: string;
  coverUrl: string;
  playUrl: string;
  createdAt?: string;
  likeCount: number;
  commentCount: number;
  favoriteCount: number;
  liked: boolean;
  favorited: boolean;
  followed: boolean;
  status: string;
  author: User;
  score?: string;
}

// VideoPage 带游标的视频列表分页结果
export interface VideoPage {
  videos: Video[];
  lastCreatedAt: number;
  lastId: number;
  hasMore: boolean;
}

// MessageActor 通知的触发者
export interface MessageActor {
  id: number;
  nickname: string;
  avatar: string;
}

// MessageItem 通知列表里的一条会话
export interface MessageItem {
  id: number;
  type: string;
  title: string;
  content: string;
  actors: MessageActor[];
  actorCount: number;
  videoId: number;
  commentId: number;
  coverUrl: string;
  isRead: boolean;
  createdAt: string;
}

// MessagePage 通知列表分页结果
export interface MessagePage {
  messages: MessageItem[];
  lastCreatedAt: number;
  lastId: number;
}

export interface Comment {
  id: number;
  content: string;
  likeCount: number;
  liked: boolean;
  createdAt: string;
  author: User;
  parentId: number;
  replyToUserId: number;
  replyToUserName: string;
  replyCount: number;
}
