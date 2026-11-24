import { NextRequest } from "next/server";
import {
  backendResponse,
  callBackend,
  errorResponse,
  requestJSON,
  sameOriginMutation,
} from "@/lib/server-api";

export const dynamic = "force-dynamic";
type RouteContext = { params: Promise<{ serverId: string }> };

export async function POST(request: NextRequest, context: RouteContext) {
  if (!sameOriginMutation(request)) {
    return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  }
  const { serverId } = await context.params;
  if (!/^[0-9a-f-]{36}$/i.test(serverId)) {
    return errorResponse(400, "invalid_server_id", "The server identifier is invalid.");
  }
  const body = await requestJSON(request);
  if (!body || typeof body !== "object" || Array.isArray(body)) {
    return errorResponse(400, "invalid_request", "An operation and allowlisted target are required.");
  }
  return backendResponse(await callBackend(
    "/v1/servers/" + encodeURIComponent(serverId) + "/remote-operations",
    { method: "POST", body },
  ));
}
