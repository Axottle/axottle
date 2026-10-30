export class APIError extends Error {
  constructor(
    message: string,
    public status: number,
    public code = "",
  ) {
    super(message);
  }
}
export const WORKSPACE_KEY = "axo.workspace";
export async function request<T>(
  path: string,
  options: {
    method?: string;
    body?: unknown;
    signal?: AbortSignal;
    project?: string;
  } = {},
): Promise<T> {
  let project = options.project;
  if (project === undefined && typeof window !== "undefined") {
    try {
      project = sessionStorage.getItem(WORKSPACE_KEY) || "";
    } catch {}
  }
  const res = await fetch(`/api/v1/${path.replace(/^\//, "")}`, {
    method: options.method || "GET",
    credentials: "include",
    signal: options.signal,
    headers: {
      ...(options.body !== undefined
        ? { "Content-Type": "application/json" }
        : {}),
      ...(project ? { "X-Axottle-Project": project } : {}),
    },
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    if (
      res.status === 401 &&
      typeof window !== "undefined" &&
      window.location.pathname.startsWith("/new")
    )
      window.location.replace("/login/");
    throw new APIError(
      data.error || `HTTP ${res.status}`,
      res.status,
      data.code,
    );
  }
  return data as T;
}
export interface Account {
  id: string;
  username: string;
  display_name: string;
  role: string;
  status: string;
  permissions: string[];
  is_bootstrap_superuser: boolean;
  has_password: boolean;
  project_ids?: string[];
  created_at?: string;
  auth_methods?: { password: boolean; totp: boolean };
}
export interface Project {
  id: string;
  name: string;
  description: string;
  created_at: string;
  updated_at: string;
}
export interface PermissionGroup {
  key: string;
  permissions: string[];
}
