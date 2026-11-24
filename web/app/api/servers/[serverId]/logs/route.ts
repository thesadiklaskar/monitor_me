import { NextRequest } from "next/server";
import { backendResponse, callBackend, errorResponse } from "@/lib/server-api";

export const dynamic = "force-dynamic";
type RouteContext = { params: Promise<{ serverId: string }> };

export async function GET(request: NextRequest, context: RouteContext) {
  const { serverId } = await context.params;
  if (!/^[0-9a-f-]{36}$/i.test(serverId)) {
    return errorResponse(400, "invalid_server_id", "The server identifier is invalid.");
  }
  const allowed = ["from", "to", "search", "severity", "source", "limit", "before_time", "before_id"];
  const query = new URLSearchParams();
  for (const key of allowed) {
    const value = request.nextUrl.searchParams.get(key);
    if (value !== null) query.set(key, value);
  }
  return backendResponse(await callBackend(
    `/v1/servers/${encodeURIComponent(serverId)}/logs?${query.toString()}`,
    {},
  ));
}
