import type { ErrorBody, Link, LinkPage } from "./types";

export class ApiError extends Error {
  status: number;
  code: string;
  requestId?: string;
  constructor(status: number, code: string, message: string, requestId?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.requestId = requestId;
  }
}

function isErrorBody(v: unknown): v is ErrorBody {
  if (typeof v !== "object" || v === null) return false;
  const e = (v as { error?: unknown }).error;
  if (typeof e !== "object" || e === null) return false;
  const d = e as { code?: unknown; message?: unknown };
  return typeof d.code === "string" && typeof d.message === "string";
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, init);        // relative: the Vite proxy forwards /v1 to Go
  } catch {
    throw new ApiError(0, "network_error", "Cannot reach the server");
  }
  if (!res.ok) {
    const body: unknown = await res.json().catch(() => null);
    if (isErrorBody(body)) {
      const { status, code, message, request_id } = body.error;
      throw new ApiError(status, code, message, request_id);
    }
    throw new ApiError(res.status, "unknown", `HTTP ${res.status}`);
  }
  return (await res.json()) as T;
}


export function createLink(url: string): Promise<Link> {
  return request<Link>("/v1/links", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ url }),
  });
}

export function listLinks(opts: { limit?: number; cursor?: string | null } = {}): Promise<LinkPage> {
  const params = new URLSearchParams({ limit: String(opts.limit ?? 20) });
  if (opts.cursor) params.set("cursor", opts.cursor);
  return request<LinkPage>(`/v1/links?${params}`);
}

export function getLink(code: string): Promise<Link> {
  return request<Link>(`/v1/links/${encodeURIComponent(code)}`);
}

const SHORT_BASE: string = import.meta.env.VITE_SHORT_BASE ?? "http://localhost:8080";
export const shortUrl = (code: string): string => `${SHORT_BASE}/${code}`;

export function messageOf(e: unknown): string {
  return e instanceof ApiError ? e.message : "Something went wrong";
}