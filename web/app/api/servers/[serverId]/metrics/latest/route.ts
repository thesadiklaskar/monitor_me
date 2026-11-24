import { NextRequest } from "next/server";
import { backendResponse, callBackend, errorResponse } from "@/lib/server-api";

export const dynamic = "force-dynamic";
type RouteContext = { params: Promise<{ serverId: string }> };

export async function GET(_request: NextRequest, context: RouteContext) {
  const { serverId } = await context.params;
  if (!/^[0-9a-f-]{36}$/i.test(serverId)) return errorResponse(400, "invalid_server_id", "The server identifier is invalid.");
  return backendResponse(await callBackend(
    `/v1/servers/${encodeURIComponent(serverId)}/metrics/latest`,
    {},
  ));
}
