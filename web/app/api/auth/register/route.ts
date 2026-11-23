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

type RegistrationInput = {
  email: string;
  display_name: string;
  password: string;
  organization_name: string;
  organization_slug: string;
};

export async function POST(request: NextRequest) {
  if (!sameOriginMutation(request)) {
    return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  }

  const input = (await requestJSON(request)) as Partial<RegistrationInput> | null;
  if (
    !input ||
    typeof input.email !== "string" ||
    typeof input.display_name !== "string" ||
    typeof input.password !== "string" ||
    typeof input.organization_name !== "string" ||
    typeof input.organization_slug !== "string"
  ) {
    return errorResponse(400, "invalid_request", "All first-owner registration fields are required.");
  }

  const registration = await callBackend("/v1/auth/register", {
    method: "POST",
    body: input,
  });
  if (registration.status !== 201) {
    return NextResponse.json(registration.body, {
      status: registration.status,
      headers: { "Cache-Control": "no-store" },
    });
  }

  // Registration intentionally returns no session. Create one server-side so the
  // raw bearer token never reaches browser JavaScript or a client-visible response.
  const login = await callBackend("/v1/auth/login", {
    method: "POST",
    body: { email: input.email, password: input.password },
  });
  if (login.status !== 200 || !login.body || typeof login.body !== "object") {
    return errorResponse(502, "registration_login_failed", "The owner account was created, but a session could not be started. Sign in with the new account.");
  }

  const session = login.body as {
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
    { status: 201, headers: { "Cache-Control": "no-store" } },
  );
  response.cookies.set(
    SESSION_COOKIE,
    session.access_token,
    sessionCookieOptions(request, session.expires_at),
  );
  return response;
}
