export type ApiError = {
  error: string;
};

export type ApiResponse<T> = {
  data: T;
};
