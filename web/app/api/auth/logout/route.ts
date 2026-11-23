import { NextRequest } from "next/server";
import {
  backendResponse,
  callBackend,
  clearSessionCookie,
  errorResponse,
  sameOriginMutation,
  sessionToken,
} from "@/lib/server-api";

export const dynamic = "force-dynamic";

export async function POST(request: NextRequest) {
  if (!sameOriginMutation(request)) {
    return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  }

  const token = sessionToken(request);
  if (!token) {
    const response = backendResponse({ status: 204, body: null });
    clearSessionCookie(response, request);
    return response;
  }

  const result = await callBackend("/v1/auth/logout", { method: "POST", token });
  if (result.status === 204 || result.status === 401) {
    const response = backendResponse({ status: 204, body: null });
    clearSessionCookie(response, request);
    return response;
  }

  const response = backendResponse({
    status: result.status,
    body: result.body ?? { error: { code: "logout_failed", message: "The session could not be revoked." } },
  });
  // Clear the browser credential even when the upstream API is unavailable.
  // The error remains visible so the operator knows revocation could not be confirmed.
  clearSessionCookie(response, request);
  return response;
}
