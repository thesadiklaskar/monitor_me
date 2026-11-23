import { NextRequest } from "next/server";
import {
  backendResponse,
  callBackend,
  errorResponse,
  requestJSON,
  sameOriginMutation,
  sessionToken,
} from "@/lib/server-api";

export const dynamic = "force-dynamic";

export async function GET(request: NextRequest) {
  const token = sessionToken(request);
  if (!token) return errorResponse(401, "unauthorized", "Sign in to view server inventory.");
  return backendResponse(await callBackend("/v1/servers", { token }));
}

export async function POST(request: NextRequest) {
  if (!sameOriginMutation(request)) {
    return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  }
  const token = sessionToken(request);
  if (!token) return errorResponse(401, "unauthorized", "Sign in to create a server.");
  const body = await requestJSON(request);
  if (!body || typeof body !== "object") {
    return errorResponse(400, "invalid_request", "A server name and optional hostname are required.");
  }
  return backendResponse(await callBackend("/v1/servers", { method: "POST", token, body }));
}
