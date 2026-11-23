import { NextRequest } from "next/server";
import { backendResponse, callBackend, errorResponse, requestJSON, sameOriginMutation, sessionToken } from "@/lib/server-api";
export const dynamic = "force-dynamic";
export async function GET(request: NextRequest) {
  const token = sessionToken(request);
  if (!token) return errorResponse(401, "unauthorized", "Sign in to view projects.");
  return backendResponse(await callBackend("/v1/projects", { token }));
}
export async function POST(request: NextRequest) {
  if (!sameOriginMutation(request)) return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  const token = sessionToken(request);
  if (!token) return errorResponse(401, "unauthorized", "Sign in to create a project.");
  const body = await requestJSON(request);
  if (!body || typeof body !== "object") return errorResponse(400, "invalid_request", "A project name and slug are required.");
  return backendResponse(await callBackend("/v1/projects", { method: "POST", token, body }));
}
