import { NextRequest, NextResponse } from "next/server";

const DEFAULT_API_BASE_URL = "http://127.0.0.1:8080";

type BackendResult = {
  status: number;
  body: unknown;
};

function backendBaseURL(): URL {
  const configured = process.env.MONITOR_ME_API_BASE_URL || DEFAULT_API_BASE_URL;
  const url = new URL(configured);
  if (!["http:", "https:"].includes(url.protocol) || url.username || url.password) {
    throw new Error("MONITOR_ME_API_BASE_URL must be an HTTP(S) URL without embedded credentials.");
  }
  return url;
}

export function sessionToken(_request: NextRequest): string {
  // Each deployment has one implicit installation context and no browser accounts.
  return "local-install-access";
}

export function sameOriginMutation(request: NextRequest): boolean {
  const originHeader = request.headers.get("origin");
  if (!originHeader) return false;

  try {
    const origin = new URL(originHeader);
    if (
      !["http:", "https:"].includes(origin.protocol) ||
      origin.username ||
      origin.password ||
      origin.pathname !== "/" ||
      origin.search ||
      origin.hash
    ) {
      return false;
    }

    // Next.js may see an internal container URL while the browser uses the
    // public host. Validate against the request host and trusted proxy headers.
    const forwardedHost = request.headers.get("x-forwarded-host")?.split(",")[0].trim();
    const host = forwardedHost || request.headers.get("host") || request.nextUrl.host;
    const forwardedProto = request.headers.get("x-forwarded-proto")?.split(",")[0].trim();
    const protocol = forwardedProto
      ? `${forwardedProto.replace(/:$/, "")}:`
      : request.nextUrl.protocol;

    if (!host || !["http:", "https:"].includes(protocol)) return false;

    const expectedOrigin = new URL(`${protocol}//${host}`).origin;
    return origin.origin === expectedOrigin;
  } catch {
    return false;
  }
}

export async function callBackend(
  path: string,
  options: {
    method?: "GET" | "POST" | "DELETE";
    token?: string | null;
    body?: unknown;
  } = {},
): Promise<BackendResult> {
  try {
    const base = backendBaseURL();
    const target = new URL(path, base);
    const headers = new Headers({ Accept: "application/json" });
    if (options.token) headers.set("Authorization", `Bearer ${options.token}`);
    if (options.body !== undefined) headers.set("Content-Type", "application/json");

    const response = await fetch(target, {
      method: options.method ?? "GET",
      headers,
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
      cache: "no-store",
      signal: AbortSignal.timeout(10_000),
    });

    if (response.status === 204) return { status: 204, body: null };
    const raw = await response.text();
    if (!raw) return { status: response.status, body: null };

    try {
      return { status: response.status, body: JSON.parse(raw) as unknown };
    } catch {
      return {
        status: 502,
        body: { error: { code: "invalid_upstream_response", message: "The API returned an unreadable response." } },
      };
    }
  } catch {
    return {
      status: 502,
      body: { error: { code: "upstream_unavailable", message: "The Monitor Me API is temporarily unavailable." } },
    };
  }
}

export function backendResponse(result: BackendResult): NextResponse {
  const headers = { "Cache-Control": "no-store" };
  if (result.status === 204) return new NextResponse(null, { status: 204, headers });
  return NextResponse.json(result.body, { status: result.status, headers });
}

export async function requestJSON(request: NextRequest): Promise<unknown | null> {
  try {
    return await request.json();
  } catch {
    return null;
  }
}

export function errorResponse(status: number, code: string, message: string): NextResponse {
  return NextResponse.json(
    { error: { code, message } },
    { status, headers: { "Cache-Control": "no-store" } },
  );
}
