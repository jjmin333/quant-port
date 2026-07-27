export type UserPublic = {
  id: number;
  email: string;
  displayName: string;
};

export type AuthSession = {
  user: UserPublic;
  accessToken?: string;
  expiresIn: number;
  brokerLinked: boolean;
};

export type BrokerAccount = {
  id: number;
  userId: number;
  brokerCode: string;
  accountName: string;
  accountSeq: number;
  accountNo?: string;
  accountType?: string;
  isPrimary: boolean;
  syncedAt?: string;
};

export type BrokerLinkStatus = {
  linked: boolean;
  brokerCode: string;
  clientIdHint?: string;
  accounts?: BrokerAccount[];
  linkedAt?: string;
};
