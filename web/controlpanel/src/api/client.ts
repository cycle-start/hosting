const BASE_URL = import.meta.env.VITE_API_URL || "";

const DEV_PARTNER_KEY = "dev_partner_override";

let getToken: () => string | null = () => null;
let onUnauthorized: () => void = () => {};

export function configureAuth(
  tokenGetter: () => string | null,
  unauthorizedHandler: () => void,
) {
  getToken = tokenGetter;
  onUnauthorized = unauthorizedHandler;
}

export function getDevPartnerOverride(): string | null {
  return localStorage.getItem(DEV_PARTNER_KEY);
}

export function setDevPartnerOverride(hostname: string | null) {
  if (hostname) {
    localStorage.setItem(DEV_PARTNER_KEY, hostname);
  } else {
    localStorage.removeItem(DEV_PARTNER_KEY);
  }
}

class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };

  const token = getToken();
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const devPartner = getDevPartnerOverride();
  if (devPartner) {
    headers["X-Dev-Partner"] = devPartner;
  }

  const res = await fetch(`${BASE_URL}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });

  if (res.status === 401) {
    onUnauthorized();
    throw new ApiError(401, "Unauthorized");
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new ApiError(res.status, err.error || res.statusText);
  }

  const text = await res.text();
  if (!text) return undefined as T;
  return JSON.parse(text);
}

export const api = {
  get: <T>(path: string) => request<T>("GET", path),
  post: <T>(path: string, body: unknown) => request<T>("POST", path, body),
  patch: <T>(path: string, body: unknown) => request<T>("PATCH", path, body),
  put: <T>(path: string, body: unknown) => request<T>("PUT", path, body),
  delete: (path: string) => request<void>("DELETE", path),
};
