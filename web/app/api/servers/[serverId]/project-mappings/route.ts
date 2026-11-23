import { NextRequest } from "next/server";
import { backendResponse, callBackend, errorResponse, requestJSON, sameOriginMutation, sessionToken } from "@/lib/server-api";
export const dynamic = "force-dynamic";
type RouteContext = { params: Promise<{ serverId: string }> };
export async function GET(request: NextRequest, context: RouteContext) {
  const token = sessionToken(request);
  if (!token) return errorResponse(401, "unauthorized", "Sign in to view workload mappings.");
  const { serverId } = await context.params;
  if (!/^[0-9a-f-]{36}$/i.test(serverId)) return errorResponse(400, "invalid_server_id", "The server identifier is invalid.");
  return backendResponse(await callBackend(`/v1/servers/${encodeURIComponent(serverId)}/project-mappings`, { token }));
}
export async function POST(request: NextRequest, context: RouteContext) {
  if (!sameOriginMutation(request)) return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  const token = sessionToken(request);
  if (!token) return errorResponse(401, "unauthorized", "Sign in to create a workload mapping.");
  const { serverId } = await context.params;
  if (!/^[0-9a-f-]{36}$/i.test(serverId)) return errorResponse(400, "invalid_server_id", "The server identifier is invalid.");
  const body = await requestJSON(request);
  if (!body || typeof body !== "object") return errorResponse(400, "invalid_request", "A project and workload selector are required.");
  return backendResponse(await callBackend(`/v1/servers/${encodeURIComponent(serverId)}/project-mappings`, { method: "POST", token, body }));
}
