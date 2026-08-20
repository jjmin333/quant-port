export type UserPublic = {
  id: number;
  email: string;
  displayName: string;
  role?: string;
  isAdmin?: boolean;
};

export type AuthSession = {
  user: UserPublic;
  accessToken?: string;
  expiresIn: number;
};
