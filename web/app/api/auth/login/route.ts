import { NextRequest, NextResponse } from "next/server";
import {
  callBackend,
  errorResponse,
  requestJSON,
  sameOriginMutation,
  sessionCookieOptions,
  SESSION_COOKIE,
} from "@/lib/server-api";

export const dynamic = "force-dynamic";

export async function POST(request: NextRequest) {
  if (!sameOriginMutation(request)) {
    return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  }

  const input = await requestJSON(request);
  if (!input || typeof input !== "object") {
    return errorResponse(400, "invalid_request", "Email and password are required.");
  }

  const result = await callBackend("/v1/auth/login", {
    method: "POST",
    body: input,
  });
  if (result.status !== 200 || !result.body || typeof result.body !== "object") {
    return NextResponse.json(result.body, {
      status: result.status,
      headers: { "Cache-Control": "no-store" },
    });
  }

  const session = result.body as {
    access_token?: unknown;
    expires_at?: unknown;
    user?: unknown;
  };
  if (
    typeof session.access_token !== "string" ||
    typeof session.expires_at !== "string" ||
    !session.user
  ) {
    return errorResponse(502, "invalid_session_response", "The API did not return a valid session.");
  }

  const response = NextResponse.json(
    { user: session.user, expires_at: session.expires_at },
    { headers: { "Cache-Control": "no-store" } },
  );
  response.cookies.set(
    SESSION_COOKIE,
    session.access_token,
    sessionCookieOptions(request, session.expires_at),
  );
  return response;
}
