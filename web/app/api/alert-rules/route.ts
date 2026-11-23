import { NextRequest } from "next/server";
import { backendResponse, callBackend, errorResponse, requestJSON, sameOriginMutation, sessionToken } from "@/lib/server-api";
export const dynamic = "force-dynamic";
export async function GET(request: NextRequest) {
  const token = sessionToken(request);
  if (!token) return errorResponse(401, "unauthorized", "Sign in to view alert rules.");
  return backendResponse(await callBackend("/v1/alert-rules", { token }));
}
export async function POST(request: NextRequest) {
  if (!sameOriginMutation(request)) return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  const token = sessionToken(request);
  if (!token) return errorResponse(401, "unauthorized", "Sign in to create an alert rule.");
  const body = await requestJSON(request);
  if (!body || typeof body !== "object") return errorResponse(400, "invalid_request", "Alert rule data is required.");
  return backendResponse(await callBackend("/v1/alert-rules", { method: "POST", token, body }));
}
