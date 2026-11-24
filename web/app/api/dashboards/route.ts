import { NextRequest } from "next/server";
import { backendResponse, callBackend, errorResponse, requestJSON, sameOriginMutation, sessionToken } from "@/lib/server-api";

export const dynamic = "force-dynamic";

export async function GET(request: NextRequest) {
  const token = sessionToken(request);
  if (!token) return errorResponse(401, "unauthorized", "Sign in to view dashboards.");
  return backendResponse(await callBackend("/v1/dashboards", { token }));
}

export async function POST(request: NextRequest) {
  if (!sameOriginMutation(request)) return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  const token = sessionToken(request);
  if (!token) return errorResponse(401, "unauthorized", "Sign in to create a dashboard.");
  const body = await requestJSON(request);
  if (!body || typeof body !== "object" || Array.isArray(body)) {
    return errorResponse(400, "invalid_request", "A dashboard name is required.");
  }
  return backendResponse(await callBackend("/v1/dashboards", { method: "POST", token, body }));
}
