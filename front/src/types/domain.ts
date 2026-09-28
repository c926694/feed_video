export interface User {
  id: number;
  username: string;
  nickname: string;
  avatar: string;
  bio: string;
  followCount: number;
  followerCount: number;
  videoCount: number;
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
  liked: boolean;
  followed: boolean;
  status: string;
  author: User;
  score?: string;
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
