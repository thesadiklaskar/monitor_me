export type APIError = {
  error?: {
    code?: string;
    message?: string;
  };
};

export async function readAPIResponse<T>(response: Response): Promise<T> {
  if (response.status === 204) return undefined as T;
  const payload = (await response.json().catch(() => null)) as (T & APIError) | null;
  if (!response.ok) {
    const message = payload?.error?.message || "The request could not be completed.";
    throw new Error(message);
  }
  if (payload === null) throw new Error("The server returned an empty response.");
  return payload as T;
}

export async function apiRequest<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const response = await fetch(path, {
    ...init,
    credentials: "same-origin",
    cache: "no-store",
    headers: {
      Accept: "application/json",
      ...(init.body ? { "Content-Type": "application/json" } : {}),
      ...init.headers,
    },
  });
  return readAPIResponse<T>(response);
}
